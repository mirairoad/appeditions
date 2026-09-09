package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// SetSetting moves one control for the whole set, and means it: a screen that
// had pinned this key gives the pin up.
//
// The alternative — leaving pinned screens alone — is defensible on paper and
// wrong in the hand. An override is made by selecting a screen; this endpoint
// is what runs when nothing is selected, so it is the author saying "all of
// them". A slider that moves four tiles out of six, with no way to tell which
// two disagreed, reads as a bug.
var SetSetting = api.Define(api.Spec[api.None, apistore.Setting, apistore.Saved]{
	Name: "SetSetting",
	Handler: func(r *api.Request[api.None, apistore.Setting]) (apistore.Saved, error) {
		var applyErr error
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			if applyErr = r.Body.ApplyTo(&p.Settings); applyErr != nil {
				return
			}
			// The set wins. A screen that pinned this key is an exception made
			// by selecting that screen, and this request was made with nothing
			// selected — so it is a statement about all of them. Without this,
			// tuning one screen and then moving the whole set left that screen
			// where it was and nothing said why.
			p.Unpin(r.Body.OverrideKey())
		})
		if applyErr != nil {
			return apistore.Saved{}, api.BadRequest(applyErr.Error())
		}
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
