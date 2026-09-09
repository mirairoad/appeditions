// Package store is everything that outlives the process: the SQLite document
// database and the directory of original screenshots beside it.
//
// The split is deliberate. Documents are small, edited constantly and want
// versioning and queries; screenshots are megabytes, written once and read as
// files. Putting a 4 MB PNG in a JSONB column would make every project read
// carry every screenshot in it.
//
//	~/.appeditions/
//	  appeditions.db              projects, templates, assets
//	  <project-slug>/
//	    assets/<id>.png       the originals, never modified
//	    exports/<locale>/…    what the renderer wrote
//	  fonts/                  optional extra fonts for the renderer
//
// The whole of a project's editable state is one document, so an edit is one
// patch and one version. That is what makes an undo, or a "what changed",
// possible later without a schema for it now.
package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mirairoad/howl-go/db"
	"github.com/mirairoad/howl-go/db/sqlite"

	_ "modernc.org/sqlite" // pure Go: no cgo, so the desktop build stays one toolchain

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// Project is the stored form: the database envelope plus the domain type.
// Embedding keeps the JSON flat, and model.Project deliberately leaves its own
// ID untagged so "id" has exactly one meaning in the stored document.
type Project struct {
	db.Doc
	model.Project
}

// Validate runs on create and on every patch. The rules here are the ones that
// would otherwise produce an unrenderable project: a set with no size, or copy
// in a language the project does not ship.
func (p *Project) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.BaseLocale == "" {
		return fmt.Errorf("a base language is required")
	}
	if !p.HasLocale(p.BaseLocale) {
		return fmt.Errorf("base language %q is not in the project's languages", p.BaseLocale)
	}
	return nil
}

// Defaults fills what a half-specified create left out. It must be on the
// pointer receiver: on a value receiver it still satisfies the interface, the
// service still calls it, and every default is written to a copy and lost.
func (p *Project) Defaults() {
	if p.Settings.SizeID == "" {
		p.Settings = model.DefaultSettings()
	}
	if p.Format == "" {
		p.Format = model.FormatPNG
	}
	if p.BaseLocale == "" {
		p.BaseLocale = "en-US"
	}
	if len(p.Locales) == 0 {
		p.Locales = []string{p.BaseLocale}
	}
	// A project written before store slots existed has one implied by its
	// settings, and one written before they moved onto the version has them at
	// the top. Both are [model.Project.Migrate]'s job now — it is the one
	// place that knows what an older document looked like.
	p.Migrate()
	if p.TemplateID == "" {
		p.TemplateID = "classic"
	}
	if p.RhythmID == "" {
		p.RhythmID = "uniform"
	}
	if p.Slug == "" {
		p.Slug = model.Slug(p.Name, "project")
	}
}

// Template is a stored look. Built-ins are seeded on every start.
//
// Key is the stable name a built-in is found by. Document ids are UUIDv7s
// minted by the service — deliberately, so nothing outside can choose one —
// so "the Notebook template" needs a second identifier that survives a reseed.
// A user's own template has no key: it is only ever referred to by its id.
type Template struct {
	db.Doc
	Key string `json:"key,omitempty"`
	model.Template
}

func (t *Template) Validate() error {
	if t.Label == "" {
		return fmt.Errorf("label is required")
	}
	return nil
}

// Asset is one original screenshot on disk. The row is the index; the bytes
// are a file, named after the row so the two cannot drift apart.
type Asset struct {
	db.Doc
	ProjectID string `json:"project_id"`
	// Name is what the file was called when it arrived. It seeds the headline
	// of a screen that has no copy yet, so `02-search-results.png` becomes
	// "02 search results" rather than a blank line.
	Name string `json:"name"`
	Ext  string `json:"ext"`
	// SHA is the content hash. Dropping the same capture twice reuses the row
	// rather than writing a second copy of four megabytes.
	SHA    string `json:"sha"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int64  `json:"bytes"`
}

// Store is the whole persistence layer. One value, held by the application,
// passed to the endpoints.
type Store struct {
	root      string
	conn      *sql.DB
	Projects  *sqlite.Service[Project, *Project]
	Templates *sqlite.Service[Template, *Template]
	Assets    *sqlite.Service[Asset, *Asset]

	images imageCache
}

// DefaultRoot is ~/.appeditions, or $APPEDITIONS_HOME when it is set. Everything the
// program owns lives under it: nothing is written next to the binary, so a
// rebuild never disturbs a project.
func DefaultRoot() string {
	if dir := os.Getenv("APPEDITIONS_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".appeditions"
	}
	return filepath.Join(home, ".appeditions")
}

// Open prepares the data directory and the database. It also seeds the
// built-in templates, which is why it can fail for reasons that have nothing
// to do with SQLite.
func Open(ctx context.Context, root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}

	// WAL and a busy timeout, in the DSN because both are connection settings:
	// SQLite has one writer, and the alternative to waiting for it is
	// SQLITE_BUSY in the middle of an edit.
	dsn := "file:" + filepath.Join(root, "appeditions.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One writer, one connection. A pool over a local file buys nothing here —
	// the whole database is a few hundred kilobytes — and costs lock contention.
	conn.SetMaxOpenConns(1)

	s := &Store{root: root, conn: conn}
	// Promoted columns are the ones something filters on. Everything else
	// stays in the document, which is the point of a document store: adding a
	// field to a project is a Go field and no DDL at all.
	if s.Projects, err = sqlite.New[Project](ctx, conn, sqlite.Options{
		Collection: "projects",
		Promote:    []sqlite.Promote{{Path: "slug"}},
	}); err != nil {
		return nil, err
	}
	if s.Templates, err = sqlite.New[Template](ctx, conn, sqlite.Options{
		Collection: "templates",
		Unique:     []string{"key"},
		Promote:    []sqlite.Promote{{Path: "builtin", Type: db.Boolean}},
	}); err != nil {
		return nil, err
	}
	if s.Assets, err = sqlite.New[Asset](ctx, conn, sqlite.Options{
		Collection: "assets",
		Promote:    []sqlite.Promote{{Path: "project_id"}, {Path: "sha"}},
	}); err != nil {
		return nil, err
	}

	if err := s.seedTemplates(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.conn.Close() }

// Root is the data directory, for the "reveal in Finder" affordance.
func (s *Store) Root() string { return s.root }

// dir is a project's own directory. Named after the slug rather than the id
// because a person opening ~/.appeditions should recognise what they are looking
// at; the slug is fixed at creation so nothing moves when a project is renamed.
func (s *Store) dir(p model.Project) string { return filepath.Join(s.root, p.Slug) }

// AssetsDir is where a project's original screenshots live.
func (s *Store) AssetsDir(p model.Project) string { return filepath.Join(s.dir(p), "assets") }

// ExportDir is where a render lands: one directory per device, then one per
// language inside it.
//
// Device first because that is the shape of the upload. App Store Connect asks
// for a device family and then a localisation, so "the iPhone 6.9-inch set" is
// the folder you open and the languages are what is in it. Language first put
// one device's set in six different places.
func (s *Store) ExportDir(p model.Project, locale, sizeID string) string {
	return filepath.Join(s.ExportsDir(p), sizeID, locale)
}

// ExportsDir is everything written for this project, the tree the zip is made
// from and the path the interface names as the backup.
func (s *Store) ExportsDir(p model.Project) string {
	return filepath.Join(s.dir(p), "exports")
}

// seedTemplates writes the built-ins if they are missing and restores them if
// they were edited. A built-in is a starting point, not a possession: someone
// who wants their own version duplicates it, and the copy is theirs.
func (s *Store) seedTemplates(ctx context.Context) error {
	keys := make([]string, 0, len(presets.BuiltinTemplates))
	for _, t := range presets.BuiltinTemplates {
		keys = append(keys, t.ID)
	}
	// One query for all of them, not one per template: this runs on every
	// start, and the filter grammar exists so a loop does not have to.
	rows, err := s.Templates.Find(ctx, db.Query{Where: db.In("key", keys)})
	if err != nil {
		return err
	}
	stored := make(map[string]Template, len(rows))
	for _, row := range rows {
		stored[row.Key] = row
	}

	// The writes below are per template rather than bulk, which `howl check`
	// flags. There is no bulk create in the contract, this runs once per start,
	// and it only writes at all when a shipped definition actually changed —
	// so the loop is four statements on a cold start and nothing after that.
	for _, t := range presets.BuiltinTemplates {
		existing, ok := stored[t.ID]
		if !ok {
			if _, err := s.Templates.Create(ctx, Template{Key: t.ID, Template: t}); err != nil {
				return err
			}
			continue
		}
		// Only rewrite when the shipped definition actually changed, so a
		// restart is not four pointless writes.
		if sameTemplate(existing.Template, t) {
			continue
		}
		if _, err := s.Templates.Patch(ctx, existing.Doc.ID, func(dst *Template) {
			dst.Template = t
		}); err != nil {
			return err
		}
	}

	// A built-in dropped from the list has to go, or a database seeded by an
	// earlier version keeps offering a look this one no longer ships and no
	// longer restores. Only rows that carry a key are touched: a duplicate is
	// somebody's own template with Builtin cleared, and it is theirs.
	shipped := make(map[string]bool, len(presets.BuiltinTemplates))
	for _, t := range presets.BuiltinTemplates {
		shipped[t.ID] = true
	}
	all, err := s.Templates.Find(ctx, db.Query{})
	if err != nil {
		return err
	}
	for _, row := range all {
		if row.Key == "" || shipped[row.Key] {
			continue
		}
		if err := s.Templates.Delete(ctx, row.Doc.ID); err != nil {
			return err
		}
	}
	return nil
}

// sameTemplate compares the shipped definition with the stored one. The
// comparison is over the encoded form because a template is a tree of pointers
// and comparing those compares addresses.
func sameTemplate(a, b model.Template) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(ja, jb)
}

// BuiltinTemplate resolves a built-in by its stable key.
func (s *Store) BuiltinTemplate(ctx context.Context, key string) (Template, error) {
	return s.Templates.One(ctx, db.Query{Where: db.Eq("key", key)})
}
