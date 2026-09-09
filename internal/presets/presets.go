// Package presets is the tables: devices, frame colours, backgrounds, fonts,
// layouts, arrangements, rhythms and export sizes. Everything here is data,
// and every lookup falls back to the first row rather than failing, because a
// typo in a stored id must degrade to something drawable rather than take the
// editor down.
//
// It is a leaf, like model — the renderer, the endpoints and the pages all
// read these tables.
package presets

import "github.com/mirairoad/appeditions/internal/model"

// Devices. Screen aspects are the real panel ratios; bezel and radius are
// fractions of the outer frame width, tuned so each family reads as itself —
// Pixels rounder, Samsungs squarer and thinner.
var Devices = []model.Device{
	{ID: "iphone-17-pro", Label: "iPhone 17 Pro", Group: "iPhone", ScreenAspect: 1320.0 / 2868, Bezel: 0.030, Radius: 0.150, Notch: model.NotchIsland},
	{ID: "iphone-17", Label: "iPhone 17", Group: "iPhone", ScreenAspect: 1206.0 / 2622, Bezel: 0.032, Radius: 0.150, Notch: model.NotchIsland},
	{ID: "iphone-se", Label: "iPhone SE", Group: "iPhone", ScreenAspect: 750.0 / 1334, Bezel: 0.045, Radius: 0.095, Notch: model.NotchNone},
	{ID: "ipad-13", Label: `iPad Pro 13"`, Group: "iPad", ScreenAspect: 2064.0 / 2752, Bezel: 0.026, Radius: 0.055, Notch: model.NotchNone},

	{ID: "pixel-9-pro", Label: "Pixel 9 Pro", Group: "Android", ScreenAspect: 1280.0 / 2856, Bezel: 0.026, Radius: 0.085, Notch: model.NotchPunch},
	{ID: "pixel-9-pro-xl", Label: "Pixel 9 Pro XL", Group: "Android", ScreenAspect: 1344.0 / 2992, Bezel: 0.025, Radius: 0.085, Notch: model.NotchPunch},
	{ID: "pixel-8a", Label: "Pixel 8a", Group: "Android", ScreenAspect: 1080.0 / 2400, Bezel: 0.040, Radius: 0.075, Notch: model.NotchPunch},
	{ID: "galaxy-s25-ultra", Label: "Galaxy S25 Ultra", Group: "Android", ScreenAspect: 1440.0 / 3120, Bezel: 0.022, Radius: 0.045, Notch: model.NotchPunch},
	{ID: "galaxy-s25", Label: "Galaxy S25", Group: "Android", ScreenAspect: 1080.0 / 2340, Bezel: 0.026, Radius: 0.090, Notch: model.NotchPunch},
	{ID: "galaxy-a56", Label: "Galaxy A56", Group: "Android", ScreenAspect: 1080.0 / 2340, Bezel: 0.038, Radius: 0.080, Notch: model.NotchPunch},
	{ID: "android-tablet", Label: "Android tablet", Group: "Android", ScreenAspect: 1600.0 / 2560, Bezel: 0.030, Radius: 0.050, Notch: model.NotchNone},

	// A watch is mostly bezel and mostly corner radius, which is why it needs
	// its own row rather than a scaled phone: at 0.09 bezel and 0.30 radius the
	// same drawing code produces a watch body.
	{ID: "watch-ultra", Label: "Apple Watch Ultra", Group: "Watch", ScreenAspect: 410.0 / 502, Bezel: 0.090, Radius: 0.300, Notch: model.NotchNone},

	{ID: "none", Label: "No frame", Group: "Other", ScreenAspect: 1320.0 / 2868, Bezel: 0, Radius: 0.045, Notch: model.NotchNone},
}

var DeviceGroups = []string{"iPhone", "iPad", "Watch", "Android", "Other"}

var FrameColors = []model.FrameColor{
	{ID: "deep-blue", Label: "Deep Blue", Body: "#28344a", Edge: "#5b6b88"},
	{ID: "black", Label: "Black", Body: "#1c1c1e", Edge: "#4a4a4e"},
	{ID: "silver", Label: "Silver", Body: "#c9ccd2", Edge: "#f2f3f5"},
	{ID: "gold", Label: "Desert Gold", Body: "#bda182", Edge: "#e8d6be"},
	{ID: "porcelain", Label: "Porcelain", Body: "#e8e4dc", Edge: "#fbf9f5"},
	{ID: "mint", Label: "Mint", Body: "#9fb8a8", Edge: "#cfe0d5"},
}

// Background swatches offered by the picker. Any colour is allowed; these are
// the ones that are one click away.
var SolidBackgrounds = []string{
	"#eaf2ff", "#fdeee4", "#e6f6ee", "#eeeaf9",
	"#fde9ef", "#fdf3d8", "#e8f4f6", "#f1f1f3",
	"#1e6ff5", "#f0502d", "#12885a", "#1f2937",
	"#7c3aed", "#db2777", "#0d9488", "#111114",
}

var GradientBackgrounds = []model.Background{
	model.Gradient("#6366f1", "#a855f7", 135),
	model.Gradient("#0ea5e9", "#22d3ee", 135),
	model.Gradient("#f97316", "#ec4899", 135),
	model.Gradient("#10b981", "#84cc16", 135),
	model.Gradient("#1e293b", "#0f172a", 160),
	model.Gradient("#fda4af", "#fef3c7", 150),
	model.Gradient("#4f46e5", "#0f172a", 200),
	model.Gradient("#e0f2fe", "#ede9fe", 135),
}

// Layouts. Where the text band and the device band sit, as fractions of the
// tile height; horizontal values are fractions of the composition width.
var Layouts = []model.Layout{
	{ID: "text-top", Label: "Text above", Span: 1, Text: &model.TextBand{Top: 0.065, Height: 0.165}, Device: model.DeviceBand{Top: 0.27, Bottom: 0.95}, PadX: 0.09},
	{ID: "text-bottom", Label: "Text below", Span: 1, Text: &model.TextBand{Top: 0.76, Height: 0.165}, Device: model.DeviceBand{Top: 0.06, Bottom: 0.72}, PadX: 0.09},
	{ID: "bleed", Label: "Bleed off edge", Span: 1, Text: &model.TextBand{Top: 0.07, Height: 0.175}, Device: model.DeviceBand{Top: 0.29, Bottom: 1.14}, PadX: 0.07},
	// Room for a three-line display headline; the device sits low and runs off
	// the bottom.
	{ID: "editorial", Label: "Tall text", Span: 1, Text: &model.TextBand{Top: 0.07, Height: 0.25}, Device: model.DeviceBand{Top: 0.41, Bottom: 1.12}, PadX: 0.085},
	// A large device, 95% of the tile wide, running off the bottom.
	{ID: "hero", Label: "Hero", Span: 1, Text: &model.TextBand{Top: 0.06, Height: 0.22}, Device: model.DeviceBand{Top: 0.3, Bottom: 1.3, Width: f(0.95), Cy: f(0.75)}, PadX: 0.09},
	// Room for two devices side by side (use the Duo arrangements).
	{ID: "duo", Label: "Duo", Span: 1, Text: &model.TextBand{Top: 0.06, Height: 0.22}, Device: model.DeviceBand{Top: 0.3, Bottom: 1.25, Width: f(0.7), Cy: f(0.74)}, PadX: 0.09},
	// Two store tiles: copy on the left tile, one big device across the seam.
	// The device leans away from the copy (top to the right) so its top-left
	// corner clears the text box.
	{ID: "panorama", Label: "Panorama", Span: 2, Text: &model.TextBand{Top: 0.07, Height: 0.3, Left: f(0.045), Width: f(0.36)}, Device: model.DeviceBand{Top: 0.3, Bottom: 1.3, Width: f(1.05), Cx: f(0.6), Cy: f(0.74)}, PadX: 0.09},
	// Two tiles sharing one headline, a device on each side (use Wings).
	{ID: "panorama-duo", Label: "Panorama duo", Span: 2, Text: &model.TextBand{Top: 0.06, Height: 0.22, Left: f(0.1), Width: f(0.8)}, Device: model.DeviceBand{Top: 0.3, Bottom: 1.25, Width: f(0.8), Cy: f(0.72)}, PadX: 0.09},
	{ID: "centered", Label: "Device only", Span: 1, Text: nil, Device: model.DeviceBand{Top: 0.07, Bottom: 0.93}, PadX: 0.1},
}

func f(v float64) *float64 { return &v }

// Positions place one or more device frames inside the layout's device slot.
// Placements draw back to front, and `source` picks which screenshot goes in
// each frame so a multi-device arrangement shows the neighbouring screens.
var Positions = []model.Position{
	{ID: "center", Label: "Single", Placements: []model.Placement{
		{Source: model.SourceSelf, Scale: 1},
	}},
	{ID: "tilted", Label: "Tilted", Placements: []model.Placement{
		{Source: model.SourceSelf, Scale: 0.98, Rotate: -7},
	}},
	{ID: "offset", Label: "Offset", Placements: []model.Placement{
		{Source: model.SourceSelf, Dx: 0.15, Dy: 0.01, Scale: 1.06, Rotate: -4},
	}},
	{ID: "right", Label: "Pushed right", Placements: []model.Placement{
		{Source: model.SourceSelf, Dx: 0.12, Dy: 0.02, Scale: 1},
	}},
	// Positive is clockwise on the canvas: the top swings right, away from
	// left-aligned copy.
	{ID: "lean", Label: "Lean", Placements: []model.Placement{
		{Source: model.SourceSelf, Scale: 1, Rotate: 10},
	}},
	{ID: "corner", Label: "Corner", Placements: []model.Placement{
		{Source: model.SourceSelf, Dx: 0.14, Dy: 0.03, Scale: 1, Rotate: 10},
	}},
	{ID: "wings", Label: "Wings", Placements: []model.Placement{
		{Source: model.SourceNext, Dx: 0.23, Scale: 1, Rotate: -6},
		{Source: model.SourceSelf, Dx: -0.23, Scale: 1, Rotate: 6},
	}},
	{ID: "duo", Label: "Duo", Placements: []model.Placement{
		{Source: model.SourceNext, Dx: -0.18, Dy: -0.05, Scale: 0.88},
		{Source: model.SourceSelf, Dx: 0.14, Dy: 0.05, Scale: 1},
	}},
	{ID: "duo-tilt", Label: "Duo tilt", Placements: []model.Placement{
		{Source: model.SourceNext, Dx: -0.18, Dy: -0.06, Scale: 0.9, Rotate: -6},
		{Source: model.SourceSelf, Dx: 0.16, Dy: 0.06, Scale: 1, Rotate: -6},
	}},
	{ID: "pair", Label: "Pair", Placements: []model.Placement{
		{Source: model.SourceNext, Dx: 0.16, Dy: -0.018, Scale: 0.84, Rotate: 7},
		{Source: model.SourceSelf, Dx: -0.09, Dy: 0.018, Scale: 0.94, Rotate: -3},
	}},
	{ID: "trio", Label: "Trio", Placements: []model.Placement{
		{Source: model.SourcePrev, Dx: -0.205, Dy: 0.012, Scale: 0.68, Rotate: -11},
		{Source: model.SourceNext, Dx: 0.205, Dy: 0.012, Scale: 0.68, Rotate: 11},
		{Source: model.SourceSelf, Dy: -0.012, Scale: 0.82},
	}},
}

// Export sizes. Only the largest device in each family is required — the
// stores downscale for the rest.
var Sizes = []model.ExportSize{
	{ID: "iphone-6-9", Label: `iPhone 6.9"`, Store: "App Store", W: 1320, H: 2868, Device: "iphone-17-pro"},
	{ID: "iphone-6-5", Label: `iPhone 6.5"`, Store: "App Store", W: 1242, H: 2688, Device: "iphone-17"},
	{ID: "ipad-13", Label: `iPad 13"`, Store: "App Store", W: 2064, H: 2752, Device: "ipad-13"},
	{ID: "android-phone", Label: "Android phone", Store: "Google Play", W: 1080, H: 1920, Device: "pixel-8a"},
	{ID: "android-phone-tall", Label: "Android phone (tall)", Store: "Google Play", W: 1080, H: 2400, Device: "pixel-9-pro"},
	{ID: "android-tablet", Label: "Android tablet", Store: "Google Play", W: 1600, H: 2560, Device: "android-tablet"},
	// The Ultra's own panel size. Apple's screenshot slot for it has moved
	// more than once, so this is the panel rather than a slot number to be
	// stale about: the renderer only needs an aspect ratio it can be trusted
	// with, and the number is one line to correct.
	{ID: "watch-ultra", Label: "Apple Watch Ultra", Store: "App Store", W: 410, H: 502, Device: "watch-ultra"},
}

// Target is the store slot for an export size, with the frame that belongs in
// it. Used when a size is added to a project and no frame was chosen.
func Target(sizeID string) model.Target {
	s := Size(sizeID)
	return model.Target{SizeID: s.ID, DeviceID: s.Device}
}

// Rhythm steps, in the vocabulary the pickers use: a layout × arrangement pair
// with an alignment.
var (
	StepClassic     = model.RhythmStep{Layout: "text-top", PositionID: "center", TextAlign: model.AlignCenter}
	StepCopyBelow   = model.RhythmStep{Layout: "text-bottom", PositionID: "center", TextAlign: model.AlignCenter}
	StepHero        = model.RhythmStep{Layout: "hero", PositionID: "center", TextAlign: model.AlignCenter}
	StepOffset      = model.RhythmStep{Layout: "hero", PositionID: "right", TextAlign: model.AlignLeft}
	StepTilt        = model.RhythmStep{Layout: "hero", PositionID: "tilted", TextAlign: model.AlignCenter}
	StepTiltRight   = model.RhythmStep{Layout: "hero", PositionID: "corner", TextAlign: model.AlignLeft}
	StepDuo         = model.RhythmStep{Layout: "duo", PositionID: "duo", TextAlign: model.AlignCenter}
	StepDuoTilt     = model.RhythmStep{Layout: "duo", PositionID: "duo-tilt", TextAlign: model.AlignCenter}
	StepPanorama    = model.RhythmStep{Layout: "panorama", PositionID: "lean", TextAlign: model.AlignLeft}
	StepPanoramaDuo = model.RhythmStep{Layout: "panorama-duo", PositionID: "wings", TextAlign: model.AlignCenter}
	StepMinimal     = model.RhythmStep{Layout: "centered", PositionID: "center"}
)

// Rhythms vary the composition across the strip. A rhythm shorter than the set
// repeats from its start; `uniform` leaves every tile on the global layout.
var Rhythms = []model.Rhythm{
	{ID: "uniform", Label: "Uniform", Description: "Every tile in the same layout and arrangement."},
	{ID: "editorial", Label: "Editorial", Description: "A panorama opener, then a hero, an offset, a breather and a tilt.",
		Steps: []model.RhythmStep{StepPanorama, StepHero, StepOffset, StepMinimal, StepTilt}},
	{ID: "showcase", Label: "Showcase", Description: "Hero first, then tilted and paired screens, ending on a breather.",
		Steps: []model.RhythmStep{StepHero, StepTilt, StepDuo, StepTiltRight, StepMinimal}},
	{ID: "magazine", Label: "Magazine", Description: "Left-aligned copy and copy-below tiles alternating with big devices.",
		Steps: []model.RhythmStep{StepOffset, StepCopyBelow, StepTiltRight, StepHero, StepMinimal}},
	{ID: "storyboard", Label: "Storyboard", Description: "A two-screen panorama, then a copy-below, a hero and a breather.",
		Steps: []model.RhythmStep{StepPanoramaDuo, StepCopyBelow, StepHero, StepMinimal, StepTilt}},
	{ID: "dynamic", Label: "Dynamic", Description: "Everything tilted: a tilt, a tilted pair, a panorama, a breather.",
		Steps: []model.RhythmStep{StepTilt, StepDuoTilt, StepPanorama, StepMinimal, StepTiltRight}},
}

// Lookups. Each falls back to the first row: an unknown id is a look nobody
// chose, which is a better failure than a blank editor.

func Device(id string) model.Device {
	for _, d := range Devices {
		if d.ID == id {
			return d
		}
	}
	return Devices[0]
}

func Frame(id string) model.FrameColor {
	for _, c := range FrameColors {
		if c.ID == id {
			return c
		}
	}
	return FrameColors[0]
}

// EveryStructure is one variant per composition structure, in the order they
// read best down a store page: the wide opener, then the ordinary tiles, then
// the wordless breather.
//
// This is what a template lays out instead of asking. Nine structures shown as
// nine cards to choose between is nine near-identical pictures and a decision
// nobody has the information to make before they have seen their own
// screenshots in them — so the set arrives with one of each, in the project's
// own look, and the author deletes the ones they do not want. Removing is a
// judgement you can make by looking; choosing up front is not.
//
// Two of these span two store tiles. SyncParts turns those into two screens
// apiece, so a set laid out from this is eleven tiles, not nine.
func EveryStructure() []model.Overrides {
	order := []string{
		"panorama", "hero", "text-top", "duo", "editorial",
		"bleed", "panorama-duo", "text-bottom", "centered",
	}
	out := make([]model.Overrides, 0, len(order))
	for _, id := range order {
		l := LayoutOf(id)
		o := model.Overrides{
			Layout:     s(l.ID),
			PositionID: s(ArrangementFor(l.ID, "")),
		}
		// The wide structures put their words in a narrow column beside the
		// device, where centred text reads as a mistake.
		if l.Span > 1 && l.ID == "panorama" {
			o.TextAlign = s(model.AlignLeft)
		}
		out = append(out, o)
	}
	return out
}

// ArrangementFor is the placement set a structure needs to look like itself.
//
// `duo` and the panoramas have room for a second device and draw an empty half
// under a single-device arrangement, which reads as broken rather than roomy —
// so picking a structure has to carry its arrangement with it. Every other
// layout keeps whatever the author already chose; a structure change is not a
// reason to throw away a tilt they set on purpose.
func ArrangementFor(layoutID, current string) string {
	switch layoutID {
	case "duo":
		if current == "duo" || current == "duo-tilt" {
			return current
		}
		return "duo"
	case "panorama-duo":
		return "wings"
	case "panorama":
		return "lean"
	}
	// Coming *from* a multi-device arrangement into a single-device structure
	// leaves a neighbour's screenshot drawn over the tile, so that has to go
	// back to something single.
	switch current {
	case "duo", "duo-tilt", "wings", "pair", "trio", "":
		return "center"
	}
	return current
}

func LayoutOf(id string) model.Layout {
	for _, l := range Layouts {
		if l.ID == id {
			return l
		}
	}
	return Layouts[0]
}

func PositionOf(id string) model.Position {
	for _, p := range Positions {
		if p.ID == id {
			return p
		}
	}
	return Positions[0]
}

func Size(id string) model.ExportSize {
	for _, s := range Sizes {
		if s.ID == id {
			return s
		}
	}
	return Sizes[0]
}

// Span is how many store tiles a screen's composition covers. The canvas is
// Span tiles wide and the export slices it back into tiles.
//
// It lives here rather than in the renderer because the browser needs the
// answer — a panorama is two <img> tags, and the client has to know that to
// repaint the strip — and the renderer is a rasteriser with megabytes of fonts
// embedded in it that must never enter a wasm build.
func Span(screen model.Screen, settings model.Settings) int {
	return LayoutOf(screen.Overrides.Apply(settings).Layout).Span
}

// Sync reconciles a project's compositions with its layouts: it is
// model.SyncParts bound to the layout table, and it is what both sides call
// after an edit.
//
// It lives here because resolving a layout is a table lookup and the model is a
// leaf that cannot do one. Every write path runs it — the browser's optimistic
// apply, the server's patch closure — so a set never holds a panorama with one
// half missing or a stray half whose composition is gone.
func Sync(p *model.Project) {
	p.SyncParts(func(s model.Screen) int { return Span(s, p.Settings) })
}

// ShowsText reports whether a screen's resolved layout has a text band. The
// readiness counter uses it: a device-only tile with no headline is finished,
// not incomplete.
func ShowsText(screen model.Screen, settings model.Settings) bool {
	return LayoutOf(screen.Overrides.Apply(settings).Layout).Text != nil
}

func RhythmOf(id string) model.Rhythm {
	for _, r := range Rhythms {
		if r.ID == id {
			return r
		}
	}
	return Rhythms[0]
}
