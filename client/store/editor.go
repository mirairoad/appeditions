// Package store is the editor's state, and the only copy of the rules that
// change it.
//
// This file compiles for the server *and* for GOOS=js. Nothing server-shaped
// may enter it — no net/http, no database/sql, no os — because the pages import
// it and the pages compile to wasm. That constraint is the point: what is in
// here runs unchanged in both places, so the browser can apply a change
// instantly and the server can apply the same change durably without either
// re-implementing the other.
//
//	editor.go         domain — server AND wasm. The types, the ops, the rules.
//	editor_client.go  signals — package-level, therefore browser-only.
//
// What lives here is the *project document*: the thing the user mutates. The
// preset tables, the asset list and the rendered PNGs do not — they are
// reference data and server output, and putting them in a store would be
// mistaking "everything the page draws" for "the state the page owns".
package store

import (
	"context"
	"sync"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// ---------------------------------------------------------------------------
// The SSR handoff. Pages take no arguments — the generated route table needs
// one uniform signature — so the first paint's data arrives on the context.
// ---------------------------------------------------------------------------

type editorKey struct{}

func WithEditor(ctx context.Context, p model.Project) context.Context {
	return context.WithValue(ctx, editorKey{}, p)
}

func EditorFrom(ctx context.Context) model.Project {
	p, _ := ctx.Value(editorKey{}).(model.Project)
	return p
}

// ---------------------------------------------------------------------------
// The store. One instance per process on the server, one in the browser tab.
// ---------------------------------------------------------------------------

// EditorStore holds the project being edited.
type EditorStore struct {
	mu      sync.Mutex
	project model.Project
	// rev increments on every applied op. The preview tiles are PNGs the
	// server draws, so a local change has to tell the browser that the bytes
	// behind an unchanged URL are stale; rev is what the tile URL carries to
	// say so.
	rev int
}

func NewEditorStore() *EditorStore { return &EditorStore{} }

// EditorSnapshot is the wire format: the whole editable state, serialised.
//
// One shape for three jobs — hydrating the browser from the server, reconciling
// back the other way, and being what an endpoint returns. It is the project
// document itself rather than a projection of it, so there is nothing to keep
// in step as the model grows.
type EditorSnapshot struct {
	Project model.Project `json:"project"`
	Rev     int           `json:"rev"`
}

// Op is a single mutation.
//
// The browser applies it locally for an instant repaint and posts the same
// value to the server, which applies it identically — one implementation of the
// rules, running in two places. Adding a mutation means adding a case here and
// nothing else: the endpoint decodes the same struct.
type Op struct {
	Kind string `json:"kind"`
	// ScreenID scopes an op to one screen. Empty means the whole set, which
	// for a setting is the difference between changing the project and pinning
	// an exception on one tile.
	ScreenID string `json:"screen_id,omitempty"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
	Locale   string `json:"locale,omitempty"`
	Headline string `json:"headline,omitempty"`
	Subhead  string `json:"subhead,omitempty"`
	Delta    int    `json:"delta,omitempty"`
}

// Op kinds. Strings rather than an enum because they cross the wire as JSON
// and a number would be unreadable in a network panel.
const (
	OpSetting = "setting" // change a setting, or pin it on one screen
	OpReset   = "reset"   // drop a screen's overrides for a section
	OpRhythm  = "rhythm"  // pin the strip's compositions
	OpMove    = "move"    // reorder a screen
	OpRemove  = "remove"  // drop a screen
	OpCopy    = "copy"    // write a screen's words in one language
	OpLocale  = "locale"  // add or remove a language
)

func (s *EditorStore) Snapshot() EditorSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return EditorSnapshot{Project: s.project, Rev: s.rev}
}

// Restore replaces the state wholesale. This is the hydrate step, and the way
// back when the server has said something the browser could not derive.
func (s *EditorStore) Restore(sn EditorSnapshot) {
	s.mu.Lock()
	s.project = sn.Project
	s.rev = sn.Rev
	s.mu.Unlock()
	s.publish()
}

func (s *EditorStore) Project() model.Project { return s.Snapshot().Project }
func (s *EditorStore) Rev() int               { return s.Snapshot().Rev }

// Apply runs one op. The server and the browser call this same method, which
// is the whole reason the package is shaped like this.
//
// An op that does not apply is a no-op rather than an error: the browser may
// hold a slightly older project than the server, and a reordering that has
// already happened should not fail a request.
func (s *EditorStore) Apply(op Op) error {
	s.mu.Lock()
	err := apply(&s.project, op)
	if err == nil {
		s.rev++
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.publish()
	return nil
}

// apply is the rules, on a plain value. Separate from the method so it can be
// called on a project the caller already holds — which is what the endpoint
// does inside the document store's patch closure.
func apply(p *model.Project, op Op) error {
	err := mutate(p, op)
	// Every op can change what a composition spans or which screens exist, so
	// the parts are reconciled once, here, rather than in each case: a
	// panorama's second half is a screen in the set and something has to
	// create, rewrite and retire it.
	presets.Sync(p)
	return err
}

func mutate(p *model.Project, op Op) error {
	switch op.Kind {
	case OpSetting:
		setting := model.Setting{Key: op.Key, Value: op.Value}
		if op.ScreenID == "" {
			return setting.ApplyTo(&p.Settings)
		}
		// The lead, not the screen named: pinning a setting on the right half
		// of a panorama is pinning it on the drawing, and pinning it on the
		// half alone would be a value SyncParts overwrites on its way out.
		screen, ok := p.Lead(op.ScreenID)
		if !ok {
			return nil
		}
		return setting.Pin(&screen.Overrides, p.Settings)

	case OpReset:
		screen, ok := p.Lead(op.ScreenID)
		if !ok {
			return nil
		}
		// A named section, or everything. Resetting by section is what makes
		// "put this tile back the way the set is" a single click.
		if keys, named := model.Sections[op.Key]; named {
			screen.Overrides = screen.Overrides.Clear(keys...)
			return nil
		}
		screen.Overrides = model.Overrides{}
		return nil

	case OpRhythm:
		p.ApplyRhythm(presets.RhythmOf(op.Value))
		return nil

	case OpMove:
		p.MoveScreen(op.ScreenID, op.Delta)
		return nil

	case OpRemove:
		p.RemoveScreen(op.ScreenID)
		return nil

	case OpCopy:
		locale := op.Locale
		if locale == "" {
			locale = p.BaseLocale
		}
		p.SetCopy(op.ScreenID, locale, model.Copy{Headline: op.Headline, Subhead: op.Subhead})
		return nil

	case OpLocale:
		if op.Value == "remove" {
			p.RemoveLocale(op.Locale)
			return nil
		}
		p.AddLocale(op.Locale)
		return nil
	}
	return nil
}

// ApplyTo runs an op against a project the caller owns. The endpoint uses it
// inside the document store's patch closure, so the server's write and the
// browser's optimistic update are the same function.
func ApplyTo(p *model.Project, op Op) error { return apply(p, op) }

// ---------------------------------------------------------------------------
// Derived reads. These are why the store is worth having: the browser can
// answer them without a round trip, using the same tables the server used.
// ---------------------------------------------------------------------------

// Spans is how wide each screen's drawing is, in store tiles, by screen id. A
// panorama's halves are two screens that each draw one tile of a two-tile
// composition, so this is 2 on both of them and 1 everywhere else.
func Spans(p model.Project) map[string]int {
	out := make(map[string]int, len(p.Screens()))
	for _, s := range p.Screens() {
		out[s.ID] = presets.Span(s, p.Settings)
	}
	return out
}

// Readiness is the one reading of how far along a set is, computed from the
// same tables on both sides. The rail, the status line and the review page all
// show it; before the store existed, changing a setting meant a round trip
// before the counts caught up.
func Readiness(p model.Project, locale string) model.Readiness {
	r := p.Readiness(locale, func(s model.Screen) bool { return presets.ShowsText(s, p.Settings) })
	return r.WithStore(presets.Size(p.Settings.SizeID).Store)
}
