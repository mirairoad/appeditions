package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type Move struct {
	Direction string `json:"direction"`
}

// MoveScreen reorders the strip. The screen's overrides travel with it: the
// pose and colour a template pinned belong to the screen, not to the position
// it happened to be in.
var MoveScreen = api.Define(api.Spec[api.None, Move, apistore.Saved]{
	Name: "MoveScreen",
	Handler: func(r *api.Request[api.None, Move]) (apistore.Saved, error) {
		delta := 1
		if r.Body.Direction == "up" {
			delta = -1
		}
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			p.MoveScreen(r.Param("screen_id"), delta)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
