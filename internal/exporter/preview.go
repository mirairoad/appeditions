package exporter

import (
	"bytes"
	"context"
	"fmt"
	"image/png"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/internal/render"
	"github.com/mirairoad/appeditions/internal/store"
)

// Preview is one screen — which is one *store tile*, and one uploaded file —
// drawn at editor size. The editor is a page of <img> tags pointed at this:
// there is no canvas in the browser, no second renderer, and therefore nothing
// that can drift from the export.
//
// A screen that draws a slice of a wider composition says so itself, so this
// takes no part argument: the composition is rendered once at its full width
// and cut at the same column the export cuts. What the editor shows is the
// file that will be uploaded, never the un-sliced composition — which is
// nearly square, sits wrong beside the portrait tiles, and hides where the cut
// falls.
func Preview(ctx context.Context, s *store.Store, p model.Project, screenID, locale string, width int) ([]byte, string, error) {
	screen, ok := p.Screen(screenID)
	if !ok {
		return nil, "", fmt.Errorf("no screen %q", screenID)
	}
	size := presets.Size(p.Settings.SizeID)
	height := width * size.H / size.W

	span := render.Span(*screen, p.Settings)
	part := min(max(screen.Part, 0), span-1)

	sources, err := previewSources(ctx, s, p, screenID, locale, width)
	if err != nil {
		return nil, "", err
	}

	copy := screen.Text(locale, p.BaseLocale)
	img := render.Scene(width, height, *screen, p.Settings, copy, sources)
	if span > 1 {
		img = crop(img, part*width, width)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), model.PreviewTag(p, *screen, locale, copy, width), nil
}

// previewSources loads the screenshots one composition can reach. Unfilled
// slots draw the placeholder, which is what makes a chosen template legible
// before any screenshot exists.
func previewSources(ctx context.Context, s *store.Store, p model.Project, screenID, locale string, width int) (render.Sources, error) {
	assets, err := s.AssetsOf(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	byID := map[string]store.Asset{}
	for _, a := range assets {
		byID[a.Doc.ID] = a
	}

	// Which composition each source draws is [model.Project.SourceAssets] —
	// the same walk [model.PreviewTag] hashes, so the bytes loaded here and
	// the tag the tile is cached under can never describe different pictures.
	sources := p.SourceAssets(screenID, locale)

	out := render.Sources{}
	for _, key := range []string{model.SourceSelf, model.SourceNext, model.SourcePrev} {
		asset, ok := byID[sources[key]]
		if !ok {
			out[key] = render.Placeholder()
			continue
		}
		img, err := s.Source(p, asset, width)
		if err != nil {
			return nil, err
		}
		out[key] = img
	}
	return out, nil
}
