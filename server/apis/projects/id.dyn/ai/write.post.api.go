// Package ai runs the local coding agent against a project's copy.
package ai

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/ai"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type Draft struct {
	Locale string `json:"locale"`
	// Provider overrides the project's own choice for this one run.
	Provider string `json:"provider,omitempty"`
	// Brief is what the author typed in the modal: who the app is for, what it
	// replaces, the one thing it does better. It overrides the project's stored
	// description for this run, because it was written for this run — and it is
	// the difference between a draft worth editing and a draft worth deleting.
	Brief string `json:"brief,omitempty"`
}

// Drafted is what came back. The count is what the page says out loud; the
// author reads the actual words in the fields, which is the point — this
// writes a draft into the form, it does not decide anything.
type Drafted struct {
	Count   int    `json:"count"`
	Message string `json:"message"`
}

// Write drafts the whole set's headlines in one request rather than one per
// screen. A store listing's headlines have to work as a sequence — a promise,
// then a proof, then a reason to install — and a model shown one screen at a
// time cannot do that.
var Write = api.Define(api.Spec[api.None, Draft, Drafted]{
	Name: "WriteCopy",
	Handler: func(r *api.Request[api.None, Draft]) (Drafted, error) {
		s := apistore.Get()
		p, err := apistore.Project(r.Context(), r.Param("id"))
		if err != nil {
			return Drafted{}, err
		}

		locale := r.Body.Locale
		if locale == "" {
			locale = p.BaseLocale
		}
		briefs, err := apistore.Briefs(r.Context(), p, locale, false)
		if err != nil {
			return Drafted{}, err
		}
		if len(briefs) == 0 {
			return Drafted{}, api.BadRequest("there are no screens to write about yet")
		}

		written, err := ai.Write(r.Context(), apistore.Runner(p, r.Body.Provider), p, briefs, locale, r.Body.Brief)
		if err != nil {
			return Drafted{}, api.BadRequest(err.Error())
		}

		if _, err := s.Edit(r.Context(), p.ID, func(p *model.Project) {
			for id, c := range written {
				p.SetCopy(id, locale, c)
			}
		}); err != nil {
			return Drafted{}, apistore.Fail(err)
		}
		return Drafted{Count: len(written), Message: "Drafted " + plural(len(written)) + " — read them before you ship them"}, nil
	},
})

func plural(n int) string {
	if n == 1 {
		return "1 headline"
	}
	return itoa(n) + " headlines"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
