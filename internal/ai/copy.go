package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// The two copy tasks. Both go out as one request covering the whole set rather
// than one per screen: a store listing's headlines have to work as a sequence —
// no repeated verbs, a promise then a proof then a close — and a model shown
// one screen at a time cannot do that.

// A Brief describes one screen to the model. The filename is included because
// it is often the only thing that says what the screen actually shows;
// `03-recipe-import.png` carries real information.
type Brief struct {
	ScreenID string `json:"id"`
	File     string `json:"screenshot"`
	Headline string `json:"current_headline,omitempty"`
	Subhead  string `json:"current_subhead,omitempty"`
}

// Write drafts copy for a whole set in one language.
func Write(ctx context.Context, r Runner, p model.Project, briefs []Brief, locale string, brief string) (map[string]model.Copy, error) {
	if len(briefs) == 0 {
		return nil, nil
	}
	briefJSON, _ := json.MarshalIndent(briefs, "", "  ")

	// The author's brief beats the stored description. It was typed for this
	// run, in the modal that asked for it, and it is the only place anyone
	// says who the app is *for* — a model given a feature list writes a
	// feature list back.
	about := describe(p)
	if strings.TrimSpace(brief) != "" {
		about = brief
	}

	// The role line is not decoration. A coding agent's default register is
	// documentation, and documentation on a store tile reads as a manual: "Manage
	// your shopping lists" instead of "Never lose the list again". Naming the
	// job changes the register before the rules get a chance to.
	prompt := fmt.Sprintf(`You are an expert marketing copywriter. You write App Store and Google Play
screenshot copy — the short lines set over a picture of the app — and you are
very good at it. This is the copy that decides whether someone installs.

The app is called %q.

What it does and who it is for, in the author's own words:
%s

Write a headline and an optional subtitle for each of these screens, in %s.
The screenshots are named in a way that usually says what the screen shows.

%s

Rules:
- The headlines are read as a sequence, in this order. Together they should
  tell one story: the promise, then the proof, then the reason to install. Do
  not repeat the same verb or the same sentence shape twice in a row.
- A headline is at most 6 words. It goes on a picture of a phone, not in a
  paragraph. Say what the person gets, not what the app has.
- The subtitle is optional and is one short sentence. Leave it empty unless it
  adds something the headline could not.
- Three kinds of emphasis, and at most one per headline. *Single asterisks*
  draw a highlighter band behind the phrase. **Double asterisks** set it in a
  second colour with no band, which suits a product name or the one word the
  sentence turns on. ***Triple asterisks*** do both, and are for the single
  strongest line in the set if any. Use them on the word that carries the
  meaning, not on every screen — most headlines want none of the three.
- Write in %s as a native speaker of it would. This is not a translation of
  English marketing; if the idiomatic phrasing is different, use it.
- Do not use \n. The renderer breaks lines to fit, and a break written into a
  headline that is later shown at another store size lands in the wrong place.
  It is there for an author adjusting one tile by eye, not for a draft.
- No exclamation marks. No "revolutionary", "seamless", "effortless", or
  "game-changing".

Reply with only a JSON array, one object per screen, in the same order:
[{"id": "<the id from above>", "headline": "…", "subhead": "…"}]`,
		p.Name, about, language(locale), briefJSON, language(locale))

	return request(ctx, r, prompt, briefs)
}

// Translate carries an existing set into another language. Given the source
// copy rather than the screenshots, because a translation that ignores what
// the English said is not a translation.
func Translate(ctx context.Context, r Runner, p model.Project, briefs []Brief, from, to string) (map[string]model.Copy, error) {
	if len(briefs) == 0 {
		return nil, nil
	}
	briefJSON, _ := json.MarshalIndent(briefs, "", "  ")

	prompt := fmt.Sprintf(`You are localising App Store screenshot copy for an app called %q, from %s into %s.

What the app does:
%s

%s

Rules:
- This is marketing copy on a picture, not documentation. Write what a native
  %s copywriter would write to mean the same thing. Rephrase freely; a literal
  translation that reads like a translation is a failure.
- Keep it short. These sit above a phone and the app shrinks type that does not
  fit — a headline twice as long as the English one will render small.
- The asterisks are markup: *these words* get a highlighter band behind them.
  Keep exactly the same number of marked spans, on the words that carry the
  meaning in %s. Never mark a particle or an article.
- Keep product names, and anything that is a proper noun in the source,
  unchanged.
- If the source subtitle is empty, leave the translation empty.

Reply with only a JSON array, one object per screen, in the same order:
[{"id": "<the id from above>", "headline": "…", "subhead": "…"}]`,
		p.Name, language(from), language(to), describe(p), briefJSON, language(to), language(to))

	return request(ctx, r, prompt, briefs)
}

// request runs the prompt and turns the reply into copy keyed by screen id.
// Anything the model made up an id for is dropped: an edit applied to a screen
// that does not exist would be silent, and a silent no-op is worse than a
// visible gap.
func request(ctx context.Context, r Runner, prompt string, briefs []Brief) (map[string]model.Copy, error) {
	reply, err := r.Run(ctx, prompt)
	if err != nil {
		return nil, err
	}
	raw, err := extractJSON(reply)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		ID       string `json:"id"`
		Headline string `json:"headline"`
		Subhead  string `json:"subhead"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("the reply was not the array asked for: %w", err)
	}

	known := map[string]bool{}
	for _, b := range briefs {
		known[b.ScreenID] = true
	}
	out := map[string]model.Copy{}
	for _, row := range rows {
		if !known[row.ID] {
			continue
		}
		out[row.ID] = model.Copy{
			Headline: strings.TrimSpace(row.Headline),
			Subhead:  strings.TrimSpace(row.Subhead),
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("nothing in the reply matched a screen in this project")
	}
	return out, nil
}

// describe is the grounding: what the app is. Without it a model writes
// plausible marketing about nothing in particular, which is the failure mode
// that makes people stop using a feature like this.
func describe(p model.Project) string {
	if strings.TrimSpace(p.Description) != "" {
		return p.Description
	}
	if p.Listing.Subtitle != "" {
		return p.Listing.Subtitle
	}
	return "(The author has not described it. Infer what you can from the screenshot filenames, and keep the copy general rather than inventing features.)"
}

// language names a locale in a way a model reads better than a tag: "ja" is
// ambiguous in a sentence, "Japanese (ja)" is not.
func language(tag string) string {
	l := presets.LocaleOf(tag)
	if l.Label == tag {
		return tag
	}
	return fmt.Sprintf("%s (%s)", l.Label, tag)
}
