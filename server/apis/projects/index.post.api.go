// Package projects creates projects.
package projects

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// New is the body of the create form. Locales arrive as the boxes that were
// ticked; the first is the language the author writes in, and every other one
// falls back to it until it has words of its own.
type New struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Sizes are the store slots the project ships to, one per device family.
	// The first is the one it opens on and is designed against; the rest are
	// drawn from the same screens at their own pixel size.
	Sizes      []string `json:"sizes"`
	TemplateID string   `json:"template_id"`
	Locales    []string `json:"locales"`
}

func (n New) Validate() error {
	if n.Name == "" {
		return api.BadRequest("give the project a name")
	}
	if len(n.Locales) == 0 {
		return api.BadRequest("tick at least one language")
	}
	if len(n.Sizes) == 0 {
		return api.BadRequest("tick at least one device")
	}
	return nil
}

var Create = api.Define(api.Spec[api.None, New, apistore.Saved]{
	Name: "CreateProject",
	Handler: func(r *api.Request[api.None, New]) (apistore.Saved, error) {
		// Each ticked slot becomes a target, with the frame that belongs in it.
		// The frames are changed afterwards from project settings; asking for
		// six of them in a create form would be asking about the look before
		// there is anything to look at.
		targets := make([]model.Target, 0, len(r.Body.Sizes))
		for _, id := range r.Body.Sizes {
			targets = append(targets, presets.Target(id))
		}

		settings := model.DefaultSettings()
		settings.SizeID = targets[0].SizeID
		settings.DeviceID = targets[0].DeviceID

		// The slots go on the first version, not on the project: which devices
		// a release ships to is a property of that release, and the next one
		// may go to fewer.
		p, err := apistore.Get().CreateProject(r.Context(), model.Project{
			Name:        r.Body.Name,
			Description: r.Body.Description,
			Settings:    settings,
			BaseLocale:  r.Body.Locales[0],
			Locales:     r.Body.Locales,
			Versions:    []model.Version{{ID: "v_1", Name: "1.0", Targets: targets}},
			VersionID:   "v_1",
		}, r.Body.TemplateID)
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID, Redirect: "/projects/" + p.ID}, nil
	},
})
