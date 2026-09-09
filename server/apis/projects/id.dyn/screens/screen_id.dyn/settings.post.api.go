// Package screen is everything addressed by one screen inside a project.
package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// PinSetting writes an override: an exception on this screen that survives
// reordering and outlives the next template's variants.
//
// It never writes a resolved value to mean "the same as the set". That would
// silently pin the key, and the screen would stop following the global for the
// rest of its life — the bug this whole nil-means-inherit shape exists to
// prevent.
var PinSetting = api.Define(api.Spec[api.None, apistore.Setting, apistore.Saved]{
	Name: "PinSetting",
	Handler: func(r *api.Request[api.None, apistore.Setting]) (apistore.Saved, error) {
		var pinErr error
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			s, ok := p.Screen(r.Param("screen_id"))
			if !ok {
				return
			}
			pinErr = r.Body.Pin(&s.Overrides, p.Settings)
		})
		if pinErr != nil {
			return apistore.Saved{}, api.BadRequest(pinErr.Error())
		}
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
