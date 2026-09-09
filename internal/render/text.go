package render

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/fogleman/gg"
	"github.com/mirairoad/appeditions/internal/model"
)

// The text engine: markup, line breaking, auto-shrink and drawing. It is the
// one part of the renderer with real logic in it, and everything up to
// [LayoutText] is pure measurement — no image, no context, testable on its own.

// A Word carries the index of the *span* it belongs to, or -1 for plain text.
//
// Glue means "no space before this one". Japanese and Chinese are written
// without spaces, so a headline in either is one whitespace-delimited token and
// would never break: the line breaker would hand back a single line four times
// the width of the tile and the auto-shrink would grind it down to unreadable
// type. Splitting CJK into per-character tokens and gluing them back together
// gives the breaker somewhere to break.
type Word struct {
	Text string
	Span int
	Glue bool
	// Break forces a line to end before this word, whatever the width says.
	Break bool
	// Accent marks a **phrase**: the same words in the second text colour,
	// with no band behind them. A marker pen and a change of ink are different
	// emphases — one shouts, the other picks out a product name — and a
	// headline that wants the quieter one had to use the loud one.
	Accent bool
}

// ParseMarkup turns `The list that *feels* like a *notebook.*` into words
// tagged with their highlight span, and `Built for **focus**` into words
// tagged for the accent colour.
//
// Three markers, because they are three emphases:
//
//	*starred*      a marker band behind the phrase
//	**doubled**    the phrase in the second text colour
//	***tripled***  both — a band, with the phrase in the second colour on it
//
// The third is the other two composed rather than a third mechanism, which is
// why it needs no colour of its own: the band comes from the marker colours
// and the ink from the second text colour, exactly as they do alone.
//
// An unmatched marker is literal-dropped rather than applied to the rest of
// the line: the author is mid-typing, and re-emphasising the whole headline on
// every keystroke is worse than doing nothing.
func ParseMarkup(text string) []Word {
	var words []Word
	spaced := true
	for _, seg := range scanMarkup(text) {
		// `*one* place` has a space across the marker and `ひとつ*に*` does
		// not. Scanning threw that away, so it is recovered from the edges of
		// the two segments — otherwise every emphasis would introduce a space
		// the author never typed.
		glued := len(words) > 0 && !spaced && !startsWithSpace(seg.text)
		before := len(words)
		for j, t := range strings.Fields(seg.text) {
			words = appendTokens(words, t, seg.span, seg.accent, j == 0 && glued)
		}
		// A break belongs to the first word that follows it. Nothing after the
		// last one is not a blank line at the end of the headline — it is an
		// author who has just typed the escape and not the next word yet.
		if seg.brk && len(words) > before {
			words[before].Break = true
			words[before].Glue = false
		}
		if trimmed := strings.TrimRight(seg.text, " \t\n"); trimmed != seg.text || seg.text == "" {
			spaced = true
		} else {
			spaced = false
		}
	}
	return words
}

// A segment is a run of text under one emphasis.
type segment struct {
	text   string
	span   int
	accent bool
	// brk marks a segment that starts a new line. It is on the segment rather
	// than being a word of its own, so an empty one — `a\n\nb`, or a break at
	// the very start — carries the flag forward instead of drawing nothing on
	// a line of its own.
	brk bool
}

// scanMarkup walks the string rather than splitting it, because `**` and `*`
// cannot be told apart by a split: `strings.Split(s, "*")` turns `**focus**`
// into four parts and two empty strings, and the odd/even rule that used to
// number the spans reads those as emphasis.
//
// A marker only opens when its partner exists later in the string. That is
// what drops the half-typed one, and it is checked here rather than by
// unwinding afterwards.
func scanMarkup(text string) []segment {
	var segs []segment
	var buf strings.Builder
	span, spans, accent := -1, 0, false
	brk := false

	flush := func() {
		if buf.Len() > 0 {
			segs = append(segs, segment{buf.String(), span, accent, brk})
			buf.Reset()
			brk = false
		}
	}
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "***"):
			// Both at once: the band and the second ink. Longest marker first,
			// or `***` is read as `**` followed by a stray `*` and the phrase
			// comes out accented with an unmatched star after it.
			open := span < 0 && !accent
			if open && !strings.Contains(text[i+3:], "***") {
				i += 3
				continue
			}
			flush()
			if open {
				span, spans = spans, spans+1
				accent = true
			} else {
				span, accent = -1, false
			}
			i += 3
		case strings.HasPrefix(text[i:], "**"):
			if !accent && !strings.Contains(text[i+2:], "**") {
				i += 2 // unmatched: neither a marker nor ink on the page
				continue
			}
			flush()
			accent = !accent
			i += 2
		case text[i] == '*':
			if span < 0 && !strings.Contains(text[i+1:], "*") {
				i++
				continue
			}
			flush()
			if span < 0 {
				span, spans = spans, spans+1
			} else {
				span = -1
			}
			i++
		case strings.HasPrefix(text[i:], `\n`):
			// The escape, because the headline field is a single-line <input>
			// and there is no way to type a real newline into one. The
			// subtitle is a <textarea> where there is, so both spellings mean
			// the same thing — an author who found one should not discover
			// that the other silently prints a backslash.
			flush()
			brk = true
			i += 2
		case text[i] == '\n':
			flush()
			brk = true
			i++
		default:
			buf.WriteByte(text[i])
			i++
		}
	}
	flush()
	return segs
}

func startsWithSpace(s string) bool {
	return s != "" && strings.ContainsRune(" \t\n", rune(s[0]))
}

// isCJK reports whether a rune is written without word spaces: kana, kanji and
// hangul, plus the fullwidth punctuation that goes with them.
func isCJK(r rune) bool {
	switch {
	case r >= 0x3040 && r <= 0x30ff, // hiragana, katakana
		r >= 0x3400 && r <= 0x4dbf, // CJK extension A
		r >= 0x4e00 && r <= 0x9fff, // CJK unified
		r >= 0xac00 && r <= 0xd7af, // hangul syllables
		r >= 0xf900 && r <= 0xfaff, // compatibility ideographs
		r >= 0xff00 && r <= 0xff9f: // fullwidth forms and halfwidth kana
		return true
	}
	return false
}

// noStart is the minimum of the Japanese line-breaking rules (kinsoku): these
// may not begin a line. They are glued to the character before them instead,
// which is what a typesetter would do and what every browser does.
const noStart = "、。，．・：；？！ゝゞーぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮ）］｝」』〉》”’"

// noEnd may not end a line, so the character after them is pulled along.
const noEnd = "（［｛「『〈《“‘"

// appendTokens splits one whitespace-delimited token into breakable pieces. A
// run of Latin stays one piece; CJK becomes one piece per character.
func appendTokens(words []Word, text string, span int, accent bool, glue bool) []Word {
	var latin strings.Builder
	flush := func() {
		if latin.Len() == 0 {
			return
		}
		words = append(words, Word{Text: latin.String(), Span: span, Accent: accent, Glue: glue})
		latin.Reset()
		glue = true
	}
	pullNext := false
	for _, r := range text {
		if !isCJK(r) && !strings.ContainsRune(noStart, r) {
			latin.WriteRune(r)
			continue
		}
		// A character that may not start a line, or one following a character
		// that may not end one, joins the piece before it rather than becoming
		// a break opportunity of its own.
		if latin.Len() == 0 && len(words) > 0 && (pullNext || strings.ContainsRune(noStart, r)) &&
			words[len(words)-1].Span == span && words[len(words)-1].Accent == accent {
			words[len(words)-1].Text += string(r)
			pullNext = strings.ContainsRune(noEnd, r)
			continue
		}
		flush()
		words = append(words, Word{Text: string(r), Span: span, Accent: accent, Glue: glue})
		glue = true
		pullNext = strings.ContainsRune(noEnd, r)
	}
	flush()
	return words
}

// StripMarkup is the headline as plain words. Export filenames are built from
// it, so a file is never called `01-everything-in-one-place.png` with stars in
// the middle of it.
func StripMarkup(text string) string {
	var b strings.Builder
	for i, w := range ParseMarkup(text) {
		// A forced break is a word boundary like any other here: the export
		// filename is one line, and `01-everything-innone-place.png` is what
		// dropping it would produce.
		if i > 0 && (!w.Glue || w.Break) {
			b.WriteByte(' ')
		}
		b.WriteString(w.Text)
	}
	return b.String()
}

// A Line is a measured row of words: the widths are kept because the drawing
// pass needs each word's own advance to place the highlight bands.
type Line struct {
	Words  []Word
	Widths []float64
	Width  float64
}

// wrap breaks words into lines no wider than maxWidth, and wherever the author
// asked for a break. A single word longer than the box is left to overflow
// rather than broken: hyphenating a headline is worse than one wide line, and
// the auto-shrink below will usually fix it.
func wrap(tf *Typeface, words []Word, maxWidth, tracking float64) []Line {
	if len(words) == 0 {
		return nil
	}
	space := tf.Width(" ", tracking)
	var lines []Line
	line := Line{}
	for _, word := range words {
		ww := tf.Width(word.Text, tracking)
		next := ww
		if len(line.Words) > 0 {
			next = line.Width + gapBefore(space, word) + ww
		}
		if len(line.Words) > 0 && (word.Break || next > maxWidth) {
			lines = append(lines, line)
			line = Line{Words: []Word{word}, Widths: []float64{ww}, Width: ww}
			continue
		}
		line.Words = append(line.Words, word)
		line.Widths = append(line.Widths, ww)
		line.Width = next
	}
	return append(lines, line)
}

// gapBefore is the advance between two words on a line: a space, or nothing
// at all where the author wrote none.
func gapBefore(space float64, w Word) float64 {
	if w.Glue {
		return 0
	}
	return space
}

// Line heights, as multiples of the font size.
const (
	HeadLineHeight = 1.14
	SubLineHeight  = 1.4
)

// AvailableTextHeight is how tall the block may grow before it would collide
// with the device. This is the gap down to the device band, not the nominal
// band height: measured against the band, turning the size slider up would only
// trigger the auto-shrink, and the control would feel broken.
func AvailableTextHeight(layout model.Layout, h float64) float64 {
	if layout.Text == nil {
		return 0
	}
	var limit float64
	if layout.Text.Top > layout.Device.Top {
		// Copy below the device: the floor of the tile is what bounds it.
		limit = 1 - layout.Text.Top - 0.03
	} else {
		limit = layout.Device.Top - layout.Text.Top - 0.02
	}
	return math.Max(layout.Text.Height, limit) * h
}

// A TextLayout is a block that has been measured and fitted.
type TextLayout struct {
	HeadSize  float64
	SubSize   float64
	HeadLines []Line
	SubLines  []Line
	Gap       float64
	head      *Typeface
	sub       *Typeface
}

// Height is the whole block: headline, gap, subtitle.
func (t TextLayout) Height() float64 {
	h := float64(len(t.HeadLines)) * t.HeadSize * HeadLineHeight
	if len(t.SubLines) > 0 {
		h += t.Gap + float64(len(t.SubLines))*t.SubSize*SubLineHeight
	}
	return h
}

// LayoutText measures the copy and shrinks it until it fits, because
// overflowing into the device is worse than smaller type. h is the tile
// height: every size in here is a fraction of it, so a preview and a
// full-resolution export lay the text out identically.
func LayoutText(copy model.Copy, s model.Settings, maxWidth, maxHeight, h float64) TextLayout {
	headSize := h * 0.04 * s.HeadlineScale
	subSize := h * 0.0205 * s.SubheadScale
	gap := h * 0.018
	headWords := ParseMarkup(copy.Headline)
	subWords := ParseMarkup(copy.Subhead)

	var last TextLayout
	for range 30 {
		head := Face(s.FontID, true, headSize)
		sub := Face(s.FontID, false, subSize)
		last = TextLayout{
			HeadSize:  headSize,
			SubSize:   subSize,
			HeadLines: wrap(head, headWords, maxWidth, s.HeadlineTracking),
			SubLines:  wrap(sub, subWords, maxWidth, 0),
			Gap:       gap,
			head:      head,
			sub:       sub,
		}
		// The floor stops the loop turning a stubborn headline into unreadable
		// type; past it, overflow is the honest outcome.
		if last.Height() <= maxHeight || headSize < h*0.014 {
			return last
		}
		headSize *= 0.94
		subSize *= 0.94
	}
	return last
}

// drawString paints one string with the pen starting at the left edge of x and
// the top of the em box at yTop, applying tracking after every rune. Runes are
// drawn one at a time because the face may change between them — a headline
// mixing English and Japanese is two fonts on one line.
func drawString(dst *image.RGBA, tf *Typeface, x, yTop float64, s string, col color.Color, tracking, alpha float64) {
	src := image.NewUniform(fadedColor(col, alpha))
	extra := tf.size * tracking
	pen := x
	baseline := yTop + tf.Ascent()
	for _, r := range s {
		g := tf.rune(r)
		g.mu.Lock()
		d := font.Drawer{
			Dst:  dst,
			Src:  src,
			Face: g.face,
			Dot:  fixed.Point26_6{X: f2i(pen), Y: f2i(baseline)},
		}
		d.DrawString(string(r))
		adv, ok := g.face.GlyphAdvance(r)
		g.mu.Unlock()
		if !ok {
			continue
		}
		pen += f2f(adv) + extra
	}
}

// fadedColor premultiplies an alpha onto a colour. The subtitle is drawn at
// 72% so it reads as secondary without needing a second colour control.
func fadedColor(col color.Color, alpha float64) color.Color {
	r, g, b, a := col.RGBA()
	return color.RGBA64{
		R: uint16(float64(r) * alpha),
		G: uint16(float64(g) * alpha),
		B: uint16(float64(b) * alpha),
		A: uint16(float64(a) * alpha),
	}
}

// drawLine paints one line, marker bands first. A run of same-span words gets
// one continuous band, so a highlighted phrase reads as a single stroke of a
// marker rather than a row of boxes.
func drawLine(ctx *gg.Context, dst *image.RGBA, tf *Typeface, line Line, x0, y, size float64, col, accent color.Color, highlights []string, tracking, alpha float64) {
	space := tf.Width(" ", tracking)
	xs := make([]float64, len(line.Words))
	x := x0
	for i, word := range line.Words {
		if i > 0 {
			x += gapBefore(space, word)
		}
		xs[i] = x
		x += line.Widths[i]
	}

	if len(highlights) > 0 {
		// The band, as fractions of the type size so it scales with it.
		//
		// It used to hug the glyphs — 0.07 of the size on each side and a box
		// only as tall as the line — which read as a background fill rather
		// than as a marker pen. A highlighter is wider than the word and
		// overshoots it above and below; these are the numbers that look like
		// one. The text size is untouched: only the box around it grew.
		const (
			padX   = 0.16 // left and right of the phrase
			top    = 0.02 // above the line box
			height = 1.22 // and how far down it runs
			radius = 0.12
		)
		pad := size * padX
		for i := 0; i < len(line.Words); {
			span := line.Words[i].Span
			j := i
			for j+1 < len(line.Words) && line.Words[j+1].Span == span {
				j++
			}
			if span >= 0 {
				left := xs[i] - pad
				right := xs[j] + line.Widths[j] + pad
				ctx.Push()
				setHex(ctx, highlights[span%len(highlights)])
				ctx.DrawRoundedRectangle(left, y+size*top, right-left, size*height, size*radius)
				ctx.Fill()
				ctx.Pop()
			}
			i = j + 1
		}
	}

	for i, word := range line.Words {
		ink := col
		if word.Accent && accent != nil {
			ink = accent
		}
		drawString(dst, tf, xs[i], y, word.Text, ink, tracking, alpha)
	}
}

// DrawTextBlock draws the copy into the layout's text band. W is the whole
// composition width (tile width × span); tileW is one tile, which is what the
// default padding is a fraction of.
func DrawTextBlock(ctx *gg.Context, dst *image.RGBA, W, tileW, h float64, layout model.Layout, copy model.Copy, s model.Settings) {
	if layout.Text == nil || (copy.Headline == "" && copy.Subhead == "") {
		return
	}

	boxLeft := tileW * layout.PadX
	if layout.Text.Left != nil {
		boxLeft = W * *layout.Text.Left
	}
	maxWidth := tileW * (1 - layout.PadX*2)
	if layout.Text.Width != nil {
		maxWidth = W * *layout.Text.Width
	}

	bandTop := layout.Text.Top * h
	bandHeight := layout.Text.Height * h
	block := LayoutText(copy, s, maxWidth, AvailableTextHeight(layout, h), h)

	y := bandTop + (bandHeight-block.Height())/2
	startX := func(lineWidth float64) float64 {
		if s.TextAlign == model.AlignLeft {
			return boxLeft
		}
		return boxLeft + (maxWidth-lineWidth)/2
	}

	col := parseHex(s.TextColor)
	// nil when no second ink is set, which drawLine reads as "use the first".
	// An empty AccentColor must not become black: a project that never uses
	// the markup would then have every **doubled** phrase it later typed come
	// out invisible on a dark background.
	var accent color.Color
	if s.AccentColor != "" {
		accent = parseHex(s.AccentColor)
	}
	for _, line := range block.HeadLines {
		drawLine(ctx, dst, block.head, line, startX(line.Width), y, block.HeadSize, col, accent, s.Highlights, s.HeadlineTracking, 1)
		y += block.HeadSize * HeadLineHeight
	}
	if len(block.SubLines) > 0 {
		y += block.Gap
		for _, line := range block.SubLines {
			// The subtitle takes the marker colours too. It is parsed for
			// *stars* exactly as the headline is — it always was — and the
			// only reason a starred phrase down here drew nothing was that
			// this call passed nil.
			drawLine(ctx, dst, block.sub, line, startX(line.Width), y, block.SubSize, col, accent, s.Highlights, 0, 0.72)
			y += block.SubSize * SubLineHeight
		}
	}
}
