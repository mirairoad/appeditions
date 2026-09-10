package store

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// builtin resolves a shipped template to the document id CreateProject and
// ApplyTemplate take. Ids are minted per machine, so a test cannot spell one.
func builtin(t *testing.T, s *Store, key string) string {
	t.Helper()
	row, err := s.BuiltinTemplate(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return row.Doc.ID
}

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSeedsBuiltinTemplates(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	templates, err := s.ListTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != len(presets.BuiltinTemplates) {
		t.Fatalf("seeded %d templates, expected %d", len(templates), len(presets.BuiltinTemplates))
	}

	// Seeding twice must not duplicate: the unique key is what enforces that,
	// and it is the kind of constraint that only shows up on the second start.
	if err := s.seedTemplates(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := s.ListTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(templates) {
		t.Fatalf("reseeding produced %d templates, was %d", len(again), len(templates))
	}
}

func TestBuiltinTemplateIsRestoredAfterEditing(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	notebook, err := s.BuiltinTemplate(ctx, "classic")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Templates.Patch(ctx, notebook.Doc.ID, func(dst *Template) {
		dst.Label = "Ruined"
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.seedTemplates(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := s.BuiltinTemplate(ctx, "classic")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Label != "Light" {
		t.Errorf("built-in was not restored: %q", restored.Label)
	}
}

func TestCreateProjectAppliesTheTemplate(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	p, err := s.CreateProject(ctx, model.Project{
		Name:       "Nobiru",
		BaseLocale: "ja",
		Locales:    []string{"ja", "en-US"},
	}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	if p.Slug != "nobiru" {
		t.Errorf("slug is %q", p.Slug)
	}
	// A built-in lays out one slot per composition structure, so a new project
	// opens on the whole range of them and the author deletes down. Two of the
	// nine span two store tiles, which is why the set is eleven screens.
	if got := len(p.Leads()); got != len(presets.EveryStructure()) {
		t.Fatalf("expected %d compositions, got %d", len(presets.EveryStructure()), got)
	}
	if len(p.Screens()) != 11 {
		t.Fatalf("expected 11 tiles for 9 compositions with two panoramas, got %d", len(p.Screens()))
	}
	if p.Screens()[0].Copy["ja"].Headline == "" {
		t.Error("empty slot carries no sample copy in the base language")
	}
	if p.Settings.Layout != "hero" {
		t.Errorf("template settings not applied: layout is %q", p.Settings.Layout)
	}
	if p.Settings.SizeID != "iphone-6-9" {
		t.Errorf("export size should survive a template: %q", p.Settings.SizeID)
	}

	if _, err := os.Stat(s.AssetsDir(p)); err != nil {
		t.Errorf("assets directory was not created: %v", err)
	}
}

func TestSlugsDoNotCollide(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	first, err := s.CreateProject(ctx, model.Project{Name: "Nobiru", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateProject(ctx, model.Project{Name: "Nobiru", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Slug == second.Slug {
		t.Fatalf("both projects took the directory %q", first.Slug)
	}
}

func TestAddShotsFillsSlotsThenAppends(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	p, err := s.CreateProject(ctx, model.Project{Name: "Shots", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	// More files than the template has slots: each slot takes one in order and
	// the rest are appended. Counted in *compositions* — a panorama is one
	// drawing carried as two screens, and the number that matters here is how
	// many pictures the set holds.
	slots := len(p.Leads())
	var assets []Asset
	for i := range slots + 3 {
		a, err := s.PutAsset(ctx, p, fmt.Sprintf("%02d-screen.png", i+1), fakePNG(t, 40+i))
		if err != nil {
			t.Fatal(err)
		}
		assets = append(assets, a)
	}
	p, err = s.AddShots(ctx, p.ID, assets)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(p.Leads()); got != slots+3 {
		t.Fatalf("expected %d compositions, got %d", slots+3, got)
	}
	for i, screen := range p.Screens() {
		if !screen.Filled() {
			t.Errorf("screen %d is still empty", i)
		}
	}
	// The slots kept their sample copy; the appended ones took their filename.
	if p.Leads()[0].Copy["en-US"].Headline == "" {
		t.Error("a filled slot lost its sample copy")
	}
	last := p.Leads()[slots+2]
	if got := last.Copy["en-US"].Headline; got != name(slots+3) {
		t.Errorf("appended screen's headline is %q, want %q", got, name(slots+3))
	}
}

// name is the headline AddShots derives from the nth uploaded file.
func name(n int) string {
	return fmt.Sprintf("%02d screen", n)
}

func TestPutAssetDeduplicates(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateProject(ctx, model.Project{Name: "Dedup", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	data := fakePNG(t, 60)
	first, err := s.PutAsset(ctx, p, "a.png", data)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.PutAsset(ctx, p, "a-copy.png", data)
	if err != nil {
		t.Fatal(err)
	}
	if first.Doc.ID != second.Doc.ID {
		t.Errorf("the same bytes produced two assets: %s and %s", first.Doc.ID, second.Doc.ID)
	}

	entries, err := os.ReadDir(s.AssetsDir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("wrote %d files for one image", len(entries))
	}
	if first.Width != 60 {
		t.Errorf("dimensions not read: %dx%d", first.Width, first.Height)
	}
}

func TestEditCopyAndLocales(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateProject(ctx, model.Project{Name: "Words", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.AddShots(ctx, p.ID, []Asset{mustAsset(t, s, p, "one.png")})
	if err != nil {
		t.Fatal(err)
	}

	id := p.Screens()[0].ID
	p, err = s.Edit(ctx, p.ID, func(p *model.Project) {
		p.AddLocale("ja")
		p.SetCopy(id, "en-US", model.Copy{Headline: "Everything in *one place*"})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Japanese has no words yet, so it falls back — adding a language must
	// never blank the preview.
	if got := p.Screens()[0].Text("ja", p.BaseLocale).Headline; got != "Everything in *one place*" {
		t.Errorf("fallback failed: %q", got)
	}
	// The template lays out three slots, so the other two are missing Japanese
	// too. What this asserts is about *this* screen: it is on the list before
	// its words are written and off it afterwards.
	if !contains(p.MissingCopy("ja"), id) {
		t.Errorf("screen not counted as missing copy: %v", p.MissingCopy("ja"))
	}

	p, err = s.Edit(ctx, p.ID, func(p *model.Project) {
		p.SetCopy(id, "ja", model.Copy{Headline: "すべてを*ひとつ*に"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Screens()[0].Text("ja", p.BaseLocale).Headline; got != "すべてを*ひとつ*に" {
		t.Errorf("Japanese copy is %q", got)
	}
	if contains(p.MissingCopy("ja"), id) {
		t.Error("screen still counted as missing copy")
	}

	// Removing a language takes its words with it; the base language cannot be
	// removed at all.
	p, err = s.Edit(ctx, p.ID, func(p *model.Project) {
		p.RemoveLocale("ja")
		p.RemoveLocale("en-US")
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.HasLocale("ja") {
		t.Error("ja was not removed")
	}
	if !p.HasLocale("en-US") {
		t.Error("the base language was removed")
	}
	if _, ok := p.Screens()[0].Copy["ja"]; ok {
		t.Error("Japanese copy outlived its language")
	}
}

func TestSaveTemplateCapturesTheVariants(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateProject(ctx, model.Project{Name: "Look", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	saved, err := s.SaveTemplate(ctx, p.ID, "My look", "Tuned for release 2.4")
	if err != nil {
		t.Fatal(err)
	}
	// One variant per composition, not per screen. A panorama is one drawing
	// carried as two screens; saving both halves would add a slot to the set
	// on every save-and-apply round trip.
	if len(saved.Variants) != len(p.Leads()) {
		t.Fatalf("saved %d variants for %d compositions", len(saved.Variants), len(p.Leads()))
	}
	if saved.Builtin {
		t.Error("a saved template must not claim to be built in")
	}

	// Applying it to a fresh project must reproduce the look.
	other, err := s.CreateProject(ctx, model.Project{Name: "Other", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	other, err = s.ApplyTemplate(ctx, other.ID, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if other.Settings.Layout != p.Settings.Layout {
		t.Errorf("layout is %q, expected %q", other.Settings.Layout, p.Settings.Layout)
	}
	// Compositions, not screens: a template saved from a set with a panorama
	// in it must lay out the same number of drawings, not one extra per half.
	if len(other.Leads()) != len(p.Leads()) {
		t.Errorf("laid out %d compositions, expected %d", len(other.Leads()), len(p.Leads()))
	}
	if len(other.Screens()) != len(p.Screens()) {
		t.Errorf("laid out %d tiles, expected %d", len(other.Screens()), len(p.Screens()))
	}
}

// A look is not separable from the pictures it was judged on. A template that
// kept the words and threw the captures away came back, months later, as a
// layout of empty boxes.
func TestSaveTemplateKeepsTheScreenshots(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateProject(ctx, model.Project{Name: "Shot look", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	var assets []Asset
	for i := range len(p.Leads()) {
		a, err := s.PutAsset(ctx, p, fmt.Sprintf("%02d-screen.png", i+1), fakePNG(t, 70+i))
		if err != nil {
			t.Fatal(err)
		}
		assets = append(assets, a)
	}
	if p, err = s.AddShots(ctx, p.ID, assets); err != nil {
		t.Fatal(err)
	}

	saved, err := s.SaveTemplate(ctx, p.ID, "With pictures", "")
	if err != nil {
		t.Fatal(err)
	}
	for i, sample := range saved.Samples {
		if sample.Shot == "" {
			t.Fatalf("slot %d saved no screenshot", i)
		}
		if _, err := os.Stat(filepath.Join(s.TemplateDir(saved.ID), sample.Shot)); err != nil {
			t.Fatalf("slot %d names %q, which is not on disk: %v", i, sample.Shot, err)
		}
	}
	// The language the words were written in, recorded rather than assumed.
	if saved.Locale != "en-US" {
		t.Errorf("template says its samples are in %q, want en-US", saved.Locale)
	}

	// A fresh project made from it comes up with pictures in every slot, and
	// they are its own assets — replaceable, deletable, listed in Media.
	other, err := s.CreateProject(ctx, model.Project{Name: "From template", BaseLocale: "en-US", Locales: []string{"en-US"}}, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, screen := range other.Screens() {
		if !screen.Filled() {
			t.Fatalf("screen %d came up empty", i)
		}
	}
	own, err := s.AssetsOf(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(own) != len(saved.Samples) {
		t.Errorf("the new project holds %d originals, want %d", len(own), len(saved.Samples))
	}
	// Deleting the project the template was saved from must not take the
	// template's pictures with it: that project is the one most likely to go,
	// because the template is what replaced it.
	if err := s.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateProject(ctx, model.Project{Name: "After", BaseLocale: "en-US", Locales: []string{"en-US"}}, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Screens()[0].Filled() {
		t.Error("the template lost its screenshots when the project that made it was deleted")
	}
}

// Applying a template to a set that already has screenshots must not import
// the template's own: the slots they would fill are the ones the project's own
// pictures keep, so they would land in Media used by nothing.
func TestApplyTemplateKeepsTheProjectsOwnShots(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	source, err := s.CreateProject(ctx, model.Project{Name: "Source", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	var assets []Asset
	for i := range len(source.Leads()) {
		a, err := s.PutAsset(ctx, source, fmt.Sprintf("%02d-source.png", i+1), fakePNG(t, 90+i))
		if err != nil {
			t.Fatal(err)
		}
		assets = append(assets, a)
	}
	if source, err = s.AddShots(ctx, source.ID, assets); err != nil {
		t.Fatal(err)
	}
	saved, err := s.SaveTemplate(ctx, source.ID, "Source look", "")
	if err != nil {
		t.Fatal(err)
	}

	target, err := s.CreateProject(ctx, model.Project{Name: "Target", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	mine, err := s.PutAsset(ctx, target, "mine.png", fakePNG(t, 7))
	if err != nil {
		t.Fatal(err)
	}
	if target, err = s.AddShots(ctx, target.ID, []Asset{mine}); err != nil {
		t.Fatal(err)
	}
	if target, err = s.ApplyTemplate(ctx, target.ID, saved.ID); err != nil {
		t.Fatal(err)
	}

	if got := target.Leads()[0].AssetID; got != mine.Doc.ID {
		t.Errorf("the first slot draws %q, want the project's own capture %q", got, mine.Doc.ID)
	}
	own, err := s.AssetsOf(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Its own, plus one per slot the template filled.
	want := 1 + len(saved.Samples) - 1
	if len(own) != want {
		t.Errorf("the project holds %d originals, want %d", len(own), want)
	}
}

func TestApplyRhythmLeavesColoursAlone(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateProject(ctx, model.Project{Name: "Rhythm", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	// By id, not by position: the rhythm's first step is a panorama, and a
	// panorama adds its second half to the set — so the screen that was at
	// index 1 is at index 2 afterwards.
	watched := p.Screens()[1].ID
	before := p.Screens()[1].Overrides.Apply(p.Settings).Background

	p, err = s.ApplyRhythm(ctx, p.ID, "editorial")
	if err != nil {
		t.Fatal(err)
	}
	if p.Screens()[0].Overrides.Layout == nil || *p.Screens()[0].Overrides.Layout != "panorama" {
		t.Error("the rhythm did not pin the first screen's layout")
	}
	if p.Screens()[1].Of != p.Screens()[0].ID {
		t.Errorf("the panorama's second half is not in the set: %+v", p.Screens()[1])
	}
	after, ok := p.Screen(watched)
	if !ok {
		t.Fatalf("screen %s is gone", watched)
	}
	if got := after.Overrides.Apply(p.Settings).Background; got != before {
		t.Errorf("the rhythm disturbed a screen's colour: %v, was %v", got, before)
	}
}

func TestReopenKeepsEverything(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	first, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := first.CreateProject(ctx, model.Project{Name: "Persist", BaseLocale: "en-US", Locales: []string{"en-US"}}, builtin(t, first, "classic"))
	if err != nil {
		t.Fatal(err)
	}
	first.Close()

	second, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	again, err := second.Project(ctx, p.ID)
	if err != nil {
		t.Fatalf("project did not survive a restart: %v", err)
	}
	if again.Name != "Persist" {
		t.Errorf("name is %q", again.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "appeditions.db")); err != nil {
		t.Errorf("database file: %v", err)
	}
}

func mustAsset(t *testing.T, s *Store, p model.Project, name string) Asset {
	t.Helper()
	a, err := s.PutAsset(context.Background(), p, name, fakePNG(t, 32))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// fakePNG is a solid square. The width doubles as the "different bytes" knob
// for the deduplication test.
func fakePNG(t *testing.T, width int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, width*2))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 251)
	}
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func contains(ids []string, id string) bool {
	for _, s := range ids {
		if s == id {
			return true
		}
	}
	return false
}

// Versions survive a round trip through the store, with their own slots.
//
// Guards the shape of the document, not one method: the screens and the store
// slots moved onto the version in two separate steps, and the failure mode both
// times was a project that came back with one version where it had three.
func TestVersionsRoundTripWithTheirOwnSlots(t *testing.T) {
	ctx := context.Background()
	s := open(t)

	p, err := s.CreateProject(ctx, model.Project{
		Name:       "Nobiru",
		BaseLocale: "en-US",
		Locales:    []string{"en-US"},
		Versions: []model.Version{{ID: "v_1", Name: "1.0", Targets: []model.Target{
			presets.Target("iphone-6-9"), presets.Target("ipad-13"),
		}}},
		VersionID: "v_1",
	}, builtin(t, s, "classic"))
	if err != nil {
		t.Fatal(err)
	}

	// A new version inherits the slots, then drops one.
	p, err = s.Edit(ctx, p.ID, func(p *model.Project) { p.AddVersion("1.1") })
	if err != nil {
		t.Fatal(err)
	}
	if got := len(p.Targets()); got != 2 {
		t.Fatalf("a new version starts with %d slots, want the 2 it was copied from", got)
	}
	p, err = s.Edit(ctx, p.ID, func(p *model.Project) { p.RemoveTarget("ipad-13") })
	if err != nil {
		t.Fatal(err)
	}

	// Read it back the way every page does.
	fresh, err := s.Project(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Versions) != 2 {
		t.Fatalf("project came back with %d versions, want 2", len(fresh.Versions))
	}
	if got := len(fresh.Versions[0].Targets); got != 2 {
		t.Errorf("1.0 ships to %d slots, want 2", got)
	}
	if got := len(fresh.Versions[1].Targets); got != 1 {
		t.Errorf("1.1 ships to %d slots, want 1 — a release may ship to fewer", got)
	}
	// And the one being edited is still the one that was selected.
	if fresh.VersionID != p.VersionID {
		t.Errorf("selected version is %q, want %q", fresh.VersionID, p.VersionID)
	}
}
