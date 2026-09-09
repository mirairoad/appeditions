package exporter

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/internal/store"
)

func project(t *testing.T) (*store.Store, model.Project) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	p, err := s.CreateProject(ctx, model.Project{
		Name:       "Nobiru",
		BaseLocale: "en-US",
		Locales:    []string{"en-US", "ja"},
	}, "classic")
	if err != nil {
		t.Fatal(err)
	}

	var assets []store.Asset
	for range 3 {
		a, err := s.PutAsset(ctx, p, "shot.png", shot(t, len(assets)))
		if err != nil {
			t.Fatal(err)
		}
		assets = append(assets, a)
	}
	if p, err = s.AddShots(ctx, p.ID, assets); err != nil {
		t.Fatal(err)
	}

	p, err = s.Edit(ctx, p.ID, func(p *model.Project) {
		for i := range p.Screens() {
			p.SetCopy(p.Screens()[i].ID, "en-US", model.Copy{Headline: "Everything in *one place*"})
			p.SetCopy(p.Screens()[i].ID, "ja", model.Copy{Headline: "すべてが*ひとつ*に"})
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

func TestExportWritesOneDirectoryPerLanguage(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	result, err := Run(ctx, s, p, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	// One file per store tile per language. Counted against the set, not a
	// literal: the built-in template's slot count is a product decision and
	// this test is about the export writing one directory per language.
	want := len(p.Screens()) * len(p.Locales)
	if result.Files != want {
		t.Fatalf("wrote %d files for %d screens in %d languages, want %d",
			result.Files, len(p.Screens()), len(p.Locales), want)
	}

	size := presets.Size(p.Settings.SizeID)
	for _, locale := range p.Locales {
		dir := s.ExportDir(p, locale, size.ID)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		// Every tile, and the listing.
		if len(entries) != len(p.Screens())+1 {
			t.Errorf("%s has %d files", locale, len(entries))
		}

		// The first file must open at exactly the store's pixel size, because
		// that is the only thing the store checks before rejecting an upload.
		f, err := os.Open(filepath.Join(dir, entries[0].Name()))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != size.W || cfg.Height != size.H {
			t.Errorf("%s: %dx%d, want %dx%d", entries[0].Name(), cfg.Width, cfg.Height, size.W, size.H)
		}
	}

	// The English files are named after the English headline. The Japanese
	// ones cannot be, so they fall back to the number and a stub.
	english, _ := os.ReadDir(s.ExportDir(p, "en-US", size.ID))
	if !strings.Contains(english[0].Name(), "everything-in-one-place") {
		t.Errorf("filename does not carry the headline: %q", english[0].Name())
	}
}

func TestPanoramaIsSlicedIntoTiles(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	// A single-tile composition becomes a panorama, which is one drawing cut
	// into two files — so the set gains exactly one screen.
	before := len(p.Screens())
	single := ""
	for _, sc := range p.Leads() {
		if presets.Span(sc, p.Settings) == 1 {
			single = sc.ID
			break
		}
	}
	if single == "" {
		t.Fatal("no single-tile composition in the fixture to widen")
	}

	panorama := "panorama"
	p, err := s.Edit(ctx, p.ID, func(p *model.Project) {
		if sc, ok := p.Screen(single); ok {
			sc.Overrides.Layout = &panorama
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(p.Screens()); got != before+1 {
		t.Fatalf("a panorama left the set at %d screens, want %d", got, before+1)
	}

	result, err := Run(ctx, s, p, []string{"en-US"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	// One file per screen.
	if result.Files != before+1 {
		t.Fatalf("wrote %d files, expected 4", result.Files)
	}

	size := presets.Size(p.Settings.SizeID)
	entries, _ := os.ReadDir(s.ExportDir(p, "en-US", size.ID))
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	// Numbered in store order: the panorama's halves are slots 1 and 2, so the
	// hero that follows is slot 3 and nothing has to be renamed on upload.
	if !strings.HasPrefix(names[0], "01-") || !strings.Contains(names[0], "-1.png") {
		t.Errorf("first tile is %q", names[0])
	}
	if !strings.HasPrefix(names[1], "02-") || !strings.Contains(names[1], "-2.png") {
		t.Errorf("second tile is %q", names[1])
	}

	f, _ := os.Open(filepath.Join(s.ExportDir(p, "en-US", size.ID), names[0]))
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != size.W {
		t.Errorf("a sliced tile is %d wide, want %d", cfg.Width, size.W)
	}
}

func TestExportReplacesTheDirectory(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)
	size := presets.Size(p.Settings.SizeID)
	dir := s.ExportDir(p, "en-US", size.ID)

	if _, err := Run(ctx, s, p, []string{"en-US"}, nil, true); err != nil {
		t.Fatal(err)
	}
	// A screen removed between exports must not leave its file behind: the
	// upload would pick up a screenshot the set no longer contains.
	p, err := s.Edit(ctx, p.ID, func(p *model.Project) { p.RemoveScreen(p.Screens()[2].ID) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, s, p, []string{"en-US"}, nil, true); err != nil {
		t.Fatal(err)
	}

	// The directory holds exactly what the set holds — no more, which is the
	// point: a screen removed between exports must not leave its file behind.
	// Counted against the set rather than a literal, so the fixture's size is
	// free to change without this quietly asserting nothing.
	entries, _ := os.ReadDir(dir)
	var tiles int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".png" {
			tiles++
		}
	}
	if tiles != len(p.Screens()) {
		t.Errorf("directory holds %d tiles for a set of %d", tiles, len(p.Screens()))
	}
	if _, err := os.Stat(filepath.Join(dir, listingFile)); err != nil {
		t.Errorf("no listing beside the pictures: %v", err)
	}
}

// The listing goes in the same directory as that language's tiles, because the
// upload is the pair — App Store Connect asks for both on one screen.
func TestExportWritesTheListingPerLanguage(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	p, err := s.Edit(ctx, p.ID, func(p *model.Project) {
		v := p.EditVersion()
		v.Name = "1.4"
		v.Copyright = "2026 Nobiru"
		v.SetStore("en-US", model.Metadata{Description: "Everything in one place.", Keywords: "lists,voice"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, s, p, []string{"en-US", "ja"}, nil, true); err != nil {
		t.Fatal(err)
	}

	size := presets.Size(p.Settings.SizeID)
	body, err := os.ReadFile(filepath.Join(s.ExportDir(p, "en-US", size.ID), listingFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1.4", "Everything in one place.", "lists,voice", "2026 Nobiru", "Description (24/4000)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("listing is missing %q:\n%s", want, body)
		}
	}

	// Japanese has no listing of its own, so it gets the base language's words
	// and says so — an empty file would read as "nothing to submit".
	ja, err := os.ReadFile(filepath.Join(s.ExportDir(p, "ja", size.ID), listingFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ja), "Everything in one place.") {
		t.Error("Japanese listing did not fall back to the base language")
	}
	if !strings.Contains(string(ja), "no ja listing yet") {
		t.Error("Japanese listing does not say it is a fallback")
	}
}

func TestPreviewTagChangesWithTheLook(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	first, tag, err := Preview(ctx, s, p, p.Screens()[0].ID, "en-US", 240)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("preview is empty")
	}

	// Same inputs, same tag — this is what makes the editor's reload of every
	// tile mostly 304s.
	_, again, err := Preview(ctx, s, p, p.Screens()[0].ID, "en-US", 240)
	if err != nil {
		t.Fatal(err)
	}
	if tag != again {
		t.Errorf("tag changed with no edit: %s then %s", tag, again)
	}

	// A different language is a different picture.
	_, japanese, err := Preview(ctx, s, p, p.Screens()[0].ID, "ja", 240)
	if err != nil {
		t.Fatal(err)
	}
	if tag == japanese {
		t.Error("the Japanese preview carries the English tag")
	}
}

// The tile URL carries the tag and is cached immutable, so anything the
// drawing reads and the tag does not is a tile that never updates again. A
// two-device arrangement draws model.SourceNext — the *neighbour's*
// screenshot — which is what this guards.
func TestPreviewTagFollowsTheNeighbourScreenshot(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	// Layout *and* arrangement, on every screen: presets.Duo is the placement
	// set that draws model.SourceNext, and the layout alone would not reach
	// for it. Set on the project so no screen's own variant disagrees.
	p, err := s.Edit(ctx, p.ID, func(p *model.Project) {
		p.Settings.Layout = "duo"
		p.Settings.PositionID = "duo"
		p.Unpin("layout")
		p.Unpin("position_id")
	})
	if err != nil {
		t.Fatal(err)
	}

	// Compositions, not screens: the neighbour a tile draws is the next
	// *drawing*, and indexing Screens would land on a panorama's second half —
	// which carries a copy of its lead's picture and is overwritten by
	// SyncParts the moment it is changed.
	leads := p.Leads()
	first, second := leads[0].ID, leads[1].ID

	_, before, err := Preview(ctx, s, p, first, "en-US", 240)
	if err != nil {
		t.Fatal(err)
	}

	// The first composition is untouched; its neighbour gets a different
	// picture.
	swapped, err := s.Edit(ctx, p.ID, func(p *model.Project) {
		if sc, ok := p.Screen(second); ok {
			sc.AssetID = "some-other-asset"
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	_, after, err := Preview(ctx, s, swapped, first, "en-US", 240)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Errorf("tag ignored the neighbour's screenshot: %s twice", before)
	}
}

func TestPreviewOfAPanoramaIsOneTile(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	panorama := "panorama"
	p, err := s.Edit(ctx, p.ID, func(p *model.Project) {
		p.Screens()[0].Overrides.Layout = &panorama
	})
	if err != nil {
		t.Fatal(err)
	}

	size := presets.Size(p.Settings.SizeID)
	const width = 240
	// Two screens, and each of them draws one store tile. Neither ever draws
	// the un-sliced composition: it is nearly square, sits wrong beside the
	// portrait tiles, and hides where the cut falls.
	for _, screen := range p.Screens()[:2] {
		body, tag, err := Preview(ctx, s, p, screen.ID, "en-US", width)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != width {
			t.Errorf("%s is %d wide, want one tile (%d)", screen.ID, cfg.Width, width)
		}
		if want := width * size.H / size.W; cfg.Height != want {
			t.Errorf("%s is %d tall, want %d", screen.ID, cfg.Height, want)
		}
		if tag == "" {
			t.Errorf("%s has no tag", screen.ID)
		}
	}

	// The two halves must be different pictures, or the cut is not happening.
	left, leftTag, _ := Preview(ctx, s, p, p.Screens()[0].ID, "en-US", width)
	right, rightTag, _ := Preview(ctx, s, p, p.Screens()[1].ID, "en-US", width)
	if bytes.Equal(left, right) {
		t.Error("both halves of the panorama are the same image")
	}
	// And they must not share an ETag, or the editor shows one of them twice:
	// everything else about the two screens is identical by design.
	if leftTag == rightTag {
		t.Error("the halves carry the same tag")
	}
}

// shot is a distinguishable fake screenshot, one per index.
func shot(t *testing.T, i int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 300, 650))
	for p := range img.Pix {
		img.Pix[p] = uint8((p + i*37) % 251)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The listing is optional. Re-running an export for a changed screenshot
// should not drop a text file back into a folder somebody has worked through.
func TestExportCanSkipTheListing(t *testing.T) {
	ctx := context.Background()
	s, p := project(t)

	result, err := Run(ctx, s, p, []string{"en-US"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	size := presets.Size(p.Settings.SizeID)
	if _, err := os.Stat(filepath.Join(s.ExportDir(p, "en-US", size.ID), listingFile)); !os.IsNotExist(err) {
		t.Errorf("a listing was written when it was not asked for: %v", err)
	}
	for _, set := range result.Sets {
		if set.Listing != "" {
			t.Errorf("the result claims a listing at %q", set.Listing)
		}
	}

	// And it comes back when it is asked for, into the same directory the
	// export just replaced.
	if _, err := Run(ctx, s, p, []string{"en-US"}, nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.ExportDir(p, "en-US", size.ID), listingFile)); err != nil {
		t.Errorf("no listing after asking for one: %v", err)
	}
}
