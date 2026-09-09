// Package apistore is what the endpoints are allowed to reach: the store, the
// AI runner, and the few conversions every handler would otherwise repeat.
//
// The store arrives through Use rather than being constructed here, because
// the same application is assembled twice — once for the server binary and
// once for the native window — and both hand it the store they opened.
package apistore

import (
	"context"
	"errors"
	"strconv"

	"github.com/mirairoad/howl-go/core/api"
	"github.com/mirairoad/howl-go/db"

	"github.com/mirairoad/appeditions/internal/ai"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/store"
)

var current *store.Store

// Use installs the store the endpoints operate on. Called once, from boot.
func Use(s *store.Store) { current = s }

// Get is the store. It panics when nothing has been installed, which can only
// happen in a binary that forgot to call Use — a startup bug, not a runtime
// condition, and better as a panic on the first request than a nil map three
// frames down.
func Get() *store.Store {
	if current == nil {
		panic("apistore: no store installed; call apistore.Use from boot")
	}
	return current
}

// Saved is what most endpoints answer with. The browser re-renders the page it
// is on afterwards, so the body only has to carry what the client cannot
// already see: where to go next, and anything worth saying out loud.
type Saved struct {
	ID       string `json:"id"`
	Redirect string `json:"redirect,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Fail maps a store error onto a status. The document store's sentinels are
// the whole vocabulary — anything else is genuinely a 500 and its details stay
// on this side.
func Fail(err error) error {
	switch {
	case errors.Is(err, db.ErrNotFound):
		return api.NotFound("no such thing here")
	case errors.Is(err, db.ErrInvalid):
		return api.BadRequest(err.Error())
	case errors.Is(err, db.ErrConflict):
		return api.Conflict("that name is taken")
	}
	return err
}

// Setting is the control-value language, re-exported. It lives in the model
// because the browser parses the same values with the same code — this is only
// here so the endpoints keep reading apistore.Setting.
type Setting = model.Setting

// ParseBackground reads the little language the swatches post.
var ParseBackground = model.ParseBackground

// Project loads a project or turns the miss into a 404.
func Project(ctx context.Context, id string) (model.Project, error) {
	p, err := Get().Project(ctx, id)
	if err != nil {
		return p, Fail(err)
	}
	return p, nil
}

// Runner is the AI configuration for a project. The provider argument
// overrides the project's own choice for one run, which is what the selector
// above the buttons does.
func Runner(p model.Project, provider string) ai.Runner {
	if provider == "" {
		provider = p.AIProvider
	}
	if provider == "" {
		// Whatever is installed. Offering nothing when a CLI is right there is
		// worse than picking the first one.
		if available := ai.Available(); len(available) > 0 {
			provider = available[0].ID
		}
	}
	return ai.Runner{Provider: provider}
}

// Briefs describes a project's screens to the model: the file the screenshot
// came from, and what the screen currently says. The filename is included
// because it is often the only thing that says what the screen actually shows.
func Briefs(ctx context.Context, p model.Project, locale string, filledOnly bool) ([]ai.Brief, error) {
	assets, err := Get().AssetsOf(ctx, p.ID)
	if err != nil {
		return nil, Fail(err)
	}
	names := map[string]string{}
	for _, a := range assets {
		names[a.Doc.ID] = a.Name
	}

	var out []ai.Brief
	for i, s := range p.Screens() {
		if filledOnly && !s.Filled() {
			continue
		}
		// One brief per composition. A panorama's second half holds a copy of
		// the first's words, and briefing the model on both would have it
		// write the same headline twice — and charge for it.
		if s.Follows() {
			continue
		}
		c := s.Copy[locale]
		file := names[s.AssetID]
		if file == "" {
			file = "screen " + strconv.Itoa(i+1) + " (no screenshot yet)"
		}
		out = append(out, ai.Brief{
			ScreenID: s.ID,
			File:     file,
			Headline: c.Headline,
			Subhead:  c.Subhead,
		})
	}
	return out, nil
}

// Only narrows a brief list to a set of screen ids — the screens a translation
// pass has left to do.
func Only(briefs []ai.Brief, ids []string) []ai.Brief {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []ai.Brief
	for _, b := range briefs {
		if want[b.ScreenID] {
			out = append(out, b)
		}
	}
	return out
}
