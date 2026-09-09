package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// DeleteVersion removes a submission and everything in it. The last one cannot
// go: a project with no versions has nowhere to put a screenshot.
type VersionRef struct {
	Version string `json:"version"`
}

var DeleteVersion = api.Define(api.Spec[api.None, VersionRef, apistore.Saved]{
	Name: "DeleteVersion",
	Handler: func(r *api.Request[api.None, VersionRef]) (apistore.Saved, error) {
		var removeErr error
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			removeErr = p.RemoveVersion(r.Body.Version)
		})
		if removeErr != nil {
			return apistore.Saved{}, api.BadRequest(removeErr.Error())
		}
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
