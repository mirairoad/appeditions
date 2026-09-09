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
}

// Param is one query parameter, or "".
func (r Request) Param(name string) string { return r.Query.Get(name) }

func WithRequest(ctx context.Context, r Request) context.Context { return state.With(ctx, r) }
func RequestOf(ctx context.Context) Request                      { return state.Get[Request](ctx) }
