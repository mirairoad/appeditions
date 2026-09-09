// Package render draws a screen. It is the whole of what this program makes:
// the preview at 300px and the exported PNG at 1320×2868 are the same call to
// [Scene] with different sizes, so what is on screen is what lands on disk.
//
// There is deliberately no second rendering path. In the app this replaces,
// the preview was a canvas in the page and the export was the same canvas at
// full size; here both are Go, server-side, which removes the browser from the
// question entirely — the preview is a PNG the export code produced.
package render

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

// Font is one family the author can choose. Weights are separate files: a
// variable font's default instance is all x/image/font/sfnt can reach, so a
// variable Inter would draw every headline at 400 however loudly the renderer
// asked for 700.
type Font struct {
	ID    string
	Label string
	// Regular and Bold are paths inside the embedded font FS.
	Regular string
	Bold    string
}

var Fonts = []Font{
	{ID: "inter", Label: "Inter", Regular: "fonts/inter-400.ttf", Bold: "fonts/inter-700.ttf"},
	{ID: "dm-sans", Label: "DM Sans", Regular: "fonts/dm-sans-400.ttf", Bold: "fonts/dm-sans-700.ttf"},
	{ID: "poppins", Label: "Poppins", Regular: "fonts/poppins-400.ttf", Bold: "fonts/poppins-700.ttf"},
	{ID: "space-grotesk", Label: "Space Grotesk", Regular: "fonts/space-grotesk-400.ttf", Bold: "fonts/space-grotesk-700.ttf"},
	{ID: "playfair", Label: "Playfair Display", Regular: "fonts/playfair-400.ttf", Bold: "fonts/playfair-700.ttf"},
}

func FontOf(id string) Font {
	for _, f := range Fonts {
		if f.ID == id {
			return f
		}
	}
	return Fonts[0]
}

// Fallback fonts, tried in order for any rune the chosen family has no glyph
// for. Without this a Japanese headline renders as a row of nothing — not an
// error, not tofu, just gaps, because a missing glyph is index 0 and index 0
// in a Latin font is an empty box or an empty advance.
//
// The embedded Noto Sans JP covers Japanese, which is the language this was
// built for. Korean and Chinese come from the host: macOS ships both, and
// anything dropped in ~/.appeditions/fonts is tried before them.
// A fallback that ships at two weights is only used at the weight being asked
// for; one that does not (a system .ttc) is used at both, because a Korean
// headline in the regular weight is better than a Korean headline in nothing.
type fallbackSpec struct {
	path string
	// weight is 400, 700, or 0 for "use at any weight".
	weight int
}

var fallbackFiles = []fallbackSpec{
	{path: "fonts/noto-jp-400.ttf", weight: 400},
	{path: "fonts/noto-jp-700.ttf", weight: 700},
}

var systemFallbacks = []string{
	"/System/Library/Fonts/Hiragino Sans GB.ttc",
	"/System/Library/Fonts/AppleSDGothicNeo.ttc",
	"/System/Library/Fonts/Songti.ttc",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
}

// loaded is one parsed font file: the sfnt handle (which is what can answer
// "do you have this rune"), kept alongside its faces.
type loaded struct {
	font *sfnt.Font
	// buf is not safe for concurrent use, so glyph-index lookups take the
	// mutex. They are cheap and the alternative — a buffer per call — allocates
	// on every rune of every line of every render.
	mu  sync.Mutex
	buf sfnt.Buffer
}

func (l *loaded) has(r rune) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	idx, err := l.font.GlyphIndex(&l.buf, r)
	return err == nil && idx != 0
}

var (
	filesMu sync.Mutex
	files   = map[string]*loaded{}

	fallbackOnce sync.Once
	fallbacks    []fallback

	facesMu sync.Mutex
	faces   = map[faceKey]*guarded{}
)

// guarded is a face and the lock that owns it. font.Face implementations are
// stateful — opentype.Face keeps one glyph buffer and one rasterizer and
// reuses them — so two goroutines drawing through the same face write into the
// same mask. The symptom is not a wrong pixel but an index-out-of-range panic
// deep in image/draw, and it only appears once the export renders tiles in
// parallel.
//
// A mutex rather than a face per goroutine: sizing a face allocates a
// rasterizer, the auto-shrink loop asks for up to thirty sizes per screen, and
// glyph drawing is a small enough share of a render that serialising it costs
// nothing measurable.
type guarded struct {
	mu   sync.Mutex
	face font.Face
}

type faceKey struct {
	path string
	size float64
	// index selects a face inside a .ttc collection.
	index int
}

func loadFile(path string) (*loaded, error) {
	filesMu.Lock()
	defer filesMu.Unlock()
	if l, ok := files[path]; ok {
		return l, nil
	}
	data, err := fontFS.ReadFile(path)
	if err != nil {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		// A .ttc holds several faces; the first one is the regular weight in
		// every collection this cares about.
		coll, cerr := sfnt.ParseCollection(data)
		if cerr != nil || coll.NumFonts() == 0 {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		f, err = coll.Font(0)
		if err != nil {
			return nil, err
		}
	}
	l := &loaded{font: f}
	files[path] = l
	return l, nil
}

// fallback is one loaded fallback file and the weight it belongs to.
type fallback struct {
	spec fallbackSpec
	font *loaded
}

func loadFallbacks() {
	fallbackOnce.Do(func() {
		specs := append([]fallbackSpec{}, fallbackFiles...)
		// A user font directory comes before the system's: someone who dropped
		// a font in there did it to get that font, not to be outvoted.
		if home, err := os.UserHomeDir(); err == nil {
			matches, _ := filepath.Glob(filepath.Join(home, ".appeditions", "fonts", "*"))
			for _, m := range matches {
				switch filepath.Ext(m) {
				case ".ttf", ".ttc", ".otf":
					specs = append(specs, fallbackSpec{path: m})
				}
			}
		}
		for _, p := range systemFallbacks {
			specs = append(specs, fallbackSpec{path: p})
		}
		for _, spec := range specs {
			if l, err := loadFile(spec.path); err == nil {
				fallbacks = append(fallbacks, fallback{spec: spec, font: l})
			}
		}
	})
}

func faceFor(l *loaded, path string, size float64) (*guarded, error) {
	facesMu.Lock()
	defer facesMu.Unlock()
	key := faceKey{path: path, size: size}
	if f, ok := faces[key]; ok {
		return f, nil
	}
	// Hinting is off: at 2868px tall a headline is 115px and hinting only
	// distorts the outline; at 300px the preview is a thumbnail either way.
	f, err := opentype.NewFace(l.font, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, err
	}
	g := &guarded{face: f}
	faces[key] = g
	return g, nil
}

// Typeface is a family at one weight and size, with the fallback chain behind
// it. Every measurement and every glyph goes through it, so the line breaker
// and the drawing agree about widths down to the last rune.
type Typeface struct {
	size     float64
	primary  *guarded
	prim     *loaded
	fallback []*guarded
	fbFonts  []*loaded
}

// Face resolves a family id and weight at a pixel size. It never fails: an
// unloadable font would mean an editor that cannot draw, and the first
// embedded family is always there.
func Face(fontID string, bold bool, size float64) *Typeface {
	f := FontOf(fontID)
	path := f.Regular
	if bold {
		path = f.Bold
	}
	l, err := loadFile(path)
	if err != nil {
		l, err = loadFile(Fonts[0].Regular)
		if err != nil {
			panic("render: no usable font: " + err.Error()) // embedded, at build time
		}
	}
	face, err := faceFor(l, path, size)
	if err != nil {
		panic("render: cannot size font: " + err.Error())
	}

	want := 400
	if bold {
		want = 700
	}

	loadFallbacks()
	t := &Typeface{size: size, primary: face, prim: l}
	for _, fb := range fallbacks {
		if fb.spec.weight != 0 && fb.spec.weight != want {
			continue
		}
		ff, err := faceFor(fb.font, fb.spec.path, size)
		if err != nil {
			continue
		}
		t.fallback = append(t.fallback, ff)
		t.fbFonts = append(t.fbFonts, fb.font)
	}
	return t
}

// rune picks the first face in the chain that actually has the glyph.
func (t *Typeface) rune(r rune) *guarded {
	if t.prim.has(r) {
		return t.primary
	}
	for i, fb := range t.fbFonts {
		if fb.has(r) {
			return t.fallback[i]
		}
	}
	return t.primary
}

// Advance is the pen movement for one rune, in pixels.
func (t *Typeface) Advance(r rune) float64 {
	g := t.rune(r)
	g.mu.Lock()
	defer g.mu.Unlock()
	adv, ok := g.face.GlyphAdvance(r)
	if !ok {
		return 0
	}
	return f2f(adv)
}

// Width measures a string with the given tracking (letter-spacing as a
// fraction of the font size). Tracking is added after every rune, including
// the last, which is what a canvas does — matching it keeps the ported line
// breaker's output identical to the original's.
func (t *Typeface) Width(s string, tracking float64) float64 {
	w := 0.0
	extra := t.size * tracking
	for _, r := range s {
		w += t.Advance(r) + extra
	}
	return w
}

// Ascent is the distance from the top of the em box to the baseline. The
// layout code positions text by its top edge, like the canvas API it came
// from, so every draw adds this.
func (t *Typeface) Ascent() float64 {
	t.primary.mu.Lock()
	defer t.primary.mu.Unlock()
	return f2f(t.primary.face.Metrics().Ascent)
}

func f2f(v fixed.Int26_6) float64 { return float64(v) / 64 }
func f2i(v float64) fixed.Int26_6 { return fixed.Int26_6(v * 64) }
