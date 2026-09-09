// Package asset is the originals a project has been given.
package asset

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// DeleteAsset removes an uploaded original and the file behind it.
//
// The Media panel is where a project's pictures are actually managed, and the
// thing it has to be able to do is throw one away — a screenshot uploaded by
// mistake otherwise sits in the list forever, offered as something to fill a
// slot with.
//
// The store refuses while a screen still points at it and says which version
// is using it. That check belongs there rather than here: the endpoint is one
// of several ways in, and a rule enforced at the door is a rule with a way
// around it.
var DeleteAsset = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "DeleteAsset",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		id := r.Param("id")
		if err := apistore.Get().RemoveAsset(r.Context(), id, r.Param("asset_id")); err != nil {
			return apistore.Saved{}, api.BadRequest(err.Error())
		}
		return apistore.Saved{ID: id}, nil
	},
})
