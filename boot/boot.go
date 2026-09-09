// Package boot assembles the application.
//
// It exists because two binaries need the identical app — the server in
// main.go and the native window in desktop/ — and the only difference between
// them is how the listener is opened.
package boot

import (
	"context"
	"net/http"

	"github.com/mirairoad/howl-go/core/api"
	"github.com/mirairoad/howl-go/core/app"
	"github.com/mirairoad/howl-go/core/mw"

	"github.com/mirairoad/appeditions/client"
	"github.com/mirairoad/appeditions/client/pages"
	"github.com/mirairoad/appeditions/client/view"
	"github.com/mirairoad/appeditions/internal/store"
	"github.com/mirairoad/appeditions/server/apis"
	"github.com/mirairoad/appeditions/server/apis/apistore"
	"github.com/mirairoad/appeditions/server/handlers"
)

// Version is stamped at build time with -ldflags "-X …boot.Version=…". It is
// only ever displayed, never branched on.
var Version = "dev"

// New opens the data directory and returns the application, the handler to
// serve it with, and the store — which the caller closes.
func New(ctx context.Context, root string) (*app.App, http.Handler, *store.Store, error) {
	s, err := store.Open(ctx, root)
	if err != nil {
		return nil, nil, nil, err
	}
	apistore.Use(s)
	loader := &handlers.Loader{Store: s, Version: Version}

	a := app.New(app.Config{
		Routes:   pages.FsClientRoutes(),
		Shell:    pages.App,
		NotFound: pages.NotFound,
		Public:   client.Public(),
		Data:     loader.Data,
		Addr:     ":9010",

		// Outermost first. Ordinary net/http decorators — nothing here knows
		// about templ, routes or this application.
		Use: []mw.Middleware{
			mw.RequestID,
			mw.LogWith(mw.LogOptions{Skip: mw.SkipNoise}),
			mw.Recover(nil),
			mw.Compress{}.Handler,
			// The query string, onto the context, for Data to read. Data is
			// given a path and nothing else, and the editor's language and
			// selected screen live in the query.
			carryRequest,
		},
	})

	mux := a.Mux()
	api.Register(mux, api.Config{}, apis.FsApiRoutes()...)
	handlers.Register(mux, s)

	return a, mux, s, nil
}

// carryRequest puts the request's path and query on the context so the page
// loader can see them. It is middleware rather than a Data parameter because
// Data's signature belongs to the framework, and this is the one thing this
// application needs that it does not carry.
func carryRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(view.WithRequest(r.Context(), view.Request{
			Path:  r.URL.Path,
			Query: r.URL.Query(),
		})))
	})
}
