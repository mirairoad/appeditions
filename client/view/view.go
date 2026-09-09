// Package view is what a page is allowed to see.
//
// Pages may not import the store or the endpoint layer — the generated route
// table lives inside the page tree, so anything a page imports has to stay a
// leaf, and `howl check` enforces it. This package is that leaf: plain data,
// assembled by boot.Data before the render and read out of the context by the
// page.
//
// It also keeps the pages honest. A page that could reach the database would
// eventually run a query inside a loop over screens; one that can only read a
// struct cannot.
package view

import (
	"context"
	"strconv"

	"github.com/mirairoad/howl-go/core/state"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// Shell is what every page gets: the things the chrome is drawn from.
type Shell struct {
	Version string
	// Providers are the AI CLIs found on this machine. Empty means the copy
	// buttons are not drawn at all — an action that fails with "command not
	// found" after the click is worse than one that was never offered.
	Providers []Provider
	// DataDir is shown in the footer and opened by the "reveal" action, so the
	// files this writes are never mysterious.
	DataDir string
}

type Provider struct {
	ID    string
	Label string
}

// Projects is the home page: every project, and what state each is in.
type Projects struct {
	Items []ProjectCard
	// Templates seed the "new project" form.
	Templates []model.Template
	Locales   []model.Locale
	Sizes     []model.ExportSize
	Devices   []model.Device
}

type ProjectCard struct {
	Project model.Project
	// Cover is the screen whose preview stands for the project, or "" when
	// nothing has been dropped in yet.
	CoverScreen string
	Readiness   model.Readiness
	Updated     string
}

// Editor is the project workspace. One struct for every step of the flow: the
// steps are views of the same project, and giving each its own loader would
// mean four ways for the sidebar to disagree with the page.
type Editor struct {
	Project model.Project
	// Locale is the language being looked at, which is the base language
	// unless the URL says otherwise.
	Locale string
	// Size is the store slot being looked at. A project ships to several, and
	// this is the one every preview on the page is drawn at — resolved from
	// the URL, so a reload and the back button keep it.
	Size model.ExportSize
	// Version is the submission being edited. It owns the screens on the
	// page, so it is here rather than reached for through Project every time
	// a template asks what it is looking at.
	Version   model.Version
	Locales   []model.Locale
	Readiness model.Readiness

	Templates []model.Template
	Rhythms   []model.Rhythm
	Sizes     []model.ExportSize
	Devices   []model.Device
	Frames    []model.FrameColor
	Layouts   []model.Layout
	Positions []model.Position
	Fonts     []Font
	// Solids and Gradients are the background swatches the picker offers. Any
	// colour is allowed; these are the ones that are one click away.
	Solids    []string
	Gradients []model.Background

	// Selected is the screen the tune panel is scoped to, or "" for the whole
	// set.
	Selected string
	// Assets are the project's uploaded originals, for the "use again" strip.
	Assets []Asset
	// Exports is where files already exist, keyed by SetKey(sizeID, locale).
	Exports map[string]string
	// ExportsDir is the root all of them sit under: the path the export step
	// names as the copy kept on disk.
	ExportsDir string
}

type Font struct {
	ID    string
	Label string
}

type Asset struct {
	ID     string
	Name   string
	Width  int
	Height int
	// Used marks an original something still points at — a screen in any
	// version, or the project's own icon. Not just the version being looked
	// at: the store refuses to delete any of them, and a panel offering a
	// button that always fails is worse than one that explains itself.
	Used bool
	// Use says what it is being used *for*, for the row that cannot offer a
	// delete: "the app icon", "the set", "1.0 and 1.1". Empty when Used is
	// false.
	Use string
}

// Screen returns the screen the tune panel is scoped to, and whether the scope
// is one screen rather than the whole set.
func (e Editor) Screen() (model.Screen, bool) {
	if e.Selected == "" {
		return model.Screen{}, false
	}
	for _, s := range e.Project.Screens() {
		if s.ID == e.Selected {
			return s, true
		}
	}
	return model.Screen{}, false
}

// Settings is what the tune panel edits: the selected screen's resolved
// values, or the project's own when the scope is the whole set.
func (e Editor) Settings() model.Settings {
	if s, ok := e.Screen(); ok {
		return s.Overrides.Apply(e.Project.Settings)
	}
	return e.Project.Settings
}

// SetKey names one exported directory: a store slot in a language. Both parts
// are needed — a project ships every language at every size, and "exported" is
// a fact about the pair, not about either one.
func SetKey(sizeID, locale string) string { return sizeID + "/" + locale }

// Targets are the store slots this project ships to.
func (e Editor) Targets() []model.Target { return e.Project.Targets() }

// SizeOf resolves an export size from the table the page was given.
func (e Editor) SizeOf(id string) model.ExportSize {
	for _, s := range e.Sizes {
		if s.ID == id {
			return s
		}
	}
	return model.ExportSize{ID: id, Label: id}
}

// TemplateSpan is how many store tiles a template's opening composition
// covers, which is how many separate tiles its card has to show. A template
// whose first variant is a panorama delivers two files, and a card that drew
// the composition whole would be showing a picture the export never produces.
//
// The settings are the defaults for the same reason the preview endpoint uses
// them: a card describes the template, not the project it might be applied to,
// and the size and device it is drawn at cannot change a layout's span.
func TemplateSpan(t model.Template) int {
	return presets.Span(model.Screen{Overrides: t.Variant(0)}, t.Settings.Apply(model.DefaultSettings()))
}

// TemplateTiles is how many store tiles applying a template lays out, which is
// not how many variants it has: a panorama variant is one composition and two
// files. The card says tiles because that is what the author is counting —
// what the store will be given.
func TemplateTiles(t model.Template) int {
	base := t.Settings.Apply(model.DefaultSettings())
	var n int
	for i := range t.Slots() {
		n += presets.Span(model.Screen{Overrides: t.Variant(i)}, base)
	}
	return n
}

// Part describes a screen that draws a slice of another's composition, for the
// one line of interface that has to say so: it is an ordinary screen in every
// other respect, but its words and its screenshot belong to the screen it was
// cut from, and a blank copy field with no explanation would read as a bug.
//
// "" for a screen that owns its own drawing, which is nearly all of them.
func (e Editor) Part(s model.Screen) string {
	if !s.Follows() {
		return ""
	}
	label := "part " + strconv.Itoa(s.Part+1)
	if i := e.Project.IndexOf(s.Of); i >= 0 {
		label += " of screen " + strconv.Itoa(i+1)
	}
	return label
}

// Copy is a screen's words in the language being looked at, falling back to
// the base language so a tile never previews blank.
func (e Editor) Copy(s model.Screen) model.Copy {
	return s.Text(e.Locale, e.Project.BaseLocale)
}

// Written reports whether this screen has its own words in the active
// language, as opposed to falling back. It is what the language tabs count.
func (e Editor) Written(s model.Screen) bool {
	c, ok := s.Copy[e.Locale]
	return ok && !c.Empty()
}

// The context accessors. state keys by type, so each of these is its own named
// type and no two can be confused.

func WithShell(ctx context.Context, v Shell) context.Context { return state.With(ctx, v) }
func ShellOf(ctx context.Context) Shell                      { return state.Get[Shell](ctx) }

func WithProjects(ctx context.Context, v Projects) context.Context { return state.With(ctx, v) }
func ProjectsOf(ctx context.Context) Projects                      { return state.Get[Projects](ctx) }

func WithEditor(ctx context.Context, v Editor) context.Context { return state.With(ctx, v) }
func EditorOf(ctx context.Context) Editor                      { return state.Get[Editor](ctx) }

// WithLibrary / LibraryOf carry the template library page.
type Library struct {
	Templates []model.Template
	// Preview is a rendered thumbnail path per template id.
	Preview map[string]string
}

func WithLibrary(ctx context.Context, v Library) context.Context { return state.With(ctx, v) }
func LibraryOf(ctx context.Context) Library                      { return state.Get[Library](ctx) }

// NewEditor builds the workspace view from the document and the URL.
//
// Both sides call it. On the server the loader adds the things only a server
// knows — the uploaded originals, which export directories exist — and in the
// browser the wasm renderer calls it with the store's project after a local
// mutation. Everything it fills is derived from the project and the preset
// tables, which are compile-time data and therefore available in both places.
//
// This is what stops the two renders disagreeing: there is one function that
// turns a project into what a page reads, not one per target.
func NewEditor(p model.Project, locale, sizeID, selected string) Editor {
	if !p.HasLocale(locale) {
		locale = p.BaseLocale
	}
	if t, ok := p.TargetOf(sizeID); ok {
		p.Use(t)
	}

	fonts := make([]Font, 0, len(FontTable))
	for _, f := range FontTable {
		fonts = append(fonts, f)
	}

	if _, ok := p.Screen(selected); !ok {
		// A stale screen id scopes the tune panel to the whole set rather than
		// to nothing.
		selected = ""
	}

	return Editor{
		Project:   p,
		Locale:    locale,
		Version:   p.Version(),
		Size:      presets.Size(p.Settings.SizeID),
		Locales:   presets.Locales,
		Readiness: readiness(p, locale),
		Rhythms:   presets.Rhythms,
		Sizes:     presets.Sizes,
		Devices:   presets.Devices,
		Frames:    presets.FrameColors,
		Layouts:   presets.Layouts,
		Positions: presets.Positions,
		Fonts:     fonts,
		Solids:    presets.SolidBackgrounds,
		Gradients: presets.GradientBackgrounds,
		Selected:  selected,
	}
}

// FontTable is the font list the type controls offer. It is here rather than
// read from the renderer because a page compiles for wasm and the renderer
// embeds twelve megabytes of font files.
var FontTable = []Font{
	{ID: "inter", Label: "Inter"},
	{ID: "dm-sans", Label: "DM Sans"},
	{ID: "poppins", Label: "Poppins"},
	{ID: "space-grotesk", Label: "Space Grotesk"},
	{ID: "playfair", Label: "Playfair Display"},
}

// readiness is the one reading of how far along a set is, from the same tables
// on both sides.
func readiness(p model.Project, locale string) model.Readiness {
	r := p.Readiness(locale, func(s model.Screen) bool { return presets.ShowsText(s, p.Settings) })
	return r.WithStore(presets.Size(p.Settings.SizeID).Store)
}
