package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type TemplateDraft struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func (t TemplateDraft) Validate() error {
	if t.Label == "" {
		return api.BadRequest("give the template a name")
	}
	return nil
}

// SaveTemplate captures the set's current look, including each screen's own
// adjustments as the variants. This is the reason templates are stored rather
// than compiled in: the look that took an afternoon is kept once and applied
// to next release's set in a click.
var SaveTemplate = api.Define(api.Spec[api.None, TemplateDraft, apistore.Saved]{
	Name: "SaveTemplate",
	Handler: func(r *api.Request[api.None, TemplateDraft]) (apistore.Saved, error) {
		t, err := apistore.Get().SaveTemplate(r.Context(), r.Param("id"), r.Body.Label, r.Body.Description)
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: t.ID, Message: "Saved “" + t.Label + "” to your templates"}, nil
	},
})
