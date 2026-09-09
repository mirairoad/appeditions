// Package project is everything addressed by a project id.
package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type TemplateChoice struct {
	TemplateID string `json:"template_id"`
}

// ApplyTemplate resets the set to a look. Screens holding a screenshot keep it
// and their words; empty slots are re-laid-out from the new template, which is
// what makes trying looks cheap after the shots are in.
var ApplyTemplate = api.Define(api.Spec[api.None, TemplateChoice, apistore.Saved]{
	Name: "ApplyTemplate",
	Handler: func(r *api.Request[api.None, TemplateChoice]) (apistore.Saved, error) {
		p, err := apistore.Get().ApplyTemplate(r.Context(), r.Param("id"), r.Body.TemplateID)
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
