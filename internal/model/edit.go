package model

import (
	"maps"
	"strconv"
	"strings"
)

// The editing operations. They live on the project rather than in the store
// because none of them touch a database: applying a template is a
// transformation of one document into another, and keeping it here means it can
// be tested without opening SQLite and reasoned about without reading SQL.
//
// IDs come in from the caller rather than being minted here. The store applies
// these inside an optimistic-lock retry, and a closure that mints its own ids
// produces different ones on the second attempt.

// ApplyTemplate resets the project to a look. A template is a reset, not a
// patch: everything goes back to the defaults and the template's own values are
// laid on top, so switching looks cannot leave a stray tilt behind from the
// last one.
//
// The export size and the device survive, because they are decisions about
// where the set is going rather than what it looks like.
func (p *Project) ApplyTemplate(t Template, newID func() string) {
	v := p.version()

	base := DefaultSettings()
	// Carried over only when they were actually chosen. A project mid-creation
	// has neither yet, and copying "" here would leave the project with no
	// export size — which the defaulting then reads as "unconfigured" and
	// repairs by throwing the whole template away.
	if p.Settings.SizeID != "" {
		base.SizeID = p.Settings.SizeID
	}
	if p.Settings.DeviceID != "" {
		base.DeviceID = p.Settings.DeviceID
	}
	p.Settings = t.Settings.Apply(base)

	// A set template lays out every slot up front so the whole look is visible
	// before a single screenshot exists. Screens that already hold one keep it
	// and their copy; the rest become empty slots carrying the template's
	// sample text.
	// Compositions, not screens. The trailing part of a panorama holds a copy
	// of its lead's screenshot; keeping it here would turn one picture into
	// two slots and the set would grow by one every time a template was
	// applied. SyncParts puts the parts back afterwards.
	var filled []Screen
	for _, s := range v.Screens {
		if s.Filled() && !s.Follows() {
			filled = append(filled, s)
		}
	}
	count := max(t.Slots(), len(filled))

	screens := make([]Screen, 0, count)
	for i := range count {
		if i < len(filled) {
			s := filled[i]
			s.Overrides = t.Variant(i)
			screens = append(screens, s)
			continue
		}
		screens = append(screens, p.emptySlot(t, i, newID()))
	}
	v.Screens = screens

	p.TemplateID = t.ID
	switch {
	case t.Rhythm != "":
		p.RhythmID = t.Rhythm
	case t.Slots() > 0:
		// The variants are the rhythm. Naming it as such is what stops the
		// rhythm picker showing "Uniform" over a set that visibly is not.
		p.RhythmID = "template"
	default:
		p.RhythmID = "uniform"
	}
}

// emptySlot is a screen with no screenshot, carrying the template's sample
// copy in the project's base language.
func (p *Project) emptySlot(t Template, index int, id string) Screen {
	sample := t.Sample(index)
	return Screen{
		ID:        id,
		Copy:      map[string]Copy{p.BaseLocale: {Headline: sample.Headline, Subhead: sample.Subhead}},
		Overrides: t.Variant(index),
	}
}

// ApplyRhythm pins each screen's composition to the rhythm's step for its
// index. Only the three composition keys move — a Notebook screen keeps its
// paper colour and its highlights, because those are the look and this is the
// pacing.
// Compositions are stepped, not screens: a rhythm counts the pictures in the
// strip, and the trailing part of a panorama is the same picture as the one
// before it. Counting it would give the rhythm an extra beat every time a
// panorama appeared, and re-applying the same rhythm would produce a different
// strip each time. The parts take their lead's overrides from SyncParts.
func (p *Project) ApplyRhythm(r Rhythm) {
	v := p.version()

	beat := 0
	for i := range v.Screens {
		if v.Screens[i].Follows() {
			continue
		}
		o := v.Screens[i].Overrides.Clear("layout", "position_id", "text_align")
		if step := r.Step(beat); step != nil {
			o.Layout = &step.Layout
			o.PositionID = &step.PositionID
			if step.TextAlign != "" {
				align := step.TextAlign
				o.TextAlign = &align
			}
		}
		v.Screens[i].Overrides = o
		beat++
	}
	p.RhythmID = r.ID
}

// A Shot is one screenshot arriving from an upload.
type Shot struct {
	ScreenID string
	AssetID  string
	Title    string
}

// AddShots fills the empty slots in order and appends whatever is left over.
// Filling first is what makes a set template work: the author picked a look
// with five slots and dropped five files, and the files should land in those
// slots rather than after them.
func (p *Project) AddShots(shots []Shot, t Template, r Rhythm) {
	v := p.version()

	for _, shot := range shots {
		if i := p.firstEmpty(); i >= 0 {
			// The slot keeps its sample copy, so the look stays intact and the
			// author replaces the words when they get to them.
			v.Screens[i].AssetID = shot.AssetID
			continue
		}
		// Counted in compositions, so a set holding a panorama does not skip a
		// variant for the half that is not a picture of its own.
		index := len(p.Leads())
		screen := Screen{
			ID:      shot.ScreenID,
			AssetID: shot.AssetID,
			Copy:    map[string]Copy{p.BaseLocale: {Headline: shot.Title}},
			// Continue the template's variant cycle and the rhythm, so a file
			// dropped later matches the ones dropped first.
			Overrides: t.Variant(index),
		}
		if step := r.Step(index); step != nil {
			screen.Overrides.Layout = &step.Layout
			screen.Overrides.PositionID = &step.PositionID
			if step.TextAlign != "" {
				align := step.TextAlign
				screen.Overrides.TextAlign = &align
			}
		}
		v.Screens = append(v.Screens, screen)
	}
}

// firstEmpty skips the trailing parts of a composition. Their screenshot is
// the lead's, and a file dropped into one would be copied over by the next
// SyncParts — an upload that vanished with nothing to explain it.
func (p *Project) firstEmpty() int {
	v := p.version()

	for i := range v.Screens {
		if !v.Screens[i].Filled() && !v.Screens[i].Follows() {
			return i
		}
	}
	return -1
}

// SyncParts reconciles the screens that are parts of one composition: a screen
// whose layout spans n tiles is followed into the set by n-1 more screens, one
// per remaining tile, and each of them carries a copy of what the lead draws.
//
// This is what makes a panorama two ordinary screenshots. The alternative —
// one screen that the export quietly cut in two — meant the halves could not
// be reordered, moved apart or deleted, because there was nothing in the
// document to move.
//
// span resolves a layout, which means reading the preset tables; this package
// is a leaf and deliberately does not know about them, so the caller supplies
// it the way Readiness does. Run it after every edit — presets.Sync is the
// bound form both sides call.
//
// The follower ids are derived from the lead's rather than minted. This runs
// inside the document store's optimistic-lock retry, and a closure that minted
// its own would produce a different id on the second attempt: the retry would
// append a second right half instead of rewriting the same one.
func (p *Project) SyncParts(span func(Screen) int) {
	v := p.version()

	spans := map[string]int{}
	for _, s := range v.Screens {
		if !s.Follows() {
			spans[s.ID] = max(span(s), 1)
		}
	}

	// Drop a part whose lead is gone, or whose lead no longer reaches it: a
	// panorama put back to a single-tile layout must not leave its right half
	// in the set, drawing a slice of a composition that is no longer that wide.
	kept := make([]Screen, 0, len(v.Screens))
	seen := map[string]bool{}
	for _, s := range v.Screens {
		if s.Follows() {
			if n, ok := spans[s.Of]; !ok || s.Part < 1 || s.Part >= n || seen[s.ID] {
				continue
			}
			seen[s.ID] = true
		}
		kept = append(kept, s)
	}

	// The lead is the only place a composition is edited, so the parts are
	// rewritten from it rather than kept in step: there is one truth and a
	// copy of it, not two truths.
	leads := map[string]Screen{}
	for _, s := range kept {
		if !s.Follows() {
			leads[s.ID] = s
		}
	}

	out := make([]Screen, 0, len(kept)+2)
	for _, s := range kept {
		if s.Follows() {
			out = append(out, part(leads[s.Of], s.ID, s.Part))
			continue
		}
		out = append(out, s)
		// A missing part is added directly after its lead, which is where it
		// belongs until someone moves it. Existing ones are left wherever they
		// were put.
		for i := 1; i < spans[s.ID]; i++ {
			if seen[partID(s.ID, i)] {
				continue
			}
			out = append(out, part(s, partID(s.ID, i), i))
		}
	}
	v.Screens = out
}

func partID(lead string, part int) string { return lead + "-" + strconv.Itoa(part+1) }

// part is one tile of a lead's composition as a screen of its own: the same
// picture and the same words, cut at a different column.
func part(lead Screen, id string, i int) Screen {
	return Screen{
		ID:      id,
		AssetID: lead.AssetID,
		// Cloned rather than shared. Two screens pointing at one map is a
		// write to either showing up in both, which is right until the pair is
		// broken up and then very wrong.
		Copy:      maps.Clone(lead.Copy),
		Overrides: lead.Overrides,
		Of:        lead.ID,
		Part:      i,
	}
}

// SetCopy writes one screen's words in one language. Addressed to a part of a
// composition it writes the composition's, because the halves of a panorama
// share one headline — there is one drawing and the words are in it.
func (p *Project) SetCopy(screenID, locale string, c Copy) {
	s, ok := p.Lead(screenID)
	if !ok {
		return
	}
	if s.Copy == nil {
		s.Copy = map[string]Copy{}
	}
	s.Copy[locale] = c
}

// MoveScreen shifts a screen by delta positions, clamped to the set. Overrides
// travel with it: the pose and the colour a template pinned belong to the
// screen, not to the slot it happened to be in.
func (p *Project) MoveScreen(id string, delta int) {
	v := p.version()

	from := p.IndexOf(id)
	to := from + delta
	if from < 0 || to < 0 || to >= len(v.Screens) {
		return
	}
	moved := v.Screens[from]
	// Built rather than spliced in place: append into a slice's own prefix
	// aliases the backing array, and the bug that produces is a screen that
	// appears twice.
	without := make([]Screen, 0, len(v.Screens)-1)
	for i, s := range v.Screens {
		if i != from {
			without = append(without, s)
		}
	}
	out := make([]Screen, 0, len(v.Screens))
	out = append(out, without[:to]...)
	out = append(out, moved)
	out = append(out, without[to:]...)
	v.Screens = out
}

// RemoveScreen drops a screen, and with it the rest of its composition.
//
// Deleting one half of a panorama deletes both. The alternative is a lead
// whose missing half grows straight back on the next SyncParts, which reads as
// "delete did nothing" — and a composition with a tile missing is not a set
// anyone would upload.
func (p *Project) RemoveScreen(id string) {
	v := p.version()

	s, ok := p.Screen(id)
	if !ok {
		return
	}
	group := s.ID
	if s.Follows() {
		group = s.Of
	}
	out := v.Screens[:0]
	for _, s := range v.Screens {
		if s.ID == group || s.Of == group {
			continue
		}
		out = append(out, s)
	}
	v.Screens = out
}

// AddLocale adds a language. Nothing is translated: every screen falls back to
// the base language until words exist, so adding Japanese never blanks a
// preview.
func (p *Project) AddLocale(tag string) {
	if tag == "" || p.HasLocale(tag) {
		return
	}
	p.Locales = append(p.Locales, tag)
}

// RemoveLocale drops a language and the copy written in it. The base language
// cannot be removed — it is what every other one falls back to.
func (p *Project) RemoveLocale(tag string) {
	v := p.version()

	if tag == p.BaseLocale {
		return
	}
	out := p.Locales[:0]
	for _, l := range p.Locales {
		if l != tag {
			out = append(out, l)
		}
	}
	p.Locales = out
	for i := range v.Screens {
		delete(v.Screens[i].Copy, tag)
	}
}

// MissingCopy lists the screens that have no headline in a language but do
// have one in the base language — exactly the work an AI translation pass, or
// a person, has left to do.
func (p *Project) MissingCopy(locale string) []string {
	var ids []string
	for _, s := range p.version().Screens {
		if locale == p.BaseLocale {
			continue
		}
		// A composition is translated once. Its trailing parts carry a copy of
		// the same words, and listing them would have the model write the same
		// headline twice and bill for it.
		if s.Follows() {
			continue
		}
		if strings.TrimSpace(s.Copy[p.BaseLocale].Headline) == "" {
			continue
		}
		if strings.TrimSpace(s.Copy[locale].Headline) == "" {
			ids = append(ids, s.ID)
		}
	}
	return ids
}
