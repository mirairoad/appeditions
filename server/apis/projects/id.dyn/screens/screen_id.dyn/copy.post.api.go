package screen

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// CopyEdit is one field of one screen's words in one language. A field at a
// time rather than the whole screen, because the editor sends a change as it
// is made and two fields in flight must not overwrite each other.
type CopyEdit struct {
	Locale string `json:"locale"`
	Field  string `json:"field"`
	Value  string `json:"value"`
}

func (c CopyEdit) Validate() error {
	if c.Locale == "" {
		return api.BadRequest("which language?")
	}
	if c.Field != "headline" && c.Field != "subhead" {
		return api.BadRequest("field must be headline or subhead")
	}
	return nil
}

var SetCopy = api.Define(api.Spec[api.None, CopyEdit, apistore.Saved]{
	Name: "SetCopy",
	Handler: func(r *api.Request[api.None, CopyEdit]) (apistore.Saved, error) {
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			s, ok := p.Screen(r.Param("screen_id"))
			if !ok {
				return
			}
			// Read what is stored for this language, not what is shown: the
			// field may be displaying the base language's words as a fallback,
			// and copying those into the other field would silently translate
			// nothing into something.
			c := s.Copy[r.Body.Locale]
			if r.Body.Field == "headline" {
				c.Headline = r.Body.Value
			} else {
				c.Subhead = r.Body.Value
			}
			p.SetCopy(s.ID, r.Body.Locale, c)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})
