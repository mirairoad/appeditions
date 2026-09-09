package presets

import "github.com/mirairoad/appeditions/internal/model"

// Pointer helpers. Overrides are pointers because nil means inherit, so every
// template value has to be addressable — this is the price of being able to
// say "this screen has no backdrop" and "this screen does not care" in the
// same struct.
func s(v string) *string                     { return &v }
func b(v model.Background) *model.Background { return &v }
func hl(v ...string) *[]string               { return &v }

// step turns a rhythm step into the three overrides it pins. A variant that
// carries a composition and a variant that carries colours are the same kind
// of thing, so a template can mix them freely.
func step(st model.RhythmStep, extra ...func(*model.Overrides)) model.Overrides {
	o := model.Overrides{Layout: s(st.Layout), PositionID: s(st.PositionID)}
	if st.TextAlign != "" {
		o.TextAlign = s(st.TextAlign)
	}
	for _, fn := range extra {
		fn(&o)
	}
	return o
}

// BuiltinTemplates ship with the app. They are seeded into the database on
// every start rather than read from here at use time, which is what makes them
// editable: a copy is a real row, and a bad edit to a built-in is one restart
// away from being restored.
// BuiltinTemplates ship with the app. They are seeded into the database on
// every start rather than read from here at use time, which is what makes them
// editable: a copy is a real row, and a bad edit to a built-in is one restart
// away from being restored.
//
// Two of them, deliberately. Four shipped looks that differ mostly in their
// palette read as four ways of doing the same thing, and the choice they ask
// for is not one anybody wants to make on the first screen. Light and dark is
// the choice that actually exists — everything past it is the Fine-tune step,
// and a look worth keeping is saved as a template of your own.
//
// The ids stay `classic` and `midnight`. They are the stable keys a stored
// project points at and a reseed matches on; renaming them would orphan every
// project that already chose one.
var BuiltinTemplates = []model.Template{
	{
		ID:          "classic",
		Label:       "Light",
		Description: "Near-white background, dark type, a framed device under a centred headline.",
		Builtin:     true,
		Settings: model.Overrides{
			Background:       b(model.Gradient("#ffffff", "#eef2f7", 165)),
			BackdropColor:    s("#e8ecf2"),
			FrameColorID:     s("black"),
			Layout:           s("hero"),
			PositionID:       s("center"),
			TextColor:        s("#0f172a"),
			TextAlign:        s(model.AlignCenter),
			Highlights:       hl("#bfdbfe", "#fde68a"),
			FontID:           s("inter"),
			HeadlineScale:    f(1.2),
			SubheadScale:     f(0.9),
			HeadlineTracking: f(-0.03),
		},
		// One slot per composition structure. A new project opens on the whole
		// range of them in its own colours, and the author deletes down to the
		// set they want — which is a judgement you can make by looking, unlike
		// picking a structure before you have seen your screenshots in it.
		Variants: EveryStructure(),
		Samples: []model.Sample{
			{Headline: "Everything in *one place*", Subhead: "Notes, tasks and files together."},
			{Headline: "Plan the week in seconds", Subhead: "Drag, drop, done."},
			{Headline: "Works *offline*, syncs later"},
		},
	},
	{
		ID:          "midnight",
		Label:       "Dark",
		Description: "Deep blue-black, high contrast type, one big device per tile. Reads well next to a light store page.",
		Builtin:     true,
		Settings: model.Overrides{
			Background:       b(model.Gradient("#1e293b", "#0f172a", 160)),
			BackdropColor:    s(""),
			FrameColorID:     s("silver"),
			Layout:           s("hero"),
			PositionID:       s("center"),
			TextColor:        s("#f8fafc"),
			TextAlign:        s(model.AlignCenter),
			Highlights:       hl("#38bdf8", "#a78bfa"),
			FontID:           s("space-grotesk"),
			HeadlineScale:    f(1.2),
			SubheadScale:     f(0.9),
			HeadlineTracking: f(-0.03),
		},
		Variants: EveryStructure(),
		Samples: []model.Sample{
			{Headline: "Built for *focus*", Subhead: "Everything else gets out of the way."},
			{Headline: "Fast, and *quiet*"},
			{Headline: "Your data stays *yours*", Subhead: "Nothing leaves the device."},
		},
	},
}

func Template(id string) model.Template {
	for _, t := range BuiltinTemplates {
		if t.ID == id {
			return t
		}
	}
	return BuiltinTemplates[0]
}
