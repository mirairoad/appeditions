package view

import (
	"context"
	"net/url"

	"github.com/mirairoad/howl-go/core/state"
)

// Request is the part of the HTTP request a render is allowed to see.
//
// It exists because app.Config.Data is handed a path and nothing else, and two
// things the editor needs are in the query string: which language is being
// looked at, and which screen the tune panel is scoped to. Both belong in the
// URL rather than in a control — a reload, a bookmark and the back button all
// have to keep them — so a middleware puts the query here and the loader reads
// it.
type Request struct {
	Path  string
	Query url.Values
	// SidebarCollapsed is the workspace sidebar's state, from the cookie the
	// trigger writes.
	//
	// It has to be readable at render time, and it has to be a cookie rather
	// than a class the browser keeps: a local navigation replaces #outlet
	// wholesale, so the sidebar is re-rendered on every step change and any
	// state held only in the DOM would spring back open each time.
	SidebarCollapsed bool
}

// Param is one query parameter, or "".
func (r Request) Param(name string) string { return r.Query.Get(name) }

func WithRequest(ctx context.Context, r Request) context.Context { return state.With(ctx, r) }
func RequestOf(ctx context.Context) Request                      { return state.Get[Request](ctx) }
