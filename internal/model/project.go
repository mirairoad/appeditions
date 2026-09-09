package model

import (
	"errors"
	"maps"
	"strconv"
	"strings"
	"unicode"
)

// A Project is one app's screenshot set: the look, the screens, and the
// languages it ships in. It is the unit of persistence — everything the editor
// touches lives in one document, so a change is one patch and one version.
//
// ID carries no JSON tag on purpose. The stored type embeds this alongside the
// database envelope, which owns "id"; the store copies the envelope's id into
// this field on the way out so a page has the identifier without importing the
// database package.
type Project struct {
	ID string `json:"-"`
	// UpdatedAt is the envelope's timestamp, copied in on the way out of the
	// store. Untagged for the same reason ID is: the envelope owns it, and two
	// fields writing "meta" would drop both.
	UpdatedAt int64  `json:"-"`
	Name      string `json:"name"`
	// Slug names the project's directory under the data root. Derived from
	// Name once, at creation, and then left alone: renaming a project must not
	// move the assets out from under the screens that point at them.
	Slug string `json:"slug"`
	// IconAssetID is the app's icon, stored like any other asset. It is the
	// project's identity in a list of them: a name in a grid of cards is a
	// row of text, and everyone recognises their own icon faster than they
	// read.
	IconAssetID string `json:"icon_asset_id,omitempty"`
	// Description is what the app does, in the author's words. It is the
	// context every AI copy request is grounded in — without it the model
	// writes plausible marketing about nothing.
	Description string `json:"description"`

	Settings Settings `json:"settings"`

	// Versions are the submissions, oldest first, and each one owns its own
	// screens. VersionID is the one being edited; it is always one of these,
	// because [Project.version] puts it back if it is not.
	Versions  []Version `json:"versions"`
	VersionID string    `json:"version_id"`

	// LegacyScreens is where a document written before versions existed keeps
	// its set: the screens sat on the project. [Project.Migrate] folds it into
	// a version and clears it, and omitempty keeps it out of everything
	// written since — so it is empty in every document this build writes.
	//
	// It has to be exported to be decoded at all. Nothing should read it;
	// [Project.Screens] is the accessor.
	LegacyScreens []Screen `json:"screens,omitempty"`

	// BaseLocale is the language the author writes in and every other locale
	// is translated from. It is always present in Locales.
	BaseLocale string   `json:"base_locale"`
	Locales    []string `json:"locales"`

	// LegacyTargets is where a document written before versions owned their
	// own store slots keeps them. Folded into every version by
	// [Project.Migrate] and cleared; omitempty keeps it out of anything
	// written since. Use [Project.Targets].
	LegacyTargets []Target `json:"targets,omitempty"`

	// TemplateID is the last look applied; new screens pick up its variant
	// cycle. RhythmID is the strip's rhythm — "uniform", a built-in id, or
	// "template" when the template's own variants set it.
	TemplateID string `json:"template_id"`
	RhythmID   string `json:"rhythm_id"`

	Listing Listing `json:"listing"`
	Format  string  `json:"format"`

	// AIProvider is which local CLI writes and translates copy for this
	// project: "claude", "codex", or "" for manual only.
	AIProvider string `json:"ai_provider"`
}

// DefaultSettings is what a new project starts from and what applying a
// template resets to before the template's own values are laid on top.
func DefaultSettings() Settings {
	return Settings{
		Background:       Solid("#eaf2ff"),
		BackdropColor:    "",
		DeviceID:         "iphone-17-pro",
		FrameColorID:     "deep-blue",
		PositionID:       "center",
		Layout:           "text-top",
		Tilt:             0,
		DeviceScale:      1,
		TextColor:        "#111114",
		TextAlign:        AlignCenter,
		Highlights:       []string{"#ffe27a"},
		FontID:           "inter",
		HeadlineScale:    1,
		SubheadScale:     1,
		HeadlineTracking: -0.01,
		SizeID:           "iphone-6-9",
	}
}

// version is the submission being edited, and it is never nil: a project with
// no versions gets one, and a VersionID naming a version that is not there
// falls back to the last. Every read and every write goes through here, so
// there is one answer to "which set is this" and nothing downstream has to
// handle the empty case.
func (p *Project) version() *Version {
	if len(p.Versions) == 0 {
		p.Versions = []Version{{ID: "v_1", Name: "1.0"}}
	}
	for i := range p.Versions {
		if p.Versions[i].ID == p.VersionID {
			return &p.Versions[i]
		}
	}
	p.VersionID = p.Versions[len(p.Versions)-1].ID
	return &p.Versions[len(p.Versions)-1]
}

// current resolves the version being edited without touching the project.
//
// The read paths use this and the write paths use [Project.version]: a value
// receiver cannot mint a missing version and does not need to, and having the
// reads go through the mutating one meant every `len(p.Screens())` in a page
// wanted an addressable project.
func (p Project) current() Version {
	for _, v := range p.Versions {
		if v.ID == p.VersionID {
			return v
		}
	}
	if len(p.Versions) > 0 {
		return p.Versions[len(p.Versions)-1]
	}
	return Version{}
}

// Version is the submission being edited.
func (p Project) Version() Version { return p.current() }

// VersionOf resolves a version by id.
func (p Project) VersionOf(id string) (Version, bool) {
	for _, v := range p.Versions {
		if v.ID == id {
			return v, true
		}
	}
	return Version{}, false
}

// Use points the project at a version. Everything the editor reads — the
// screens, the listing, the readiness count — follows from this one field, the
// way the store slot follows from Settings.SizeID.
func (p *Project) UseVersion(id string) bool {
	if _, ok := p.VersionOf(id); !ok {
		return false
	}
	p.VersionID = id
	return true
}

// Targets is the current version's store slots, never empty.
//
// Repaired here rather than in version(): a document can legitimately arrive
// with none — a version created before slots were per-version, or one whose
// last slot was somehow dropped — and every caller wants a list it can index,
// not an error it has to decide about. The default is the one Settings already
// names, so the repair agrees with what the editor is drawing.
func (p Project) Targets() []Target {
	if t := p.current().Targets; len(t) > 0 {
		return t
	}
	return []Target{{SizeID: p.Settings.SizeID, DeviceID: p.Settings.DeviceID}}
}

// Screens is the current version's set. A method rather than a field because
// which set it is depends on the version selected, and two places resolving
// that would eventually disagree.
func (p Project) Screens() []Screen { return p.current().Screens }

// EditVersion is the submission being edited, for writing. The pointer escapes
// this package on purpose: the release form sets six fields at once, and six
// setters would be six chances for one of them to write into the wrong
// version.
func (p *Project) EditVersion() *Version { return p.version() }

// AddVersion starts the next submission from the current one and selects it.
//
// A copy, not an empty set. Shipping 1.1 is last release's screenshots with
// two replaced and a new "What's New"; starting from nothing would mean
// re-uploading a set the author already has. The listing comes across too —
// the description rarely changes between point releases, and What's New is the
// field the author is here to rewrite anyway.
//
// The screens are deep-copied. They carry maps, and two versions sharing one
// Copy map would have an edit to 1.1's headline silently rewrite 1.0's.
func (p *Project) AddVersion(name string) {
	from := p.current()
	next := Version{
		ID:        mintVersionID(p.Versions),
		Name:      ClampName(name),
		Copyright: from.Copyright,
		// The slots come across too. A new release ships where the last one
		// shipped until somebody says otherwise — starting at none meant the
		// accessor's one-slot fallback took over, and 1.1 quietly went to the
		// phone alone after 1.0 had gone to the phone and the iPad.
		Targets: append([]Target(nil), p.Targets()...),
		Screens: make([]Screen, 0, len(from.Screens)),
		Store:   map[string]Metadata{},
	}
	for _, s := range from.Screens {
		s.Copy = maps.Clone(s.Copy)
		next.Screens = append(next.Screens, s)
	}
	for tag, m := range from.Store {
		next.Store[tag] = m
	}
	p.Versions = append(p.Versions, next)
	p.VersionID = next.ID
}

// RemoveVersion drops a submission. The last one cannot go: a project with no
// versions has nowhere to put a screenshot, and the editor would be looking at
// a set that does not exist.
func (p *Project) RemoveVersion(id string) error {
	if len(p.Versions) < 2 {
		return errors.New("a project keeps at least one version")
	}
	out := p.Versions[:0]
	for _, v := range p.Versions {
		if v.ID != id {
			out = append(out, v)
		}
	}
	if len(out) == len(p.Versions) {
		return errors.New("no such version")
	}
	p.Versions = out
	if p.VersionID == id {
		p.VersionID = p.Versions[len(p.Versions)-1].ID
	}
	return nil
}

// mintVersionID counts rather than hashes, because these are read in URLs and
// in a log line. It skips ids already taken, so deleting v_2 and adding one
// does not resurrect the deleted one's links.
func mintVersionID(existing []Version) string {
	taken := make(map[string]bool, len(existing))
	for _, v := range existing {
		taken[v.ID] = true
	}
	for n := len(existing) + 1; ; n++ {
		id := "v_" + strconv.Itoa(n)
		if !taken[id] {
			return id
		}
	}
}

// SetScreens replaces the current version's set. The one write path from
// outside this package: everything else that changes the set — adding shots,
// moving, removing — is a method here, because the rules for what a set may
// contain (the halves of a panorama, above all) belong with the set.
func (p *Project) SetScreens(screens []Screen) { p.version().Screens = screens }

// Migrate brings a stored document up to the current shape. Called once, on
// the way out of the store, so nothing downstream has to know what an older
// document looked like.
//
// Before versions existed the screens sat on the project. That set becomes
// version 1.0 — not an empty version beside an orphaned set, which is what
// dropping the field would have produced: a project that opened with every
// screenshot gone and no way to tell that they were still in the file.
func (p *Project) Migrate() {
	if len(p.LegacyScreens) > 0 && len(p.Versions) == 0 {
		p.Versions = []Version{{ID: "v_1", Name: "1.0", Screens: p.LegacyScreens}}
		p.VersionID = "v_1"
	}
	p.LegacyScreens = nil

	// Store slots moved onto the version after the screens did, so a document
	// can be halfway: versions already, targets still at the top. Every
	// version gets the old list — they all shipped to those slots, because
	// there was only one list at the time. Which of them a release *stops*
	// shipping to is a decision, and it is made on the Release step.
	if len(p.LegacyTargets) > 0 {
		for i := range p.Versions {
			if len(p.Versions[i].Targets) == 0 {
				p.Versions[i].Targets = append([]Target(nil), p.LegacyTargets...)
			}
		}
	}
	p.LegacyTargets = nil

	p.version() // mints the first version, and repairs a dangling VersionID
	p.Targets() // and a version with no slot to draw in
}

// Screen returns the screen with this id and whether it was found.
func (p *Project) Screen(id string) (*Screen, bool) {
	v := p.version()

	for i := range v.Screens {
		if v.Screens[i].ID == id {
			return &v.Screens[i], true
		}
	}
	return nil, false
}

// Lead resolves a screen id to the screen that owns the composition it belongs
// to. The halves of a panorama are two screens but one drawing, so every edit
// to either of them — the words, a pinned setting, a replaced screenshot — is
// an edit to the drawing, and lands here.
func (p *Project) Lead(id string) (*Screen, bool) {
	s, ok := p.Screen(id)
	if !ok {
		return nil, false
	}
	if s.Follows() {
		return p.Screen(s.Of)
	}
	return s, true
}

// Leads is the compositions, in order: the screens that own a drawing. The
// arrangements that show a neighbouring screenshot walk this rather than
// Screens, or a panorama's own second half would be its "next" and a trio
// would draw the same picture twice.
func (p *Project) Leads() []Screen {
	v := p.version()

	out := make([]Screen, 0, len(v.Screens))
	for _, s := range v.Screens {
		if !s.Follows() {
			out = append(out, s)
		}
	}
	return out
}

// LeadIndex is a screen's position among the compositions, which is the index
// the neighbour sources are resolved against.
func (p *Project) LeadIndex(id string) int {
	if s, ok := p.Lead(id); ok {
		id = s.ID
	}
	for i, s := range p.Leads() {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// SourceAssets is which screenshot each placement source draws for one tile:
// the composition's own, and its neighbours', keyed by [SourceSelf],
// [SourceNext] and [SourcePrev]. A key is absent when that composition has no
// screenshot yet, which is what the placeholder is for.
//
// One implementation, two readers — the exporter loads these bytes and
// [PreviewTag] hashes these ids. Two walks of the ring would eventually
// disagree about what a tile draws, and under an immutable preview URL that
// disagreement is a tile that never updates again.
func (p *Project) SourceAssets(screenID string) map[string]string {
	leads := p.Leads()
	index := p.LeadIndex(screenID)
	if len(leads) == 0 || index < 0 {
		return nil
	}
	out := map[string]string{}
	for key, offset := range map[string]int{
		SourceSelf: 0,
		SourceNext: 1,
		SourcePrev: -1,
	} {
		j := ((index+offset)%len(leads) + len(leads)) % len(leads)
		if id := leads[j].AssetID; id != "" {
			out[key] = id
		}
	}
	return out
}

// Unpin drops one control's override from every screen, so a value set for the
// whole set actually reaches the whole set.
//
// This is what "the set wins" means. An override is an exception, and the way
// you make an exception here is by selecting a screen — so a change made with
// *no* screen selected is a statement about all of them, and leaving three
// screens pinned to last week's tilt is not what anybody meant by moving the
// whole-set slider. Before this, tuning one screen and then moving the set
// silently did nothing to that screen, and the only cure was to find it again
// and press "follow the set".
//
// Deliberately not applied to a scoped write: that one is the exception, and
// pinning it is the entire point.
func (p *Project) Unpin(key string) {
	v := p.version()

	for i := range v.Screens {
		v.Screens[i].Overrides = v.Screens[i].Overrides.Clear(key)
	}
}

func (p *Project) IndexOf(id string) int {
	v := p.version()

	for i := range v.Screens {
		if v.Screens[i].ID == id {
			return i
		}
	}
	return -1
}

// HasTarget reports whether the project ships this export size.
func (p *Project) HasTarget(sizeID string) bool {
	for _, t := range p.version().Targets {
		if t.SizeID == sizeID {
			return true
		}
	}
	return false
}

// TargetOf returns the target for an export size.
func (p *Project) TargetOf(sizeID string) (Target, bool) {
	for _, t := range p.version().Targets {
		if t.SizeID == sizeID {
			return t, true
		}
	}
	return Target{}, false
}

// AddTarget adds a store slot, or updates the frame of one already there.
func (p *Project) AddTarget(t Target) {
	v := p.version()

	if t.SizeID == "" {
		return
	}
	for i := range v.Targets {
		if v.Targets[i].SizeID == t.SizeID {
			if t.DeviceID != "" {
				v.Targets[i].DeviceID = t.DeviceID
			}
			return
		}
	}
	v.Targets = append(v.Targets, t)
}

// RemoveTarget drops a store slot. The last one cannot be removed: a project
// that ships to nothing has no size to render at, and every preview in the
// editor would have no aspect ratio to be drawn in.
func (p *Project) RemoveTarget(sizeID string) {
	v := p.version()

	if len(v.Targets) <= 1 {
		return
	}
	out := v.Targets[:0]
	for _, t := range v.Targets {
		if t.SizeID != sizeID {
			out = append(out, t)
		}
	}
	v.Targets = out

	// The size being designed against has to be one the project still ships.
	if !p.HasTarget(p.Settings.SizeID) {
		p.Use(v.Targets[0])
	}
}

// Use points the project's settings at one of its targets. This is what the
// editor's size switcher does, and what the export does once per target: the
// renderer takes a single Settings, so "render at this size" is "resolve the
// target into Settings first".
func (p *Project) Use(t Target) {
	p.Settings.SizeID = t.SizeID
	if t.DeviceID != "" {
		p.Settings.DeviceID = t.DeviceID
	}
}

// Active is the target the settings currently point at.
func (p *Project) Active() Target {
	v := p.version()

	if t, ok := p.TargetOf(p.Settings.SizeID); ok {
		return t
	}
	if len(v.Targets) > 0 {
		return v.Targets[0]
	}
	return Target{SizeID: p.Settings.SizeID, DeviceID: p.Settings.DeviceID}
}

// HasLocale reports whether the project ships this language.
func (p *Project) HasLocale(tag string) bool {
	for _, l := range p.Locales {
		if l == tag {
			return true
		}
	}
	return false
}

// Store limits that decide whether a set can be uploaded at all. Counted in
// uploaded files, which is one per screen: a panorama's halves are two screens
// and take two of the store's slots.
var storeLimits = map[string][2]int{
	"App Store":   {1, 10},
	"Google Play": {2, 8},
}

// Readiness is the one reading of "how far along is this set", shared by the
// rail, the footer and the review page. Two readings would eventually
// disagree, and the disagreement would be about whether the user may export.
type Readiness struct {
	Total        int
	Filled       int
	MissingShots int
	// MissingCopy counts screens whose layout shows copy but whose headline is
	// empty in the locale being looked at.
	MissingCopy int
	Tiles       int
	Store       string
	Min, Max    int
	OverLimit   bool
	UnderMin    bool
	// CanExport is the hard gate: every slot filled. Copy and store limits are
	// advice — the author may know something the counter does not.
	CanExport bool
}

// Readiness measures the set in one locale. One screen is one store tile and
// one uploaded file, so the tile count is the screen count — a panorama's two
// halves are two screens and were already counted as two.
//
// Copy is counted per composition rather than per screen: the halves share one
// headline, and reporting a panorama as two missing headlines would send the
// author looking for a second field that does not exist.
func (p *Project) Readiness(locale string, showsText func(Screen) bool) Readiness {
	v := p.version()

	r := Readiness{Total: len(v.Screens), Tiles: len(v.Screens)}
	for _, s := range v.Screens {
		if s.Filled() {
			r.Filled++
		}
		if s.Follows() {
			continue
		}
		if showsText(s) && strings.TrimSpace(s.Text(locale, p.BaseLocale).Headline) == "" {
			r.MissingCopy++
		}
	}
	r.MissingShots = r.Total - r.Filled
	r.CanExport = r.Total > 0 && r.Filled == r.Total
	return r
}

// WithStore fills in the store limits for an export size. Split from Readiness
// so the caller resolves the size once.
func (r Readiness) WithStore(store string) Readiness {
	r.Store = store
	limit, ok := storeLimits[store]
	if !ok {
		return r
	}
	r.Min, r.Max = limit[0], limit[1]
	r.OverLimit = r.Tiles > r.Max
	r.UnderMin = r.Total > 0 && r.Tiles < r.Min
	return r
}

// Slug turns a name into a directory-safe identifier. Falls back to the given
// default rather than returning "", because the result names a directory and
// an empty one would put a project's assets in the data root.
func Slug(text, fallback string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
			dash = false
		default:
			// Non-ASCII names (a Japanese app title, say) would otherwise
			// collapse to nothing; one dash per run keeps the shape readable.
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		return fallback
	}
	return out
}
