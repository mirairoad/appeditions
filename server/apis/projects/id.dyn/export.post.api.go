package project

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/exporter"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type ExportRequest struct {
	Locales []string `json:"locales"`
	// Sizes are the store slots to write. Empty means every target the
	// project ships to, which is what the button does.
	Sizes  []string `json:"sizes"`
	Format string   `json:"format"`
	// Listing writes each language's store text beside its pictures. On by
	// default in the form; a real boolean rather than a present-or-absent
	// checkbox, so "unticked" and "the field was never sent" are different
	// answers.
	Listing bool `json:"listing"`
}

// Export writes the files. It is the same renderer the preview tiles come
// from, at the store's pixel size — there is no second export path, so an
// approved preview and the uploaded file cannot disagree.
var Export = api.Define(api.Spec[api.None, ExportRequest, exporter.Result]{
	Name: "Export",
	Handler: func(r *api.Request[api.None, ExportRequest]) (exporter.Result, error) {
		s := apistore.Get()

		// The format is a project setting, not a per-export flag: someone who
		// switched to JPEG because Connect rejected an alpha channel wants it
		// to stay switched.
		format := r.Body.Format
		if format != model.FormatJPEG {
			format = model.FormatPNG
		}
		p, err := s.Edit(r.Context(), r.Param("id"), func(p *model.Project) { p.Format = format })
		if err != nil {
			return exporter.Result{}, apistore.Fail(err)
		}

		result, err := exporter.Run(r.Context(), s, p, r.Body.Locales, r.Body.Sizes, r.Body.Listing)
		if err != nil {
			return exporter.Result{}, err
		}
		return result, nil
	},
})
