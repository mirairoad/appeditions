package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type RhythmChoice struct {
	RhythmID string `json:"rhythm_id"`
}

// ApplyRhythm re-paces the strip. It moves three keys per screen — layout,
// arrangement, alignment — and leaves the colours alone, so a Notebook set
// keeps its paper while its compositions change.
var ApplyRhythm = api.Define(api.Spec[api.None, RhythmChoice, apistore.Saved]{
	Name: "ApplyRhythm",
	Handler: func(r *api.Request[api.None, RhythmChoice]) (apistore.Saved, error) {
		p, err := apistore.Get().ApplyRhythm(r.Context(), r.Param("id"), r.Body.RhythmID)
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
