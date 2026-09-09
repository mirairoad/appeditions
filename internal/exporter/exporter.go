// Package exporter turns a project into files on disk: one directory per
// language, one PNG per store tile.
//
// It calls the same [render.Scene] the editor's preview calls, with the store
// size instead of a thumbnail size. That is the whole design — there is no
// export renderer — so an approved preview and the uploaded file cannot
// disagree.
package exporter

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/internal/render"
	"github.com/mirairoad/appeditions/internal/store"
)

// Result is what an export produced: one Set per target × language, because
// that is one directory and one upload.
type Result struct {
	Files int   `json:"files"`
	Sets  []Set `json:"sets"`
}

type Set struct {
	SizeID string   `json:"size_id"`
	Locale string   `json:"locale"`
	Dir    string   `json:"dir"`
	Files  []string `json:"files"`
	// Listing is the store text written beside the tiles. Counted separately
	// from Files, which is the store tiles — "wrote 8 files" for a six-tile
	// set is a number the author cannot reconcile with what they can see.
	Listing string `json:"listing,omitempty"`
}

// Run renders every screen of a project at every requested target, in every
// requested language, and writes the tiles.
//
// A project ships to a set of store slots the way it ships in a set of
// languages, so this is a loop over both: the same screens and the same copy,
// drawn at each target's pixel size with the frame that belongs in it. One
// directory per pair, which is one upload.
//
// It replaces the contents of each directory it writes: a second export after
// deleting a screen must not leave the old file behind, or the upload picks it
// up.
func Run(ctx context.Context, s *store.Store, p model.Project, locales, sizeIDs []string, listing bool) (Result, error) {
	if len(locales) == 0 {
		locales = p.Locales
	}
	targets := chooseTargets(p, sizeIDs)
	if len(targets) == 0 {
		return Result{}, fmt.Errorf("this project ships to no store sizes")
	}

	// Decoded once, at the widest target: every target above the cached
	// preview width gets the original back anyway, so decoding per target
	// would be the same pixels read three times.
	widest := 0
	for _, t := range targets {
		if w := presets.Size(t.SizeID).W; w > widest {
			widest = w
		}
	}
	sources, err := loadSources(ctx, s, p, widest)
	if err != nil {
		return Result{}, err
	}

	var result Result
	for _, target := range targets {
		// The renderer takes one Settings, so "render at this target" is
		// "resolve the target into Settings first". p is a copy; Settings is a
		// value field, so this cannot leak into the stored document.
		at := p
		at.Use(target)
		size := presets.Size(target.SizeID)

		for _, locale := range locales {
			dir := s.ExportDir(at, locale, size.ID)
			if err := os.RemoveAll(dir); err != nil {
				return Result{}, err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return Result{}, err
			}

			files, err := renderLocale(ctx, at, locale, size, sources, dir)
			if err != nil {
				return Result{}, err
			}
			// The listing goes in the directory the pictures went in, because
			// the upload is the pair: App Store Connect asks for this
			// language's description on the same screen it asks for this
			// language's screenshots, and having to go back to the app to
			// copy the text is the step this is here to remove.
			//
			// Optional, because an export that is only being re-run for a
			// changed screenshot should not also drop a text file into a
			// folder somebody has already worked through.
			var listingFor string
			if listing {
				if err := writeListing(at, locale, dir); err != nil {
					return Result{}, err
				}
				listingFor = listingFile
			}
			result.Files += len(files)
			result.Sets = append(result.Sets, Set{
				SizeID: size.ID, Locale: locale, Dir: dir, Files: files, Listing: listingFor,
			})
		}
	}
	return result, nil
}

// chooseTargets narrows the project's targets to the ones asked for. An empty
// ask means all of them, and an id the project does not ship is ignored rather
// than invented: exporting a size that was never designed against would write
// a directory nobody chose.
func chooseTargets(p model.Project, sizeIDs []string) []model.Target {
	if len(sizeIDs) == 0 {
		return p.Targets()
	}
	var out []model.Target
	for _, id := range sizeIDs {
		if t, ok := p.TargetOf(id); ok {
			out = append(out, t)
		}
	}
	return out
}

// tile is one unit of work: one screen, which is one file. A screen that draws
// a slice of a wider composition carries which slice in itself, so there is
// nothing here to pair up.
type tile struct {
	screen model.Screen
	span   int
	name   string
}

func renderLocale(ctx context.Context, p model.Project, locale string, size model.ExportSize, sources map[string]image.Image, dir string) ([]string, error) {
	// One file per screen, numbered in store order, so the set uploads as
	// consecutive slots. A panorama's halves are two screens here and take two
	// numbers, wherever in the set they have been moved to.
	tiles := make([]tile, 0, len(p.Screens()))
	for i, screen := range p.Screens() {
		name := slug(render.StripMarkup(screen.Text(locale, p.BaseLocale).Headline))
		if name == "" {
			name = "screen"
		}
		span := render.Span(screen, p.Settings)
		if span > 1 {
			// Both halves are named from the same headline, so the suffix is
			// the only thing telling them apart in a directory listing.
			name = fmt.Sprintf("%s-%d", name, screen.Part+1)
		}
		tiles = append(tiles, tile{
			screen: screen,
			span:   span,
			name:   fmt.Sprintf("%02d-%s.%s", i+1, name, ext(p.Format)),
		})
	}

	// One goroutine per tile, bounded by sem to one render per core: a
	// ten-second wait becomes a two-second one, and nothing thrashes.
	files := make([]string, len(tiles))
	errs := make([]error, len(tiles))
	var wg sync.WaitGroup

	for i, t := range tiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = renderTile(p, locale, size, sources, dir, t)
			files[i] = t.name
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// renderTile draws one file. Split out of the loop so its acquire/release of
// the semaphore is a function's lifetime rather than a defer that would
// accumulate for every tile in the set.
func renderTile(p model.Project, locale string, size model.ExportSize, sources map[string]image.Image, dir string, t tile) error {
	sem <- struct{}{}
	defer func() { <-sem }()

	img := render.Scene(size.W, size.H, t.screen, p.Settings,
		t.screen.Text(locale, p.BaseLocale), neighbours(p, sources, t.screen.ID))

	// A span-2 composition is drawn once at double width and cut here, so the
	// seam is exact rather than two renders that nearly line up. Both halves
	// draw the whole composition and keep a different column of it.
	if t.span > 1 {
		img = crop(img, t.screen.Part*size.W, size.W)
	}
	return write(filepath.Join(dir, t.name), img, p.Format)
}

// sem bounds the render fan-out to the number of cores. A 1320x2868 render is
// a few hundred milliseconds of resampling and rasterising and there is one
// per tile per language; unbounded, a ten-language export would start eighty
// of them at once and spend its time in the scheduler.
var sem = make(chan struct{}, runtime.NumCPU())

// neighbours is the screenshot set one composition can reach: its own, and the
// two beside it. The wrap is deliberate — a three-device arrangement on the
// last screen shows the first one rather than a gap.
//
// Compositions, not screens. A panorama's second half holds a copy of the
// first's screenshot, so walking the raw list would make a screen its own
// neighbour and a trio would draw the same picture twice.
func neighbours(p model.Project, sources map[string]image.Image, screenID string) render.Sources {
	leads := p.Leads()
	i := p.LeadIndex(screenID)
	at := func(j int) image.Image {
		if len(leads) == 0 || i < 0 {
			return nil
		}
		j = ((j % len(leads)) + len(leads)) % len(leads)
		return sources[leads[j].AssetID]
	}
	return render.Sources{
		model.SourceSelf: at(i),
		model.SourceNext: at(i + 1),
		model.SourcePrev: at(i - 1),
	}
}

// loadSources decodes every screenshot once. Decoding per tile would decode a
// panorama's source twice and a trio's three times.
func loadSources(ctx context.Context, s *store.Store, p model.Project, targetW int) (map[string]image.Image, error) {
	assets, err := s.AssetsOf(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	byID := map[string]store.Asset{}
	for _, a := range assets {
		byID[a.Doc.ID] = a
	}

	out := map[string]image.Image{}
	for _, screen := range p.Screens() {
		if screen.AssetID == "" || out[screen.AssetID] != nil {
			continue
		}
		asset, ok := byID[screen.AssetID]
		if !ok {
			continue
		}
		img, err := s.Source(p, asset, targetW)
		if err != nil {
			return nil, err
		}
		out[screen.AssetID] = img
	}
	return out, nil
}

func crop(img *image.RGBA, x, w int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, w, img.Bounds().Dy()))
	for y := range img.Bounds().Dy() {
		copy(out.Pix[y*out.Stride:(y+1)*out.Stride], img.Pix[y*img.Stride+x*4:])
	}
	return out
}

func write(path string, img image.Image, format string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if format == model.FormatJPEG {
		// App Store Connect rejects an image carrying an alpha channel, and a
		// PNG encoder writes RGBA even when every pixel is opaque. JPEG is the
		// way out of that, not a quality preference — hence 95, not 80.
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	}
	return png.Encode(f, img)
}

func ext(format string) string {
	if format == model.FormatJPEG {
		return "jpg"
	}
	return "png"
}

// slug makes a filename fragment out of a headline. Non-ASCII collapses to
// nothing, which is why a Japanese set's files are numbered rather than named
// — the number is the part the store cares about anyway.
func slug(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

// listingFile is the metadata beside the pictures. Named for what it is rather
// than after a store's import format: nothing consumes this but a person with
// App Store Connect open in another window, and a .txt they can read is worth
// more to them than a schema they cannot.
const listingFile = "listing.txt"

// writeListing puts one language's store text in that language's directory.
//
// Plain text, in the order App Store Connect asks for it, with the character
// count against the limit on each heading — because the one thing an author
// wants to know while pasting is whether it will be rejected. The version's
// own name and copyright are at the top: they are per version, not per
// language, and a directory that did not say which version it was would be
// indistinguishable from the last one after two releases.
func writeListing(p model.Project, locale, dir string) error {
	v := p.Version()
	m := v.Text(locale, p.BaseLocale)

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s — %s\n", p.Name, v.Name, locale)
	if v.Text(locale, p.BaseLocale) != v.Own(locale) {
		fmt.Fprintf(&b, "(no %s listing yet — this is %s)\n", locale, p.BaseLocale)
	}
	b.WriteString("\n")

	field := func(label string, value string, max int) {
		fmt.Fprintf(&b, "== %s (%d/%d) ==\n", label, len([]rune(value)), max)
		if value == "" {
			b.WriteString("(empty)\n\n")
			return
		}
		b.WriteString(value)
		b.WriteString("\n\n")
	}
	field("Promotional text", m.Promotional, model.MaxPromotional)
	field("Description", m.Description, model.MaxDescription)
	field("What's New in This Version", m.WhatsNew, model.MaxWhatsNew)
	field("Keywords", m.Keywords, model.MaxKeywords)
	field("Support URL", m.SupportURL, model.MaxURL)
	field("Marketing URL", m.MarketingURL, model.MaxURL)
	field("Copyright", v.Copyright, model.MaxCopyright)

	return os.WriteFile(filepath.Join(dir, listingFile), []byte(b.String()), 0o644)
}
