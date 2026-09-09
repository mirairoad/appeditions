package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// RemoveScreen takes a screen out of the set. The screenshot behind it is
// kept: originals are never deleted by an edit to the set, so putting the
// screen back is dropping the same file in again — and the file is still
// there.
var RemoveScreen = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "RemoveScreen",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			p.RemoveScreen(r.Param("screen_id"))
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
