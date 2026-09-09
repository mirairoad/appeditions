package ai

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/ai"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type TranslateRequest struct {
	Locale string `json:"locale"`
	// OnlyMissing leaves anything already written in this language alone,
	// which is what the button offers by default: a translation pass should
	// not overwrite the line somebody fixed by hand.
	OnlyMissing bool   `json:"only_missing"`
	Provider    string `json:"provider,omitempty"`
}

// Translate carries the base language into another one. It is given the source
// words rather than the screenshots: a translation that ignores what the
// original said is not a translation.
var Translate = api.Define(api.Spec[api.None, TranslateRequest, Drafted]{
	Name: "TranslateCopy",
	Handler: func(r *api.Request[api.None, TranslateRequest]) (Drafted, error) {
		s := apistore.Get()
		p, err := apistore.Project(r.Context(), r.Param("id"))
		if err != nil {
			return Drafted{}, err
		}
		if r.Body.Locale == "" || r.Body.Locale == p.BaseLocale {
			return Drafted{}, api.BadRequest("choose a language other than the one you write in")
		}

		briefs, err := apistore.Briefs(r.Context(), p, p.BaseLocale, false)
		if err != nil {
			return Drafted{}, err
		}
		if r.Body.OnlyMissing {
			briefs = apistore.Only(briefs, p.MissingCopy(r.Body.Locale))
		}
		if len(briefs) == 0 {
			return Drafted{}, api.BadRequest("nothing to translate — every screen already has words in this language")
		}

		written, err := ai.Translate(r.Context(), apistore.Runner(p, r.Body.Provider), p, briefs, p.BaseLocale, r.Body.Locale)
		if err != nil {
			return Drafted{}, api.BadRequest(err.Error())
		}

		if _, err := s.Edit(r.Context(), p.ID, func(p *model.Project) {
			for id, c := range written {
				p.SetCopy(id, r.Body.Locale, c)
			}
		}); err != nil {
			return Drafted{}, apistore.Fail(err)
		}
		return Drafted{Count: len(written), Message: "Translated " + plural(len(written))}, nil
	},
})
