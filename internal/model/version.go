package model

import "strings"

// MaxVersionName is the length of the name field. Apple's is 32 characters and
// it is free text there too — "1.0.1", "1.1", "2026.3-beta" are all legal, and
// nothing here parses it, because a scheme this app invented would eventually
// disagree with the one the store accepts.
const MaxVersionName = 32

// A Version is one submission: the metadata the store asks for, and the
// screenshots that go up with it.
//
// It owns the screens rather than the project owning them, because that is
// what a version *is*. Shipping 1.1 means new captures and a new "What's New"
// beside the old ones, and an author who wants last release's set back wants
// the set exactly as it was — not the current set with the differences
// remembered somewhere else. The look stays on the project: a house style is
// the thing that does not change between releases, and the Look step is
// global for the same reason.
type Version struct {
	ID string `json:"id"`
	// Name is what the store calls this build. Free text, capped at
	// [MaxVersionName].
	Name string `json:"name"`
	// Copyright is deliberately not in [Metadata]. App Store Connect has one
	// copyright per version for the whole app — it is not a per-language
	// field, and offering it once per language would invite eleven answers to
	// a question with one.
	Copyright string `json:"copyright,omitempty"`
	// Store is the listing text, keyed by locale tag, the way a screen's copy
	// is. A locale with no entry falls back to the base one, so adding a
	// language never blanks the page.
	Store map[string]Metadata `json:"store,omitempty"`

	// Targets are the store slots this submission ships to — one per device
	// family. Per version rather than per project because a release does not
	// have to go everywhere the last one did: an iPad build that slipped, or a
	// phone-only point release, is an ordinary thing to ship, and a project
	// with one list would make the export write directories for a device this
	// version has nothing to say about.
	//
	// Never empty. [Project.version] does not repair this — [Project.Targets]
	// does, because a version with no slot has no aspect ratio to draw a
	// preview in and every tile in the editor would have nowhere to go.
	Targets []Target `json:"targets"`

	Screens []Screen `json:"screens"`
}

// Metadata is what App Store Connect asks for per language, per version.
//
// The limits are Apple's, and they are here rather than in the page because
// the count under a field and the check before an export have to agree. They
// are counts of characters, not bytes: a Japanese description is 4000
// characters there too, and len() on a UTF-8 string would reject a valid one
// at about a third of its length.
type Metadata struct {
	// Promotional appears above the description and can be changed without
	// submitting a build, which is what it is for.
	Promotional string `json:"promotional,omitempty"`
	Description string `json:"description,omitempty"`
	// WhatsNew is required for every version after the first.
	WhatsNew string `json:"whats_new,omitempty"`
	// Keywords is one comma-separated list, and the commas count.
	Keywords     string `json:"keywords,omitempty"`
	SupportURL   string `json:"support_url,omitempty"`
	MarketingURL string `json:"marketing_url,omitempty"`
}

// Field limits, in characters. Apple's.
const (
	MaxPromotional = 170
	MaxDescription = 4000
	MaxWhatsNew    = 4000
	MaxKeywords    = 100
	MaxURL         = 255
	MaxCopyright   = 200
)

// Written reports whether anything has been filled in. Used for "this language
// has no listing yet" without asking about six fields at every call site.
func (m Metadata) Written() bool {
	return m != Metadata{}
}

// Text is the listing in a locale, falling back to the base language — the
// same rule a screen's copy follows, for the same reason: adding a language
// must never blank the page, it must show what will be submitted if nobody
// translates it.
func (v Version) Text(locale, base string) Metadata {
	if m, ok := v.Store[locale]; ok && m.Written() {
		return m
	}
	return v.Store[base]
}

// Own is the listing actually stored for a locale, with no fallback. What the
// form edits: a field showing the base language's words must not save them
// into the language being looked at the moment anything else is typed.
func (v Version) Own(locale string) Metadata { return v.Store[locale] }

// SetStore writes one locale's listing, dropping it when it is emptied so a
// blank language does not persist as an entry that Written() has to reason
// about.
func (v *Version) SetStore(locale string, m Metadata) {
	if !m.Written() {
		delete(v.Store, locale)
		return
	}
	if v.Store == nil {
		v.Store = map[string]Metadata{}
	}
	v.Store[locale] = m
}

// MissingListing is the languages with no description of their own. The
// description is the one field the store will not accept an empty value for,
// so it is what "this language is not ready" means.
func (p Project) MissingListing() []string {
	v := p.current()
	var out []string
	for _, tag := range p.Locales {
		if tag == p.BaseLocale {
			continue
		}
		if !v.Own(tag).Written() {
			out = append(out, tag)
		}
	}
	return out
}

// Clamp trims a field to what the store accepts, in characters rather than
// bytes: a Japanese description is 4000 characters there too, and len() on a
// UTF-8 string would cut a valid one at about a third of its length.
func Clamp(v string, max int) string {
	r := []rune(strings.TrimSpace(v))
	if len(r) > max {
		return string(r[:max])
	}
	return string(r)
}

// ClampName trims a version name to what the store accepts. Applied on write
// rather than rejected, because a name one character too long is a typo and
// refusing the whole save teaches nothing.
func ClampName(name string) string { return Clamp(name, MaxVersionName) }
