package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type Reset struct {
	// Section is a group of controls from model.Sections; empty clears the
	// screen's overrides entirely.
	Section string `json:"section"`
}

// ClearOverrides puts a screen back to following the set. Clearing is removing
// the keys, not writing the set's current values into them — otherwise the
// screen would stop following the set the moment it was "reset".
var ClearOverrides = api.Define(api.Spec[api.None, Reset, apistore.Saved]{
	Name: "ClearOverrides",
	Handler: func(r *api.Request[api.None, Reset]) (apistore.Saved, error) {
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			s, ok := p.Screen(r.Param("screen_id"))
			if !ok {
				return
			}
			if r.Body.Section == "" {
				s.Overrides = model.Overrides{}
				return
			}
			s.Overrides = s.Overrides.Clear(model.Sections[r.Body.Section]...)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
