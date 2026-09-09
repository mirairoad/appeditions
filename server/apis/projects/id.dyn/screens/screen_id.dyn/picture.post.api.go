package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Pick is which uploaded original this screen draws. Empty clears the slot,
// which is how a tile goes back to being a laid-out placeholder without being
// deleted from the set.
type Pick struct {
	AssetID string `json:"asset_id"`
}

// SetPicture points a screen at an original the project already holds.
//
// Separate from the multipart upload, and the only path the Replace button
// uses now: a screenshot arrives once, through Media, and every later
// assignment is a reference to it. Uploading again to replace a tile made a
// second copy of a picture already on disk, and left the originals list
// growing with duplicates nobody could tell apart.
var SetPicture = api.Define(api.Spec[api.None, Pick, apistore.Saved]{
	Name: "SetPicture",
	Handler: func(r *api.Request[api.None, Pick]) (apistore.Saved, error) {
		// Belongs to this project, or a screen could be pointed at another
		// project's file and the export would write a picture from a set
		// nobody here can see.
		if r.Body.AssetID != "" {
			asset, err := apistore.Get().Asset(r.Context(), r.Body.AssetID)
			if err != nil || asset.ProjectID != r.Param("id") {
				return apistore.Saved{}, api.BadRequest("no such picture in this project")
			}
		}

		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			// The composition, not the half: replacing the right tile of a
			// panorama replaces the picture both halves are cut from, and
			// writing it onto the half alone would be undone by the sync that
			// follows every edit.
			if screen, ok := p.Lead(r.Param("screen_id")); ok {
				screen.AssetID = r.Body.AssetID
			}
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
