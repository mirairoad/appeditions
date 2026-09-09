package ui

// The icon set, as data rather than as one templ per icon: an icon is its
// inner paths plus the few attributes that differ from the default, and Icon
// renders the wrapper. Adding one is a row.
//
// Everything is 24×24, stroke currentColor at 1.5, no fill — the outline
// weight the rest of the interface is drawn at. Icons inherit their row's
// colour, so size them with the class argument and never with a width
// attribute.
type IconDef struct {
	// ViewBox defaults to "0 0 24 24".
	ViewBox string
	// Fill defaults to "none". Set it to "currentColor" for a solid icon, and
	// leave Stroke empty when you do.
	Fill string
	// Stroke defaults to "currentColor" unless Fill is set.
	Stroke      string
	StrokeWidth string
	// Body is the inner SVG markup: paths and circles, nothing else.
	Body string
}

// Icons is the registry. An unknown name renders nothing rather than a broken
// glyph: a missing icon should cost a missing icon and not a missing row.
var Icons = map[string]IconDef{
	// The five steps of the flow, in order. Each one is the thing it produces,
	// not an abstraction of it: the look is stacked cards, the screenshots are
	// a picture, the copy is a letter.
	"look": {Body: `<path d="m12 3 9 5-9 5-9-5Z"/><path d="m3 13 9 5 9-5"/>`},
	// A tag: the version, and the words that ship with it.
	"release": {Body: `<path d="M20.6 13.4 12 4.8V2H4v8h2.8l8.6 8.6a2 2 0 0 0 2.8 0l2.4-2.4a2 2 0 0 0 0-2.8Z"/><circle cx="7.5" cy="6.5" r=".8"/>`},
	"shots":   {Body: `<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="9.5" r="1.5"/><path d="m4 17 5-5 4 4 3-2 4 4"/>`},
	"copy":    {Body: `<path d="M5 6V4h14v2"/><path d="M12 4v16"/><path d="M9 20h6"/>`},
	// A phone body: which store slots a release ships to.
	"device": {Body: `<rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/>`},
	// Two arrows round a cycle: swap the picture in this tile for another.
	"swap":   {Body: `<path d="M17 3 21 7l-4 4"/><path d="M21 7H8a4 4 0 0 0-4 4"/><path d="M7 21l-4-4 4-4"/><path d="M3 17h13a4 4 0 0 0 4-4"/>`},
	"tune":   {Body: `<path d="M5 21V14"/><path d="M5 10V3"/><path d="M12 21v-9"/><path d="M12 8V3"/><path d="M19 21v-5"/><path d="M19 12V3"/><path d="M2 14h6"/><path d="M9 12h6"/><path d="M16 16h6"/>`},
	"export": {Body: `<path d="M12 3v12"/><path d="m8 11 4 4 4-4"/><path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>`},

	"projects":  {Body: `<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`},
	"templates": {Body: `<rect x="3" y="3" width="7" height="18" rx="1.5"/><rect x="14" y="3" width="7" height="8" rx="1.5"/><rect x="14" y="15" width="7" height="6" rx="1.5"/>`},
	"language":  {Body: `<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a15 15 0 0 1 0 18a15 15 0 0 1 0-18"/>`},
	// The AI actions. A wand rather than a robot: what it does here is write a
	// draft, and the author keeps or replaces it.
	"sparkle":  {Body: `<path d="m12 4 1.6 4.4L18 10l-4.4 1.6L12 16l-1.6-4.4L6 10l4.4-1.6Z"/><path d="M18 16.5 18.7 18l1.5.7-1.5.7L18 21l-.7-1.6-1.5-.7 1.5-.7Z"/>`},
	"menu":     {Body: `<path d="M4 6h16"/><path d="M4 12h16"/><path d="M4 18h16"/>`},
	"plus":     {Body: `<path d="M12 5v14"/><path d="M5 12h14"/>`},
	"close":    {Body: `<path d="M6 6l12 12"/><path d="M18 6 6 18"/>`},
	"check":    {Body: `<path d="m5 13 4 4L19 7"/>`},
	"edit":     {Body: `<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/>`},
	"trash":    {Body: `<path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v5"/><path d="M14 11v5"/>`},
	"back":     {Body: `<path d="M15 6l-6 6 6 6"/>`},
	"chevron":  {Body: `<path d="m6 9 6 6 6-6"/>`},
	"forward":  {Body: `<path d="M5 12h14"/><path d="m13 6 6 6-6 6"/>`},
	"alert":    {Body: `<path d="M12 3 2.8 19h18.4Z"/><path d="M12 9v4"/><path d="M12 16h.01"/>`},
	"info":     {Body: `<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>`},
	"dot":      {Fill: "currentColor", Body: `<circle cx="12" cy="12" r="4"/>`},
	"folder":   {Body: `<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/><path d="M8 13h8"/>`},
	"upload":   {Body: `<path d="M12 17V5"/><path d="m8 9 4-4 4 4"/><path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>`},
	"save":     {Body: `<path d="M5 5a2 2 0 0 1 2-2h8l4 4v12a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2Z"/><path d="M8 3v6h7"/><path d="M8 15h8"/>`},
	"copy-doc": {Body: `<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1"/>`},
	// Reordering a screen in the strip.
	"up":   {Body: `<path d="M12 19V5"/><path d="m6 11 6-6 6 6"/>`},
	"down": {Body: `<path d="M12 5v14"/><path d="m6 13 6 6 6-6"/>`},
}

// icon resolves a name and fills in the defaults, so a registry entry only
// states what differs.
func icon(name string) (IconDef, bool) {
	def, ok := Icons[name]
	if !ok {
		return IconDef{}, false
	}
	if def.ViewBox == "" {
		def.ViewBox = "0 0 24 24"
	}
	if def.Fill == "" {
		def.Fill = "none"
	}
	if def.Stroke == "" && def.Fill == "none" {
		def.Stroke = "currentColor"
	}
	if def.StrokeWidth == "" && def.Stroke != "" {
		def.StrokeWidth = "1.5"
	}
	return def, true
}
