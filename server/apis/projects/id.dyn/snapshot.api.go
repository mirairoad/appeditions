package project

import (
	"github.com/mirairoad/howl-go/core/api"

	clientstore "github.com/mirairoad/appeditions/client/store"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Snapshot is the hydrate step: the whole editable state, once, when the
// browser's store comes up.
//
// It is the project document rather than a projection of it, so there is
// nothing to keep in step as the model grows — the same struct the server
// stores, the browser mutates and this returns.
var Snapshot = api.Define(api.Spec[api.None, api.None, clientstore.EditorSnapshot]{
	Name: "EditorSnapshot",
	Handler: func(r *api.Request[api.None, api.None]) (clientstore.EditorSnapshot, error) {
		p, err := apistore.Project(r.Context(), r.Param("id"))
		if err != nil {
			return clientstore.EditorSnapshot{}, err
		}
		return clientstore.EditorSnapshot{Project: p}, nil
	},
})
