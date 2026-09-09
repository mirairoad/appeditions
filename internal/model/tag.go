package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// PreviewTag identifies a drawing: a hash of everything the renderer reads to
// draw one tile. It is the preview's ETag *and* the `v` in its URL, so the two
// can never disagree about what a tile is.
//
// It lives in the vocabulary rather than in the exporter because client/ui
// writes it into the tile's src and that package compiles to wasm — reaching
// it through the exporter would pull in the renderer, and the renderer embeds
// twelve megabytes of fonts.
//
// Neighbour screenshots are in the hash because arrangements draw them: a
// two-device placement shows [SourceNext], so replacing screen 2's picture
// changes screen 1's tile. Without them that tile revalidated to a 304 and
// kept the old drawing.
//
// They are hashed whether or not the current arrangement reaches for them.
// Asking the preset table which sources are live would be exact, and the cost
// of being wrong is not symmetric: over-hashing redraws two neighbouring tiles
// for nothing, under-hashing pins a stale tile behind an immutable URL for
// good.
func PreviewTag(p Project, screen Screen, copy Copy, width int) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	enc.Encode(screen.Overrides.Apply(p.Settings)) //nolint:errcheck // a hash writer cannot fail
	enc.Encode(copy)                               //nolint:errcheck

	// Sorted, because map iteration order is randomised and a tag that changed
	// between two renders of the same state would make every request a miss.
	sources := p.SourceAssets(screen.ID)
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s;", k, sources[k])
	}

	fmt.Fprintf(h, "%s/%d/%d", screen.AssetID, width, screen.Part)
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// TemplateTag identifies a template card's drawing, the same way [PreviewTag]
// identifies a tile's. The Look step re-renders on every change and the
// gallery is five of these, so without it every card blanked for a
// revalidation each time a template or a rhythm was applied — measured: 20
// template-preview renders in three seconds of clicking, at ~55 ms each.
//
// What the handler draws: the template's settings over the defaults, with the
// project's size and device if the URL named them, its first variant and its
// first sample. So that is what is hashed.
func TemplateTag(t Template, sizeID, deviceID string, width, part int) string {
	settings := DefaultSettings()
	if sizeID != "" {
		settings.SizeID = sizeID
	}
	if deviceID != "" {
		settings.DeviceID = deviceID
	}
	settings = t.Settings.Apply(settings)

	h := sha256.New()
	enc := json.NewEncoder(h)
	enc.Encode(t.Variant(0).Apply(settings)) //nolint:errcheck // a hash writer cannot fail
	enc.Encode(t.Sample(0))                  //nolint:errcheck
	fmt.Fprintf(h, "%s/%d/%d", t.ID, width, part)
	return hex.EncodeToString(h.Sum(nil)[:16])
}
