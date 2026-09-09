package render

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

func TestParseMarkup(t *testing.T) {
	got := ParseMarkup("The list that *feels* like a *notebook.*")
	want := []Word{
		{Text: "The", Span: -1}, {Text: "list", Span: -1}, {Text: "that", Span: -1},
		{Text: "feels", Span: 0},
		{Text: "like", Span: -1}, {Text: "a", Span: -1},
		{Text: "notebook.", Span: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d words, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseMarkupTagsDoubledPhrases(t *testing.T) {
	got := ParseMarkup("Built for **focus**, not for *busywork*")
	want := []Word{
		{Text: "Built", Span: -1}, {Text: "for", Span: -1},
		{Text: "focus", Span: -1, Accent: true},
		{Text: ",", Span: -1, Glue: true},
		{Text: "not", Span: -1}, {Text: "for", Span: -1},
		{Text: "busywork", Span: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d words, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The two markers share a character, so the scanner has to tell `**` from `*`
// before it counts spans — a split on "*" reads `**focus**` as two empty
// highlights and numbers everything after it wrongly.
func TestParseMarkupDoubledDoesNotConsumeSpans(t *testing.T) {
	got := ParseMarkup("**a** *b* *c*")
	var spans []int
	for _, w := range got {
		spans = append(spans, w.Span)
	}
	if len(spans) != 3 || spans[0] != -1 || spans[1] != 0 || spans[2] != 1 {
		t.Errorf("spans are %v, want [-1 0 1]", spans)
	}
	if !got[0].Accent {
		t.Error("the doubled phrase is not accented")
	}
}

func TestParseMarkupDropsUnmatchedDouble(t *testing.T) {
	got := ParseMarkup("Built for **focus")
	if len(got) != 3 || got[2].Accent {
		t.Fatalf("unmatched ** should not accent: %+v", got)
	}
}

func TestStripMarkupDropsBothMarkers(t *testing.T) {
	if got := StripMarkup("Built for **focus**, not for *busywork*"); got != "Built for focus, not for busywork" {
		t.Errorf("got %q", got)
	}
}

func TestParseMarkupDropsUnmatchedStar(t *testing.T) {
	// Mid-typing: the author has opened a span and not closed it. Highlighting
	// the rest of the headline on every keystroke would be worse than nothing.
	got := ParseMarkup("Works *offline")
	if len(got) != 2 || got[1].Span != -1 {
		t.Fatalf("unmatched star should not highlight: %v", got)
	}
}

func TestStripMarkup(t *testing.T) {
	if got := StripMarkup("Everything in *one place*"); got != "Everything in one place" {
		t.Errorf("got %q", got)
	}
}

func TestWrapBreaksOnWidth(t *testing.T) {
	tf := Face("inter", true, 40)
	words := ParseMarkup("one two three four five six seven eight nine ten")
	lines := wrap(tf, words, 200, 0)
	if len(lines) < 2 {
		t.Fatalf("expected several lines, got %d", len(lines))
	}
	for i, line := range lines {
		// A line may exceed the box only when it holds a single word, which is
		// deliberate: hyphenating a headline is worse than one wide line.
		if line.Width > 200 && len(line.Words) > 1 {
			t.Errorf("line %d is %.0f wide with %d words", i, line.Width, len(line.Words))
		}
	}
}

func TestAutoShrinkFitsTheGap(t *testing.T) {
	s := model.DefaultSettings()
	layout := presets.LayoutOf("text-top")
	const h = 2868
	avail := AvailableTextHeight(layout, h)

	long := model.Copy{Headline: "A headline long enough that it has to be shrunk to clear the device band below it", Subhead: "And a subtitle underneath it as well."}
	block := LayoutText(long, s, 1320*0.82, avail, h)
	if block.Height() > avail {
		t.Errorf("block is %.0f tall, gap is %.0f", block.Height(), avail)
	}
	// The shrink must have actually happened, or the assertion above is
	// passing for the wrong reason.
	if block.HeadSize >= h*0.04 {
		t.Errorf("headline did not shrink: %.1f", block.HeadSize)
	}
}

func TestFallbackCoversJapanese(t *testing.T) {
	tf := Face("inter", true, 64)
	// Inter has no kana. Without the fallback chain this advance is zero and a
	// Japanese headline renders as an empty line at full width.
	if w := tf.Width("使い方", 0); w < 100 {
		t.Errorf("Japanese measured %.1f wide — the CJK fallback is not loaded", w)
	}
}

func TestWrapsJapaneseWithoutSpaces(t *testing.T) {
	tf := Face("inter", true, 60)
	// One whitespace-delimited token, 24 characters wide. Before CJK tokenising
	// this came back as a single line 1400px wide inside a 600px box.
	words := ParseMarkup("買い物リストがひとつになって毎日の家事が楽になります")
	lines := wrap(tf, words, 600, 0)
	if len(lines) < 2 {
		t.Fatalf("Japanese did not break: %d line(s)", len(lines))
	}
	for i, line := range lines {
		if line.Width > 600 && len(line.Words) > 1 {
			t.Errorf("line %d is %.0f wide", i, line.Width)
		}
		// Kinsoku: a line may not open with a character that cannot start one.
		if first := []rune(line.Words[0].Text)[0]; strings.ContainsRune(noStart, first) {
			t.Errorf("line %d starts with %q", i, first)
		}
	}
}

func TestSpanFollowsTheResolvedLayout(t *testing.T) {
	s := model.DefaultSettings()
	panorama := "panorama"
	screen := model.Screen{Overrides: model.Overrides{Layout: &panorama}}
	if got := Span(screen, s); got != 2 {
		t.Errorf("panorama should span 2 tiles, got %d", got)
	}
	if got := Span(model.Screen{}, s); got != 1 {
		t.Errorf("default layout should span 1 tile, got %d", got)
	}
}

// TestVisual writes one PNG per look into $TMPDIR/appeditions-render. It asserts
// nothing about pixels — a canvas's correctness is not visible to an
// assertion, and claiming otherwise is how a wrong bezel ships. It exists so
// the drawing can be looked at after a change.
func TestVisual(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "appeditions-render")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	shot := Placeholder()
	sources := Sources{model.SourceSelf: shot, model.SourceNext: shot, model.SourcePrev: shot}

	cases := []struct {
		name     string
		copy     model.Copy
		mutate   func(*model.Settings)
		override model.Overrides
	}{
		{name: "classic", copy: model.Copy{Headline: "Everything in *one place*", Subhead: "Notes, tasks and files together."}},
		{
			// A break where the author asked for one, in a headline the width
			// would have broken somewhere else entirely.
			name: "forced-break",
			copy: model.Copy{
				Headline: `Plan the week\nin seconds`,
				Subhead:  "Drag, drop,\ndone.",
			},
			mutate: func(s *model.Settings) { s.Layout = "text-top" },
		},
		{
			// All three emphases, so the difference between them is a thing you
			// can see rather than a sentence in a panel: a band for *starred*,
			// a change of ink for **doubled**, and both for ***tripled***.
			name: "two-emphases",
			copy: model.Copy{
				Headline: "Built for ***focus***, not for *busywork*",
				Subhead:  "The one app that gets **out of the way**.",
			},
			mutate: func(s *model.Settings) {
				s.Layout = "text-top"
				s.TextColor = "#0f172a"
				s.AccentColor = "#2563eb"
				s.Highlights = []string{"#fde68a"}
			},
		},
		{
			// The subtitle is parsed for *stars* exactly as the headline is, and
			// draws its bands from the same list. It went years passing nil for
			// the colours, so a starred phrase down here drew nothing and there
			// was no way to tell that from "the feature is not for subtitles".
			name: "marked-subtitle",
			copy: model.Copy{
				Headline: "Everything in *one place*",
				Subhead:  "Notes, tasks and files — *together at last*.",
			},
			mutate: func(s *model.Settings) {
				s.Highlights = []string{"#bfdbfe", "#fde68a"}
				s.Layout = "text-top"
			},
		},
		{
			name: "notebook",
			copy: model.Copy{Headline: "The list that *feels* like a *notebook.*"},
			mutate: func(s *model.Settings) {
				s.Background = model.Solid("#f3efe4")
				s.BackdropColor = "#ebe5d6"
				s.FrameColorID = "black"
				s.Layout = "editorial"
				s.TextAlign = model.AlignLeft
				s.HeadlineScale = 1.5
				s.HeadlineTracking = -0.035
				s.TextColor = "#23231f"
				s.Highlights = []string{"#e9c7c2", "#cfdbc1"}
				s.DeviceScale = 0.86
			},
		},
		{
			name: "gradient-trio",
			copy: model.Copy{Headline: "Capture *anything*", Subhead: "Text, voice or a photo."},
			mutate: func(s *model.Settings) {
				s.Background = model.Gradient("#6366f1", "#a855f7", 135)
				s.TextColor = "#ffffff"
				s.Layout = "hero"
				s.PositionID = "trio"
				s.FontID = "space-grotesk"
			},
		},
		{
			name: "panorama",
			copy: model.Copy{Headline: "Your whole day, *one glance*", Subhead: "Calendar, tasks and notes."},
			mutate: func(s *model.Settings) {
				s.Layout = "panorama"
				s.PositionID = "lean"
				s.TextAlign = model.AlignLeft
				s.Background = model.Gradient("#e8f1ff", "#ffffff", 160)
				s.FontID = "dm-sans"
			},
		},
		{
			name: "japanese",
			copy: model.Copy{Headline: "買い物リストが*ひとつ*になって、毎日の家事がぐっと楽になります", Subhead: "メモも、タスクも、ファイルも。"},
			mutate: func(s *model.Settings) {
				s.Background = model.Solid("#111114")
				s.TextColor = "#ffffff"
				s.Layout = "hero"
				s.PositionID = "tilted"
				s.Highlights = []string{"#ffe27a"}
			},
		},
	}

	size := presets.Size("iphone-6-9")
	for _, tc := range cases {
		settings := model.DefaultSettings()
		if tc.mutate != nil {
			tc.mutate(&settings)
		}
		screen := model.Screen{Overrides: tc.override}
		// Half resolution: the geometry is identical and the file is a quarter
		// of the size to open.
		img := Scene(size.W/2, size.H/2, screen, settings, tc.copy, sources)

		f, err := os.Create(filepath.Join(dir, tc.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	t.Logf("wrote %d renders to %s", len(cases), dir)
}

// *** is the other two composed: a band and the second ink on the same phrase.
// It has to be scanned before ** or the phrase comes out accented with a stray
// star after it, which is the bug that made the two-marker scanner necessary in
// the first place.
func TestParseMarkupTripleIsBoth(t *testing.T) {
	got := ParseMarkup("Built for ***focus***")
	if len(got) != 3 {
		t.Fatalf("got %d words: %+v", len(got), got)
	}
	last := got[2]
	if last.Text != "focus" || last.Span != 0 || !last.Accent {
		t.Errorf("tripled phrase is %+v, want span 0 and accented", last)
	}
}

func TestParseMarkupTripleMixesWithTheOthers(t *testing.T) {
	got := ParseMarkup("***a*** **b** *c*")
	want := []Word{
		{Text: "a", Span: 0, Accent: true},
		{Text: "b", Span: -1, Accent: true},
		{Text: "c", Span: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d words: %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseMarkupDropsUnmatchedTriple(t *testing.T) {
	got := ParseMarkup("Built for ***focus")
	if len(got) != 3 || got[2].Accent || got[2].Span != -1 {
		t.Fatalf("unmatched *** should not emphasise: %+v", got)
	}
}

func TestStripMarkupDropsTheTriple(t *testing.T) {
	if got := StripMarkup("Built for ***focus***"); got != "Built for focus" {
		t.Errorf("got %q", got)
	}
}

// A forced break, in both spellings. The headline field is a single-line
// <input> where a real newline cannot be typed, so the escape has to work; the
// subtitle is a <textarea> where it can, so the character has to work too.
func TestParseMarkupForcesBreaks(t *testing.T) {
	for _, text := range []string{`Everything\nin one place`, "Everything\nin one place"} {
		got := ParseMarkup(text)
		if len(got) != 4 {
			t.Fatalf("%q: got %d words: %+v", text, len(got), got)
		}
		if !got[1].Break {
			t.Errorf("%q: the word after the break is not marked: %+v", text, got[1])
		}
		for i, w := range got {
			if i != 1 && w.Break {
				t.Errorf("%q: word %d should not break: %+v", text, i, w)
			}
		}
	}
}

func TestWrapHonoursAForcedBreak(t *testing.T) {
	tf := Face("inter", true, 40)
	// Wide enough that width alone would never break this.
	lines := wrap(tf, ParseMarkup(`one\ntwo`), 10000, 0)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(lines), lines)
	}
	if lines[0].Words[0].Text != "one" || lines[1].Words[0].Text != "two" {
		t.Errorf("broke in the wrong place: %+v", lines)
	}
}

// A break at the very end is an author mid-typing, not a blank line.
func TestParseMarkupIgnoresATrailingBreak(t *testing.T) {
	got := ParseMarkup(`one two\n`)
	if len(got) != 2 {
		t.Fatalf("got %d words: %+v", len(got), got)
	}
}

func TestStripMarkupTurnsABreakIntoASpace(t *testing.T) {
	if got := StripMarkup(`Everything\nin one place`); got != "Everything in one place" {
		t.Errorf("got %q", got)
	}
}

// Emphasis survives a break, and a break inside one does not end it.
func TestBreakInsideAnEmphasis(t *testing.T) {
	got := ParseMarkup(`*one\ntwo*`)
	if len(got) != 2 {
		t.Fatalf("got %d words: %+v", len(got), got)
	}
	if got[0].Span != 0 || got[1].Span != 0 {
		t.Errorf("the span did not survive the break: %+v", got)
	}
	if !got[1].Break {
		t.Errorf("the break was lost inside the span: %+v", got)
	}
}
