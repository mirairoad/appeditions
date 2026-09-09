package store

import (
	"encoding/json"
	"testing"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// The claim this package makes is that one implementation of the rules runs in
// two places: the browser applies an op locally for an instant repaint, and the
// server applies the same op to the stored document. These tests are what stop
// that claim quietly becoming false.

func project() model.Project {
	p := model.Project{
		Name:       "Nobiru",
		BaseLocale: "en-US",
		Locales:    []string{"en-US", "ja"},
		Settings:   model.DefaultSettings(),
		Versions:   []model.Version{{ID: "v_1", Name: "1.0", Targets: []model.Target{presets.Target("iphone-6-9")}}},
		VersionID:  "v_1",
	}
	for _, id := range []string{"a", "b", "c"} {
		p.SetScreens(append(p.Screens(), model.Screen{
			ID:      "s_" + id,
			AssetID: "asset_" + id,
			Copy:    map[string]model.Copy{"en-US": {Headline: "Everything in *one place*"}},
		}))
	}
	return p
}

func encode(t *testing.T, p model.Project) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestBothPathsAgree is the load-bearing one. The browser goes through the
// store; the server goes through ApplyTo on a document it already holds. If
// those ever diverge, an optimistic update starts lying about what was saved.
func TestBothPathsAgree(t *testing.T) {
	ops := []Op{
		{Kind: OpSetting, Key: "tilt", Value: "7"},
		{Kind: OpSetting, Key: "background", Value: "gradient:#6366f1:#a855f7:135"},
		{Kind: OpSetting, ScreenID: "s_b", Key: "layout", Value: "panorama"},
		{Kind: OpRhythm, Value: "editorial"},
		{Kind: OpMove, ScreenID: "s_c", Delta: -1},
		{Kind: OpCopy, ScreenID: "s_a", Locale: "ja", Headline: "すべてを*ひとつ*に"},
		{Kind: OpLocale, Locale: "ko"},
		{Kind: OpReset, ScreenID: "s_b", Key: "layout"},
		{Kind: OpRemove, ScreenID: "s_a"},
	}

	for _, op := range ops {
		t.Run(op.Kind+"/"+op.Key, func(t *testing.T) {
			// The browser's path.
			browser := NewEditorStore()
			browser.Restore(EditorSnapshot{Project: project()})
			if err := browser.Apply(op); err != nil {
				t.Fatalf("store apply: %v", err)
			}

			// The server's path: the same function, against the document the
			// patch closure was handed.
			server := project()
			if err := ApplyTo(&server, op); err != nil {
				t.Fatalf("ApplyTo: %v", err)
			}

			if got, want := encode(t, browser.Project()), encode(t, server); got != want {
				t.Errorf("the two paths produced different documents\n browser: %s\n server:  %s", got, want)
			}
		})
	}
}

// TestSnapshotRoundTrips covers the wire: the browser is hydrated from JSON the
// server encoded, so anything the model gains has to survive the trip.
func TestSnapshotRoundTrips(t *testing.T) {
	before := EditorSnapshot{Project: project(), Rev: 3}
	wire, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var after EditorSnapshot
	if err := json.Unmarshal(wire, &after); err != nil {
		t.Fatal(err)
	}
	if encode(t, after.Project) != encode(t, before.Project) {
		t.Error("the project did not survive the wire")
	}

	s := NewEditorStore()
	s.Restore(after)
	if got := s.Project().Name; got != "Nobiru" {
		t.Errorf("restored name is %q", got)
	}
}

// TestDerivedReadsMatchTheRenderer guards the reason Span moved out of the
// renderer: the browser has to answer "how many tiles is this screen" with the
// same number the export will produce.
func TestDerivedReadsMatchTheRenderer(t *testing.T) {
	p := project()
	if err := ApplyTo(&p, Op{Kind: OpSetting, ScreenID: "s_b", Key: "layout", Value: "panorama"}); err != nil {
		t.Fatal(err)
	}

	spans := Spans(p)
	if spans["s_a"] != 1 {
		t.Errorf("a single-tile screen spans %d", spans["s_a"])
	}
	if spans["s_b"] != 2 {
		t.Errorf("a panorama spans %d, want 2", spans["s_b"])
	}

	// The right half is a screen of its own, so it is measured like one: four
	// screens, four files, four store tiles.
	r := Readiness(p, "en-US")
	if r.Total != 4 || r.Filled != 4 {
		t.Errorf("readiness counted %d of %d", r.Filled, r.Total)
	}
	if r.Tiles != 4 {
		t.Errorf("three screens with one panorama is %d tiles, want 4", r.Tiles)
	}
}

// TestAPanoramaIsTwoOrdinaryScreens is the claim the parts model exists for. A
// two-tile composition uploads as two files, and the author has to be able to
// treat them as two files: move either one, put them in an order that breaks
// the picture, or drop the pair.
func TestAPanoramaIsTwoOrdinaryScreens(t *testing.T) {
	p := project()
	apply := func(t *testing.T, op Op) {
		t.Helper()
		if err := ApplyTo(&p, op); err != nil {
			t.Fatal(err)
		}
	}

	apply(t, Op{Kind: OpSetting, ScreenID: "s_b", Key: "layout", Value: "panorama"})
	if len(p.Screens()) != 4 {
		t.Fatalf("a panorama produced %d screens, want 4", len(p.Screens()))
	}
	half := p.Screens()[2]
	if half.Of != "s_b" || half.Part != 1 {
		t.Fatalf("the second half is %+v", half)
	}
	// It shares the picture it is cut from, so every read path — the renderer,
	// the readiness count — sees a complete screen.
	if half.AssetID != "asset_b" {
		t.Errorf("the half carries %q, want the lead's screenshot", half.AssetID)
	}

	// Out of order on purpose. This is the thing that could not be expressed
	// while a panorama was one screen with two tiles.
	apply(t, Op{Kind: OpMove, ScreenID: half.ID, Delta: 1})
	if got := p.Screens()[3].ID; got != half.ID {
		t.Errorf("the half is at %v, want it moved past the next screen", ids(p))
	}
	if len(p.Screens()) != 4 {
		t.Errorf("moving a half changed the set to %v", ids(p))
	}

	// The words belong to the composition, so writing them on either screen
	// writes them once.
	apply(t, Op{Kind: OpCopy, ScreenID: half.ID, Locale: "en-US", Headline: "One *wide* shot"})
	if got := p.Screens()[1].Copy["en-US"].Headline; got != "One *wide* shot" {
		t.Errorf("the lead says %q", got)
	}
	if got := p.Screens()[3].Copy["en-US"].Headline; got != "One *wide* shot" {
		t.Errorf("the half says %q", got)
	}

	// Deleting either half deletes the composition: the other one is half a
	// picture, and a lead whose half grew straight back would read as a delete
	// that did nothing.
	apply(t, Op{Kind: OpRemove, ScreenID: half.ID})
	if got := ids(p); len(got) != 2 || got[0] != "s_a" || got[1] != "s_c" {
		t.Errorf("removing a half left %v", got)
	}
}

// TestALayoutChangeRetiresTheHalf: the parts follow the layout both ways, or a
// set keeps a slice of a composition that is no longer that wide.
func TestALayoutChangeRetiresTheHalf(t *testing.T) {
	p := project()
	for _, op := range []Op{
		{Kind: OpSetting, ScreenID: "s_b", Key: "layout", Value: "panorama"},
		{Kind: OpSetting, ScreenID: "s_b", Key: "layout", Value: "hero"},
	} {
		if err := ApplyTo(&p, op); err != nil {
			t.Fatal(err)
		}
	}
	if got := ids(p); len(got) != 3 {
		t.Errorf("after going back to one tile the set is %v", got)
	}
}

func ids(p model.Project) []string {
	out := make([]string, len(p.Screens()))
	for i, s := range p.Screens() {
		out[i] = s.ID
	}
	return out
}

// TestUnknownOpIsNotAnError: the browser may hold a slightly older document
// than the server, and a reorder that has already happened must not fail a
// request.
func TestUnknownOpIsNotAnError(t *testing.T) {
	p := project()
	for _, op := range []Op{
		{Kind: "nonsense"},
		{Kind: OpMove, ScreenID: "s_missing", Delta: 1},
		{Kind: OpRemove, ScreenID: "s_missing"},
		{Kind: OpSetting, ScreenID: "s_missing", Key: "tilt", Value: "3"},
	} {
		if err := ApplyTo(&p, op); err != nil {
			t.Errorf("%v should be a no-op, got %v", op, err)
		}
	}
}

// TestBadValueIsRejected: the parsing is shared, so a control posting nonsense
// is refused identically on both sides rather than writing a broken document.
func TestBadValueIsRejected(t *testing.T) {
	p := project()
	if err := ApplyTo(&p, Op{Kind: OpSetting, Key: "tilt", Value: "sideways"}); err == nil {
		t.Error("a tilt of \"sideways\" should not apply")
	}
	if err := ApplyTo(&p, Op{Kind: OpSetting, Key: "nope", Value: "x"}); err == nil {
		t.Error("an unknown setting key should not apply")
	}
}
