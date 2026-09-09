package project

import (
	"slices"
	"strings"

	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Release names one version's store metadata in one language.
//
// The listing is per (version, language) because that is what App Store
// Connect asks for: a project shipping in three languages submits three
// descriptions. Copyright is the exception and sits outside Locale — Apple has
// one per version for the whole app.
type Release struct {
	// Version is which submission, or "" for the one being edited.
	Version string `json:"version,omitempty"`
	// Name renames the version. Free text, clamped to 32 characters.
	Name      string `json:"name,omitempty"`
	Copyright string `json:"copyright,omitempty"`

	// Sizes are the store slots this version ships to, and Frames the device
	// body in each — "<size_id>:<device_id>", because a JSON object with a key
	// per slot would be a dynamic shape the endpoint contract cannot declare.
	//
	// Both arrive only when the form carried them; a release saved from a page
	// that did not show the picker must not silently drop every slot.
	Sizes  []string `json:"sizes,omitempty"`
	Frames []string `json:"frames,omitempty"`

	Locale       string `json:"locale"`
	Promotional  string `json:"promotional"`
	Description  string `json:"description"`
	WhatsNew     string `json:"whats_new"`
	Keywords     string `json:"keywords"`
	SupportURL   string `json:"support_url"`
	MarketingURL string `json:"marketing_url"`
}

// SaveRelease writes the version's name, its copyright and one language's
// listing in a single request.
//
// One request rather than a field at a time: this is a form somebody fills in
// and leaves, not a control that moves a picture, and saving per keystroke
// would re-render the page under the cursor.
var SaveRelease = api.Define(api.Spec[api.None, Release, apistore.Saved]{
	Name: "SaveRelease",
	Handler: func(r *api.Request[api.None, Release]) (apistore.Saved, error) {
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			if r.Body.Version != "" {
				p.UseVersion(r.Body.Version)
			}
			ApplyRelease(p, r.Body)
		})
		if err != nil {
			return apistore.Saved{}, apistore.Fail(err)
		}
		return apistore.Saved{ID: p.ID}, nil
	},
})

// ApplyRelease writes the whole form onto the version being edited: its name,
// its copyright, which devices it ships to, and one language's listing.
//
// Shared with NewVersion, because the create modal and the edit modal are the
// same form. Two implementations of "what this form means" would drift, and
// the drift would be a field that saves when you edit and vanishes when you
// create.
func ApplyRelease(p *model.Project, body Release) {
	v := p.EditVersion()
	if body.Name != "" {
		v.Name = model.ClampName(body.Name)
	}
	v.Copyright = model.Clamp(body.Copyright, model.MaxCopyright)
	applySlots(p, body)

	locale := body.Locale
	if locale == "" {
		locale = p.BaseLocale
	}
	v.SetStore(locale, model.Metadata{
		Promotional:  model.Clamp(body.Promotional, model.MaxPromotional),
		Description:  model.Clamp(body.Description, model.MaxDescription),
		WhatsNew:     model.Clamp(body.WhatsNew, model.MaxWhatsNew),
		Keywords:     model.Clamp(body.Keywords, model.MaxKeywords),
		SupportURL:   model.Clamp(body.SupportURL, model.MaxURL),
		MarketingURL: model.Clamp(body.MarketingURL, model.MaxURL),
	})
}

// applySlots reconciles which devices this version ships to.
//
// An empty list is ignored rather than obeyed: a version that ships nowhere
// has no size to render at, and every preview in the editor would have no
// aspect ratio to be drawn in. Refusing here is better than leaving one in
// that state and explaining it later.
func applySlots(p *model.Project, body Release) {
	if len(body.Sizes) == 0 {
		return
	}

	// The frame for each slot, from the selects that came with it. A slot the
	// form did not name keeps the frame that belongs to it — presets.Target
	// carries the body that goes in each size, which is why ticking "iPad
	// 13-inch" draws an iPad rather than inheriting the last size's phone.
	frame := map[string]string{}
	for _, f := range body.Frames {
		if sizeID, deviceID, ok := strings.Cut(f, ":"); ok {
			frame[sizeID] = deviceID
		}
	}

	keep := map[string]bool{}
	for _, id := range body.Sizes {
		keep[id] = true
	}
	for _, t := range slices.Clone(p.Targets()) {
		if !keep[t.SizeID] {
			p.RemoveTarget(t.SizeID)
		}
	}
	for _, id := range body.Sizes {
		target := presets.Target(id)
		if d := frame[id]; d != "" {
			target.DeviceID = d
		}
		p.AddTarget(target)
	}
	// The size being designed against has to be one this version still ships,
	// or the editor draws at a slot the export will not write.
	if !p.HasTarget(p.Settings.SizeID) {
		p.Use(p.Targets()[0])
	}
}
