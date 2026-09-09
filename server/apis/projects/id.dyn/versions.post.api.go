package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// NewVersion starts the next submission by copying the current one.
//
// A copy, not an empty set. Shipping 1.1 is almost always last release's
// screenshots with two replaced and a new "What's New" — starting from nothing
// would mean re-uploading a set the author already has, and the first thing
// anybody would do is go and find the old one.
// The create modal is the same form as the edit modal, so it posts the same
// body: a version is created and filled in one request rather than created
// empty and then saved, which would leave a half-made release behind if the
// second request never happened.
var NewVersion = api.Define(api.Spec[api.None, Release, apistore.Saved]{
	Name: "NewVersion",
	Handler: func(r *api.Request[api.None, Release]) (apistore.Saved, error) {
		name := model.ClampName(r.Body.Name)
		if name == "" {
			return apistore.Saved{}, api.BadRequest("a version needs a name")
		}
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			// AddVersion copies the current release and selects the copy, so
			// everything ApplyRelease then writes lands on the new one.
			p.AddVersion(name)
			ApplyRelease(p, r.Body)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
