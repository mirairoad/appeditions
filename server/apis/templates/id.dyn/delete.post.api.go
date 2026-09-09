package template

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Delete removes a template of the author's own. A built-in cannot be deleted,
// because it would be back on the next start — offering an action that undoes
// itself is worse than not offering it.
var Delete = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "DeleteTemplate",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		if err := apistore.Get().DeleteTemplate(r.Context(), r.Param("id")); err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: r.Param("id")}, nil
	},
})
