package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mirairoad/howl-go/db"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// The store's half of editing: resolve the ids an operation needs into values,
// then apply the model's own transformation inside a patch. Nothing in here
// decides anything about a project — that is all in model/edit.go — so the two
// can be read separately.

// NewScreenID is the identifier a screen carries inside its project document.
// Short and random rather than sequential: screens are reordered, deleted and
// re-added, and an index-shaped id would look meaningful while being wrong.
func NewScreenID() string {
	var b [6]byte
	rand.Read(b[:]) //nolint:errcheck // crypto/rand on any supported OS does not fail
	return "s_" + hex.EncodeToString(b[:])
}

// withID copies the envelope's id into the domain type, which leaves its own
// ID untagged so the stored JSON has one "id". Every read path goes through
// here; a page that received a project with an empty ID could not link to it.
func withID(p Project) model.Project {
	p.Project.ID = p.Doc.ID
	p.Project.UpdatedAt = p.Doc.Meta.UpdatedAt
	// Before anything else reads it: a document written before versions
	// existed keeps its set on the project, and Migrate folds that into
	// version 1.0. Without it such a project opens with every screenshot
	// gone — the screens are still in the file, and nothing on screen says so.
	p.Project.Migrate()
	// Reconciled on the way out as well as on the way in, because documents
	// written before a panorama's halves were separate screens hold the lead
	// alone: without this they would render and export as one tile of a
	// two-tile drawing, and the missing half would appear only after the next
	// edit. Deriving the ids from the lead is what makes reading and writing
	// agree about what the set contains.
	presets.Sync(&p.Project)
	return p.Project
}

// ListProjects returns every project, most recently touched first — which is
// the order someone coming back to this actually wants.
func (s *Store) ListProjects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.Projects.Find(ctx, db.Query{Sort: db.Desc("meta.updated_at")})
	if err != nil {
		return nil, err
	}
	out := make([]model.Project, len(rows))
	for i, row := range rows {
		out[i] = withID(row)
	}
	return out, nil
}

func (s *Store) Project(ctx context.Context, id string) (model.Project, error) {
	row, err := s.Projects.Get(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	return withID(row), nil
}

// CreateProject makes the project and its directory, and applies the chosen
// template so the editor opens on a look rather than on nothing.
//
// templateID is a template document id, or "" for the Classic built-in. It is
// deliberately not a built-in key: a project can be started from a template the
// author saved, and those have no key.
func (s *Store) CreateProject(ctx context.Context, p model.Project, templateID string) (model.Project, error) {
	p.Slug = s.uniqueSlug(ctx, model.Slug(p.Name, "project"))
	// Creation does not go through Edit, and a template whose first slot is a
	// panorama has to open with both of its halves in the set.
	presets.Sync(&p)

	row, err := s.Projects.Create(ctx, Project{Project: p})
	if err != nil {
		return model.Project{}, err
	}
	out := withID(row)
	if err := os.MkdirAll(s.AssetsDir(out), 0o755); err != nil {
		return model.Project{}, err
	}

	// The look is applied after the row exists rather than before, because a
	// template brings its own screenshots and an asset belongs to a project id
	// — which does not exist until the row does. ApplyTemplate files them in
	// and lays out the slots in one edit.
	tmpl := s.templateOrBuiltin(ctx, templateID)
	if tmpl.ID == "" {
		return out, nil
	}
	return s.ApplyTemplate(ctx, out.ID, tmpl.ID)
}

// uniqueSlug keeps two projects called "Nobiru" from sharing a directory. The
// suffix is part of the slug from birth, so nothing ever has to be moved.
func (s *Store) uniqueSlug(ctx context.Context, want string) string {
	slug := want
	for n := 2; n < 100; n++ {
		if _, err := s.Projects.One(ctx, db.Query{Where: db.Eq("slug", slug)}); err != nil {
			return slug
		}
		slug = want + "-" + strconv.Itoa(n)
	}
	return want + "-" + NewScreenID()
}

// Edit applies a change to a project. The mutation may run more than once —
// the service retries it against a fresh document when another write lands
// first — so it must be an edit and nothing else.
func (s *Store) Edit(ctx context.Context, id string, mutate func(*model.Project)) (model.Project, error) {
	row, err := s.Projects.Patch(ctx, id, func(p *Project) {
		p.Project.ID = p.Doc.ID // the mutation may read it
		// The patch reads the stored document straight from the row, so an
		// unmigrated one has to be brought forward here too — otherwise the
		// first edit to an old project writes it back with its screens still
		// under the legacy key and a version that is empty.
		p.Project.Migrate()
		mutate(&p.Project)
		// Every server-side write lands here, so this is the one place the
		// parts of a composition are reconciled: a layout that just became a
		// panorama gains its second half, and one that stopped being one loses
		// it. Deriving the ids from the lead is what makes it safe under the
		// retry — the same document comes out however many times this runs.
		presets.Sync(&p.Project)
	})
	if err != nil {
		return model.Project{}, err
	}
	return withID(row), nil
}

// DeleteProject removes a project from the application. The delete is soft —
// the document keeps its row with a deleted_at stamp — and the project's
// directory is deliberately left alone.
//
// What is on disk is the work: the originals the author dropped in and the
// exports they already uploaded. Removing a project from a list should not
// reach outside the application and destroy those, and a directory nobody
// references costs a few megabytes against the alternative of an unrecoverable
// click.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	if err := s.Projects.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}

// Asset resolves one stored original by id.
func (s *Store) Asset(ctx context.Context, id string) (Asset, error) {
	return s.Assets.Get(ctx, id)
}

// ApplyTemplate resolves a template document and applies it.
func (s *Store) ApplyTemplate(ctx context.Context, projectID, templateID string) (model.Project, error) {
	tmpl, err := s.Templates.Get(ctx, templateID)
	if err != nil {
		return model.Project{}, err
	}
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return model.Project{}, err
	}
	// Outside the patch: the mutation may run again under the optimistic lock,
	// and importing the pictures twice would write two rows for one file.
	tmpl.Template.ID = tmpl.Doc.ID
	shots := s.importShots(ctx, p, tmpl.Template, filledLeads(p))

	return s.Edit(ctx, projectID, func(p *model.Project) {
		p.ApplyTemplate(tmpl.Template, shots, NewScreenID)
		// The model works in the template's own vocabulary; the project stores
		// the document id, because that is what a link has to carry.
		p.TemplateID = tmpl.Doc.ID
	})
}

func (s *Store) ApplyRhythm(ctx context.Context, projectID, rhythmID string) (model.Project, error) {
	return s.Edit(ctx, projectID, func(p *model.Project) {
		p.ApplyRhythm(presets.RhythmOf(rhythmID))
	})
}

// AddShots files uploaded screenshots into the set: into the empty slots
// first, then appended.
func (s *Store) AddShots(ctx context.Context, projectID string, assets []Asset) (model.Project, error) {
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return model.Project{}, err
	}
	tmpl := s.templateOrBuiltin(ctx, p.TemplateID)
	rhythm := presets.RhythmOf(p.RhythmID)

	shots := make([]model.Shot, len(assets))
	for i, a := range assets {
		shots[i] = model.Shot{ScreenID: NewScreenID(), AssetID: a.Doc.ID, Title: a.Title()}
	}
	return s.Edit(ctx, projectID, func(p *model.Project) { p.AddShots(shots, tmpl, rhythm) })
}

// templateOrBuiltin resolves a project's template, falling back to Classic. A
// project whose template was deleted must still be editable; it just stops
// having variants to continue.
func (s *Store) templateOrBuiltin(ctx context.Context, id string) model.Template {
	if id != "" {
		if t, err := s.Templates.Get(ctx, id); err == nil {
			t.Template.ID = t.Doc.ID
			return t.Template
		}
	}
	if t, err := s.BuiltinTemplate(ctx, "classic"); err == nil {
		t.Template.ID = t.Doc.ID
		return t.Template
	}
	return model.Template{}
}

// ListTemplates returns the built-ins first, then the author's own.
func (s *Store) ListTemplates(ctx context.Context) ([]model.Template, error) {
	rows, err := s.Templates.Find(ctx, db.Query{Sort: db.Desc("builtin")})
	if err != nil {
		return nil, err
	}
	out := make([]model.Template, len(rows))
	for i, row := range rows {
		row.Template.ID = row.Doc.ID
		out[i] = row.Template
	}
	return out, nil
}

func (s *Store) Template(ctx context.Context, id string) (model.Template, error) {
	row, err := s.Templates.Get(ctx, id)
	if err != nil {
		return model.Template{}, err
	}
	row.Template.ID = row.Doc.ID
	return row.Template, nil
}

// SaveTemplate captures a project's current look as a new template. This is
// the "customise and reuse" path: the author tunes a set until it is right and
// keeps the result, rather than re-deriving it next release.
//
// The variants are the screens' own overrides, so the saved template
// reproduces the rhythm and the alternating colours, not just the base look.
func (s *Store) SaveTemplate(ctx context.Context, projectID, label, description string) (model.Template, error) {
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return model.Template{}, err
	}

	t := model.Template{
		Label:       label,
		Description: description,
		Settings:    overridesFrom(p.Settings),
		Rhythm:      p.RhythmID,
	}
	// Compositions, not screens — the same rule ApplyTemplate reads by. A
	// panorama is one drawing carried as two screens, so capturing both halves
	// would save two variants for one slot and the set would grow by one per
	// panorama every time the template was saved and applied again. SyncParts
	// puts the halves back on the way out.
	for _, screen := range p.Leads() {
		t.Variants = append(t.Variants, screen.Overrides)
		c := screen.Copy[p.BaseLocale]
		t.Samples = append(t.Samples, model.Sample{Headline: c.Headline, Subhead: c.Subhead})
	}
	// A rhythm the variants already encode would be applied twice, and the
	// second application would overwrite the first with the built-in steps.
	if len(t.Variants) > 0 {
		t.Rhythm = ""
	}

	t.Locale = p.BaseLocale

	row, err := s.Templates.Create(ctx, Template{Template: t})
	if err != nil {
		return model.Template{}, err
	}
	// The pictures go in after the row exists, because the directory they live
	// in is named after its id. A failure here leaves a template with words
	// and no captures, which is the old behaviour and still usable — better
	// than refusing to save a look somebody spent an afternoon on.
	if shots, err := s.keepShots(ctx, p, row.Doc.ID, t.Samples); err == nil {
		if patched, err := s.Templates.Patch(ctx, row.Doc.ID, func(dst *Template) {
			dst.Samples = shots
		}); err == nil {
			row = patched
		}
	}
	row.Template.ID = row.Doc.ID
	return row.Template, nil
}

// keepShots copies the screenshot behind each slot into the template's own
// directory and names it in the sample.
//
// Copied, not referenced. An asset row belongs to one project and goes when it
// does; a template pointing at one would come back, months later, as a set of
// missing pictures — and the project it was saved from is exactly the one most
// likely to have been deleted, because the template is what replaced it.
//
// The base language's capture, because that is the language the samples are
// written in. A localised shot is a translation of this one.
func (s *Store) keepShots(ctx context.Context, p model.Project, templateID string, samples []model.Sample) ([]model.Sample, error) {
	assets, err := s.AssetsOf(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	byID := map[string]Asset{}
	for _, a := range assets {
		byID[a.Doc.ID] = a
	}

	dir := s.TemplateDir(templateID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	out := append([]model.Sample(nil), samples...)
	for i, lead := range p.Leads() {
		if i >= len(out) {
			break
		}
		asset, ok := byID[lead.AssetID]
		if !ok {
			continue
		}
		data, err := os.ReadFile(s.Path(p, asset))
		if err != nil {
			continue // the row outlived its file; the slot keeps its words
		}
		name := fmt.Sprintf("%02d%s", i+1, asset.Ext)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return nil, err
		}
		out[i].Shot = name
	}
	return out, nil
}

// importShots copies a template's screenshots into a project as ordinary
// assets, and answers with the asset id for each slot.
//
// Ordinary assets on purpose: once they are in, they are replaceable,
// deletable and listed in Media like anything the author dropped in
// themselves. PutAsset hashes, so applying the same template twice to one
// project reuses the rows rather than filling the originals list with copies.
//
// keep is how many slots the project already has pictures for. Those are the
// ones ApplyTemplate leaves alone, so importing their replacements would drop
// screenshots into Media that nothing on screen uses.
func (s *Store) importShots(ctx context.Context, p model.Project, t model.Template, keep int) []string {
	dir := s.TemplateDir(t.ID)
	shots := make([]string, len(t.Samples))
	for i, sample := range t.Samples {
		if sample.Shot == "" || i < keep {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, sample.Shot))
		if err != nil {
			continue // a template saved before shots were kept, or a lost file
		}
		asset, err := s.PutAsset(ctx, p, model.Slug(t.Label, "template")+"-"+sample.Shot, data)
		if err != nil {
			continue
		}
		shots[i] = asset.Doc.ID
	}
	return shots
}

// filledLeads counts the compositions the project already has a picture for,
// which is the count ApplyTemplate keeps at the front of the set.
func filledLeads(p model.Project) int {
	n := 0
	for _, screen := range p.Leads() {
		if screen.Filled() {
			n++
		}
	}
	return n
}

// overridesFrom turns a full settings value into overrides that pin all of it
// except the two decisions a template never owns: the export size and the
// device.
func overridesFrom(s model.Settings) model.Overrides {
	bg := s.Background
	highlights := append([]string(nil), s.Highlights...)
	return model.Overrides{
		Background:       &bg,
		BackdropColor:    &s.BackdropColor,
		FrameColorID:     &s.FrameColorID,
		PositionID:       &s.PositionID,
		Layout:           &s.Layout,
		Tilt:             &s.Tilt,
		DeviceScale:      &s.DeviceScale,
		TextColor:        &s.TextColor,
		TextAlign:        &s.TextAlign,
		Highlights:       &highlights,
		FontID:           &s.FontID,
		HeadlineScale:    &s.HeadlineScale,
		SubheadScale:     &s.SubheadScale,
		HeadlineTracking: &s.HeadlineTracking,
	}
}

// DuplicateTemplate copies a template into one of the author's own. The copy
// carries no built-in key, so it is never reseeded and never restored —
// exactly as editable as one made from scratch.
func (s *Store) DuplicateTemplate(ctx context.Context, id string) (model.Template, error) {
	src, err := s.Templates.Get(ctx, id)
	if err != nil {
		return model.Template{}, err
	}
	copy := src.Template
	copy.Builtin = false
	copy.Label = copy.Label + " copy"

	row, err := s.Templates.Create(ctx, Template{Template: copy})
	if err != nil {
		return model.Template{}, err
	}
	// The copy gets its own pictures. Sharing the directory would mean
	// deleting either template took the other's screenshots with it.
	if err := copyDir(s.TemplateDir(id), s.TemplateDir(row.Doc.ID)); err != nil {
		return model.Template{}, err
	}
	row.Template.ID = row.Doc.ID
	return row.Template, nil
}

// copyDir copies a template's screenshots. A source that is not there is not
// an error: templates saved before shots were kept have no directory.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTemplate removes one of the author's own. A built-in is refused rather
// than deleted and reseeded: an action that undoes itself on the next start is
// worse than no action.
func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	row, err := s.Templates.Get(ctx, id)
	if err != nil {
		return err
	}
	if row.Builtin {
		return fmt.Errorf("%w: a built-in template comes back on the next start", db.ErrInvalid)
	}
	if err := s.Templates.Delete(ctx, id); err != nil {
		return err
	}
	// The row goes before the files, for the reason RemoveAsset gives: a
	// directory with no row is invisible to the application and stays on disk
	// forever, where a row with no directory is a template that lays out its
	// slots and leaves them empty.
	return os.RemoveAll(s.TemplateDir(id))
}
