package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Delete removes a project from the application.
//
// The delete is soft and the project's directory is left on disk: what is
// there is the author's own screenshots and the exports they may already have
// uploaded, and a button in a list should not reach outside the application to
// destroy those. The confirmation says so, so nobody has to guess.
var Delete = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "DeleteProject",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		id := r.Param("id")
		p, err := apistore.Project(r.Context(), id)
		if err != nil {
			return apistore.Saved{}, err
		}
		if err := apistore.Get().DeleteProject(r.Context(), id); err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{
			ID:      id,
			Message: p.Name + " removed. Its files are still in " + apistore.Get().Root(),
		}, nil
	},
})
