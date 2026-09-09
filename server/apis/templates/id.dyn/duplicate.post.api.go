// Package template is everything addressed by a template id.
package template

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Duplicate is how a shipped template becomes yours. The copy is an ordinary
// row with no built-in key, so it is never reseeded and never restored — it is
// exactly as editable as one you made from scratch.
var Duplicate = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "DuplicateTemplate",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		t, err := apistore.Get().DuplicateTemplate(r.Context(), r.Param("id"))
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: t.ID, Message: "Copied to “" + t.Label + "”"}, nil
	},
})
