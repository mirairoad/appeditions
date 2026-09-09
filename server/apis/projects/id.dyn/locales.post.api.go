package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// LocaleChange is one language in or out. Two fields rather than a verb and a
// tag, because the button that sends it knows which of the two it is and the
// endpoint should not have to interpret a mode string.
type LocaleChange struct {
	Add    string `json:"add,omitempty"`
	Remove string `json:"remove,omitempty"`
}

// SetLocales adds or removes a language. Adding one translates nothing:
// every screen falls back to the base language until words exist, so the set
// still renders and the copy step shows exactly what is left to write.
var SetLocales = api.Define(api.Spec[api.None, LocaleChange, apistore.Saved]{
	Name: "SetLocales",
	Handler: func(r *api.Request[api.None, LocaleChange]) (apistore.Saved, error) {
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			p.AddLocale(r.Body.Add)
			p.RemoveLocale(r.Body.Remove)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
