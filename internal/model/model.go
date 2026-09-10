// Package model is the vocabulary: what a project, a screen, a look and an
// export size are. It is a leaf — no database, no HTTP, no rendering — so the
// pages, the renderer, the store and the endpoints can all agree on one set of
// types without importing each other.
//
// The shapes come from the app this replaces (a TypeScript/canvas tool), with
// two additions that changed the flow: a project owns its screens and its
// languages, and every screen's copy is per-locale.
package model

import "strings"

// A Background is either a flat colour or a two-stop linear gradient. Kind
// decides which of the other fields mean anything, and it is stored rather
// than inferred so a half-edited gradient does not silently become a solid.
type Background struct {
	Kind  string `json:"kind"` // "solid" | "gradient"
	Color string `json:"color,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	// CSS-style angle: 180 runs top to bottom, 135 top-left to bottom-right.
	Angle float64 `json:"angle,omitempty"`
}

func Solid(color string) Background { return Background{Kind: "solid", Color: color} }

func Gradient(from, to string, angle float64) Background {
	return Background{Kind: "gradient", From: from, To: to, Angle: angle}
}

// Notch kinds. A device draws its cutout from geometry, like everything else
// about it — there is no bitmap of a Dynamic Island anywhere in this program.
const (
	NotchIsland = "island"
	NotchPunch  = "punch"
	NotchNone   = "none"
)

// A Device is a geometric description, not artwork: an aspect ratio, a bezel
// and a corner radius as fractions of the outer frame width, and which cutout
// to draw. Adding a phone is one row in presets.Devices.
type Device struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"` // iPhone | iPad | Android | Other
	// ScreenAspect is width/height of the screen area, not the outer frame.
	ScreenAspect float64 `json:"screen_aspect"`
	Bezel        float64 `json:"bezel"`
	Radius       float64 `json:"radius"`
	Notch        string  `json:"notch"`
}

// FrameAspect is the outer frame's width/height once the bezel is added on all
// four sides. The renderer fits boxes to this, never to ScreenAspect.
func (d Device) FrameAspect() float64 {
	screenW := 1 - 2*d.Bezel
	screenH := screenW / d.ScreenAspect
	return 1 / (screenH + 2*d.Bezel)
}

type FrameColor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Body  string `json:"body"`
	Edge  string `json:"edge"`
}

// Placement sources. A multi-device arrangement shows the neighbouring
// screens rather than repeating one image, so a strip reads as a strip.
const (
	SourceSelf = "self"
	SourceNext = "next"
	SourcePrev = "prev"
)

// A Placement puts one device frame in the layout's device slot. Dx/Dy are
// offsets from the slot centre as fractions of composition width / tile
// height; Scale multiplies the fitted width; Rotate is degrees clockwise.
type Placement struct {
	Source string  `json:"source"`
	Dx     float64 `json:"dx"`
	Dy     float64 `json:"dy"`
	Scale  float64 `json:"scale"`
	Rotate float64 `json:"rotate"`
}

// A Position (an "arrangement" in the UI) is how many frames appear and where.
// Placements are drawn back to front.
type Position struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	Placements []Placement `json:"placements"`
}

// TextBand positions the copy across the composition. Left/Width are fractions
// of the composition width; nil means "the tile minus PadX on both sides", which
// is what every single-tile layout wants.
type TextBand struct {
	Top    float64  `json:"top"`
	Height float64  `json:"height"`
	Left   *float64 `json:"left,omitempty"`
	Width  *float64 `json:"width,omitempty"`
}

// DeviceBand is the band the device is fitted into. Bottom may exceed 1 to
// bleed off-canvas. Width fixes the frame width as a fraction of the tile
// instead of fitting the band; Cx/Cy move the centre off the band centre.
type DeviceBand struct {
	Top    float64  `json:"top"`
	Bottom float64  `json:"bottom"`
	Width  *float64 `json:"width,omitempty"`
	Cx     *float64 `json:"cx,omitempty"`
	Cy     *float64 `json:"cy,omitempty"`
}

// A Layout is one composition. Vertical fractions are of the tile height,
// horizontal ones of the composition width (tile width × span), so a panorama
// can put its copy on the left tile and its device across the seam.
type Layout struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Span is how many store tiles the composition covers. A span-2 layout is
	// drawn once at double width and sliced into two PNGs on export.
	Span int `json:"span"`
	// Text is nil for a device-only layout.
	Text   *TextBand  `json:"text"`
	Device DeviceBand `json:"device"`
	PadX   float64    `json:"pad_x"`
}

type ExportSize struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Store string `json:"store"` // "App Store" | "Google Play"
	W     int    `json:"w"`
	H     int    `json:"h"`
	// Device is the frame that belongs in this slot. An iPad slot drawn with
	// an iPhone body is the wrong picture at the right pixel size, and nobody
	// picking "iPad 13-inch" is asking for that — so adding a target defaults
	// its frame from here rather than inheriting whatever the last one used.
	Device string `json:"device"`
}

// A Target is one store slot a project ships to: the pixel size the store
// wants, and the device frame drawn inside it.
//
// A project has a list of these for the same reason it has a list of
// languages. Shipping an app means producing a set per device family, every
// release, and the alternative — one project per size, with the copy written
// three times — is the work this exists to remove.
type Target struct {
	SizeID   string `json:"size_id"`
	DeviceID string `json:"device_id"`
}

const (
	AlignCenter = "center"
	AlignLeft   = "left"
)

// Settings is the whole look, resolved for one target: SizeID and DeviceID are
// the store slot currently being drawn, and a project ships to a list of them.
//
// Everything here except SizeID can be overridden on a single screen. One
// exported set must share one canvas size — a screen that disagreed would come
// out at a size the store rejects — so SizeID is not part of Overrides at all.
// Rendering another target is resolving that target into a copy of Settings,
// which is what Project.Use does.
type Settings struct {
	Background Background `json:"background"`
	// BackdropColor is the rounded card behind the device band. "" = none.
	BackdropColor string  `json:"backdrop_color"`
	DeviceID      string  `json:"device_id"`
	FrameColorID  string  `json:"frame_color_id"`
	PositionID    string  `json:"position_id"`
	Layout        string  `json:"layout"`
	Tilt          float64 `json:"tilt"`
	DeviceScale   float64 `json:"device_scale"`
	TextColor     string  `json:"text_color"`
	// AccentColor is what a **doubled** phrase is set in: the same type, a
	// different ink. Empty means it follows TextColor, so a project that never
	// uses the markup never has a second colour to keep in step.
	AccentColor string `json:"accent_color,omitempty"`
	TextAlign   string `json:"text_align"`
	// Highlights are the marker bands drawn behind *starred* headline words.
	// Spans cycle through the list, so a second starred phrase takes the
	// second colour.
	Highlights []string `json:"highlights"`
	FontID     string   `json:"font_id"`
	// Multipliers on the base headline / subtitle size.
	HeadlineScale float64 `json:"headline_scale"`
	SubheadScale  float64 `json:"subhead_scale"`
	// Headline letter-spacing as a fraction of the font size (em).
	HeadlineTracking float64 `json:"headline_tracking"`
	SizeID           string  `json:"size_id"`
}

// Overrides is a Settings with every field optional: nil means inherit. The
// pointers matter — writing a resolved value here would silently pin it, and
// the screen would stop following the global setting for the rest of its life.
type Overrides struct {
	Background       *Background `json:"background,omitempty"`
	BackdropColor    *string     `json:"backdrop_color,omitempty"`
	DeviceID         *string     `json:"device_id,omitempty"`
	FrameColorID     *string     `json:"frame_color_id,omitempty"`
	PositionID       *string     `json:"position_id,omitempty"`
	Layout           *string     `json:"layout,omitempty"`
	Tilt             *float64    `json:"tilt,omitempty"`
	DeviceScale      *float64    `json:"device_scale,omitempty"`
	TextColor        *string     `json:"text_color,omitempty"`
	AccentColor      *string     `json:"accent_color,omitempty"`
	TextAlign        *string     `json:"text_align,omitempty"`
	Highlights       *[]string   `json:"highlights,omitempty"`
	FontID           *string     `json:"font_id,omitempty"`
	HeadlineScale    *float64    `json:"headline_scale,omitempty"`
	SubheadScale     *float64    `json:"subhead_scale,omitempty"`
	HeadlineTracking *float64    `json:"headline_tracking,omitempty"`
}

// Apply returns settings with every set override laid on top. This is called
// in exactly one place — the top of the renderer — so the preview and the
// export can never disagree about what a screen inherits.
func (o Overrides) Apply(s Settings) Settings {
	if o.Background != nil {
		s.Background = *o.Background
	}
	if o.BackdropColor != nil {
		s.BackdropColor = *o.BackdropColor
	}
	if o.DeviceID != nil {
		s.DeviceID = *o.DeviceID
	}
	if o.FrameColorID != nil {
		s.FrameColorID = *o.FrameColorID
	}
	if o.PositionID != nil {
		s.PositionID = *o.PositionID
	}
	if o.Layout != nil {
		s.Layout = *o.Layout
	}
	if o.Tilt != nil {
		s.Tilt = *o.Tilt
	}
	if o.DeviceScale != nil {
		s.DeviceScale = *o.DeviceScale
	}
	if o.AccentColor != nil {
		s.AccentColor = *o.AccentColor
	}
	if o.TextColor != nil {
		s.TextColor = *o.TextColor
	}
	if o.TextAlign != nil {
		s.TextAlign = *o.TextAlign
	}
	if o.Highlights != nil {
		s.Highlights = *o.Highlights
	}
	if o.FontID != nil {
		s.FontID = *o.FontID
	}
	if o.HeadlineScale != nil {
		s.HeadlineScale = *o.HeadlineScale
	}
	if o.SubheadScale != nil {
		s.SubheadScale = *o.SubheadScale
	}
	if o.HeadlineTracking != nil {
		s.HeadlineTracking = *o.HeadlineTracking
	}
	return s
}

// Section names the group of controls a key belongs to, for the per-section
// reset in the tune panel.
var Sections = map[string][]string{
	"background": {"background", "backdrop_color"},
	"device":     {"device_id", "frame_color_id"},
	"layout":     {"layout", "position_id"},
	"type":       {"font_id", "headline_scale", "subhead_scale", "headline_tracking", "text_color", "text_align", "highlights"},
	"adjust":     {"tilt", "device_scale"},
}

// Clear removes the named override keys, returning the result. Used by the
// "reset this section" affordance and by the rhythm, which owns three keys and
// must not disturb the colours a template pinned.
func (o Overrides) Clear(keys ...string) Overrides {
	for _, key := range keys {
		switch key {
		case "background":
			o.Background = nil
		case "backdrop_color":
			o.BackdropColor = nil
		case "device_id":
			o.DeviceID = nil
		case "frame_color_id":
			o.FrameColorID = nil
		case "position_id":
			o.PositionID = nil
		case "layout":
			o.Layout = nil
		case "tilt":
			o.Tilt = nil
		case "device_scale":
			o.DeviceScale = nil
		case "text_color":
			o.TextColor = nil
		case "accent_color":
			o.AccentColor = nil
		case "text_align":
			o.TextAlign = nil
		case "highlights":
			o.Highlights = nil
		case "font_id":
			o.FontID = nil
		case "headline_scale":
			o.HeadlineScale = nil
		case "subhead_scale":
			o.SubheadScale = nil
		case "headline_tracking":
			o.HeadlineTracking = nil
		}
	}
	return o
}

// Copy is one screen's words in one language. Headline carries markup:
// *starred* words get a marker band behind them.
type Copy struct {
	Headline string `json:"headline"`
	Subhead  string `json:"subhead"`
}

func (c Copy) Empty() bool {
	return strings.TrimSpace(c.Headline) == "" && strings.TrimSpace(c.Subhead) == ""
}

// A Screen is one output image per language. It owns a source screenshot, its
// copy in every locale the project supports, and its overrides.
//
// One screen is one uploaded file. A panorama is drawn once across two tiles
// and uploaded as two files, so it is two screens here — not one screen that
// quietly produces two. That is what lets either half be moved, reordered or
// deleted like any other screenshot, including into an order that breaks the
// picture, which is a decision the author is allowed to make.
type Screen struct {
	ID string `json:"id"`
	// AssetID is the stored original screenshot; "" while the slot is empty.
	// It is the base language's capture, and every language draws it unless
	// Shots names another.
	AssetID string `json:"asset_id"`
	// Shots is a per-language screenshot, keyed by locale tag, for the case
	// AssetID cannot cover: the picture is of the app, and a Japanese listing
	// showing an English UI is not the app anyone is downloading.
	//
	// Absent means inherit, the way a nil override does. A project that ships
	// one set of captures never writes this, and a project that localises two
	// screens out of six writes two entries rather than a full second set.
	Shots map[string]string `json:"shots,omitempty"`
	// Copy is keyed by locale tag. A locale with no entry falls back to the
	// project's base locale, so adding a language never blanks the preview.
	Copy      map[string]Copy `json:"copy"`
	Overrides Overrides       `json:"overrides"`

	// Of names the screen that owns the composition this one draws a slice of,
	// and Part is which slice. Both are zero on a screen that owns its own
	// drawing — which is every screen except the trailing parts of a
	// multi-tile layout.
	//
	// The parts of one composition share a screenshot and a set of words:
	// there is one picture, cut into files. SyncParts copies the lead's down
	// onto them after every edit, so every read path — the renderer, the
	// readiness count, the preview tag — sees a complete screen and none of
	// them needs to know about the relationship.
	Of   string `json:"of,omitempty"`
	Part int    `json:"part,omitempty"`
}

// Follows reports whether this screen is a trailing part of another screen's
// composition, rather than one that owns its own.
func (s Screen) Follows() bool { return s.Of != "" }

// Text returns the copy for a locale, falling back to base. The fallback is
// what makes "add Japanese" a non-destructive act: the set still renders, in
// English, until the Japanese words exist.
func (s Screen) Text(locale, base string) Copy {
	if c, ok := s.Copy[locale]; ok && !c.Empty() {
		return c
	}
	return s.Copy[base]
}

// Asset is the screenshot this screen draws in one language: its own capture
// for that locale, or the base language's.
//
// Resolved here and nowhere else, for the reason overrides are resolved in one
// place — the exporter loads these bytes and [PreviewTag] hashes this id, and
// two walks that disagreed would pin a stale tile behind an immutable URL.
func (s Screen) Asset(locale string) string {
	if id := s.Shots[locale]; id != "" {
		return id
	}
	return s.AssetID
}

// Localised reports whether this screen has its own capture in a language,
// rather than drawing the base one. What the picker uses to say which of the
// two a tile is showing.
func (s Screen) Localised(locale string) bool { return s.Shots[locale] != "" }

// Filled is whether the slot has a picture at all, which is the base capture:
// a language-specific shot is a replacement for one, never the only one, so a
// screen with nothing base is an empty slot however many locales it names.
func (s Screen) Filled() bool { return s.AssetID != "" }

// A RhythmStep is which composition a tile takes. Applied as overrides by
// screen index, so it survives reordering with the screen it was applied to.
type RhythmStep struct {
	Layout     string `json:"layout"`
	PositionID string `json:"position_id"`
	TextAlign  string `json:"text_align,omitempty"` // "" = leave the screen's own
}

// A Rhythm is the strip's sequence of compositions, independent of the look:
// a panorama opener, a hero, an offset, a breather. A rhythm shorter than the
// set repeats from its start; no steps at all means every tile is uniform.
type Rhythm struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Description string       `json:"description"`
	Steps       []RhythmStep `json:"steps"`
}

// Step returns the overrides this rhythm pins on screen i, or nil for uniform.
func (r Rhythm) Step(i int) *RhythmStep {
	if len(r.Steps) == 0 {
		return nil
	}
	step := r.Steps[i%len(r.Steps)]
	return &step
}

// Sample is the copy an unfilled slot carries, so a template's whole look is
// visible before any screenshot exists.
type Sample struct {
	Headline string `json:"headline"`
	Subhead  string `json:"subhead"`
	// Shot is the screenshot this slot was designed against, kept with the
	// template as a file of its own and named relative to the template's
	// directory. "" when the slot was empty when the template was saved.
	//
	// A look is not separable from the pictures it was judged on — a headline
	// sits where it does because of what was behind it — so a template that
	// kept the words and threw the captures away came back as a layout nobody
	// could evaluate. The store copies these in on apply as ordinary assets,
	// which is what lets them be replaced one at a time afterwards.
	Shot string `json:"shot,omitempty"`
}

// A Template is a complete look: a settings preset plus optional per-screen
// variants that cycle by index. A template with variants is a *set* template —
// it has a fixed number of slots, laid out the moment it is chosen. One
// without variants is freeform: any number of screens.
//
// Unlike the app this replaces, templates are stored, not compiled in: the
// built-ins are seeded into the database on first run and can be edited,
// duplicated and added to.
type Template struct {
	// ID carries no JSON tag, for the same reason model.Project's does not:
	// the stored form embeds this next to the database envelope, which owns
	// "id". Two fields tagged "id" at the same depth is not an error in Go —
	// encoding/json silently drops both, and the document comes back with no
	// identifier at all.
	ID          string `json:"-"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Settings is applied over the defaults, so applying a template is a reset
	// rather than a patch. Export size and device are never part of it — those
	// stay the user's choice across a template change.
	Settings Overrides `json:"settings"`
	// Variants[i % len] is pinned on screen i.
	Variants []Overrides `json:"variants,omitempty"`
	// Rhythm the variants follow, for the picker. "" = the template's own.
	Rhythm string `json:"rhythm,omitempty"`
	// Locale is the language the samples are written in: the base language of
	// the project this was saved from, which is "en-US" unless it was changed.
	// Recorded rather than assumed, because a template saved from a Japanese
	// project carries Japanese words and the picker should say so.
	Locale  string   `json:"locale,omitempty"`
	Samples []Sample `json:"samples"`
	// Builtin templates ship with the app. They can be duplicated but the
	// originals are restored on every start, so a bad edit is one restart away
	// from fixed.
	Builtin bool `json:"builtin"`
}

// Slots is how many screens a set template lays out. 0 means freeform.
func (t Template) Slots() int { return len(t.Variants) }

// Variant is the overrides pinned on the screen at index i.
func (t Template) Variant(i int) Overrides {
	if len(t.Variants) == 0 {
		return Overrides{}
	}
	return t.Variants[i%len(t.Variants)]
}

func (t Template) Sample(i int) Sample {
	if len(t.Samples) == 0 {
		return Sample{}
	}
	return t.Samples[i%len(t.Samples)]
}

// A Locale is one language a project ships copy in. Tag is the App Store /
// Play locale code, which is what the export directories are named after.
type Locale struct {
	Tag    string `json:"tag"`   // "en-US", "ja", "pt-BR"
	Label  string `json:"label"` // "English (US)"
	Native string `json:"native,omitempty"`
	// RTL flips the text alignment of a left-aligned layout. Nothing else in
	// the renderer cares about direction.
	RTL bool `json:"rtl,omitempty"`
}

// Listing is the cosmetic store-page detail used by the review preview.
// Nothing here reaches an exported pixel.
type Listing struct {
	Name      string `json:"name"`
	Subtitle  string `json:"subtitle"`
	Developer string `json:"developer"`
	Category  string `json:"category"`
}

// Export formats. App Store Connect rejects images carrying an alpha channel
// and a PNG encoder always writes RGBA, so JPEG is the escape hatch rather
// than a quality choice.
const (
	FormatPNG  = "png"
	FormatJPEG = "jpeg"
)
