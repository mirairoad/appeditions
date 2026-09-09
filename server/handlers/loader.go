// Package handlers is the server half the typed endpoint layer does not
// cover: the page loader, the image renderer, and the multipart upload.
//
// Everything JSON-shaped lives in server/apis, where the file's location is
// its URL and the Go types are the contract. What is here is what does not fit
// that shape — a PNG body, a multipart body, and the per-render context the
// pages read.
package handlers

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	clientstore "github.com/mirairoad/appeditions/client/store"
	"github.com/mirairoad/appeditions/client/view"
	"github.com/mirairoad/appeditions/internal/ai"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/internal/render"
	"github.com/mirairoad/appeditions/internal/store"
)

// Loader turns a request into the values the pages read. Pages take no
// arguments and may not reach a database, so this is the only place a page's
// data comes from.
type Loader struct {
	Store   *store.Store
	Version string
}

// Data is app.Config.Data: called once per render, before the page.
//
// It dispatches on the path rather than on a route name because the route
// table is generated and this runs before the page is chosen. A path that
// matches nothing gets the shell and nothing else, which is what a 404 needs.
func (l *Loader) Data(ctx context.Context, path string) context.Context {
	ctx = view.WithShell(ctx, l.shell())

	seg := segments(path)
	switch {
	case len(seg) == 0:
		return l.projects(ctx)
	case seg[0] == "templates":
		return l.library(ctx)
	case seg[0] == "projects" && len(seg) >= 2:
		return l.editor(ctx, seg[1])
	}
	return ctx
}

func (l *Loader) shell() view.Shell {
	var providers []view.Provider
	for _, p := range ai.Available() {
		providers = append(providers, view.Provider{ID: p.ID, Label: p.Label})
	}
	return view.Shell{Version: l.Version, Providers: providers, DataDir: l.Store.Root()}
}

func (l *Loader) projects(ctx context.Context) context.Context {
	items, err := l.Store.ListProjects(ctx)
	if err != nil {
		return ctx
	}
	templates, _ := l.Store.ListTemplates(ctx)

	cards := make([]view.ProjectCard, 0, len(items))
	for _, p := range items {
		card := view.ProjectCard{
			Project:   p,
			Readiness: readiness(p, p.BaseLocale),
			Updated:   since(p.UpdatedAt),
		}
		// The cover is the first screen with a picture in it. A card showing an
		// empty slot would be showing the template rather than the project.
		// Not a slice of a wider composition, though: half a panorama on a
		// card is half a picture, cut at a column chosen for a phone screen.
		for _, s := range p.Screens() {
			if s.Filled() && !s.Follows() {
				card.CoverScreen = s.ID
				break
			}
		}
		cards = append(cards, card)
	}

	return view.WithProjects(ctx, view.Projects{
		Items:     cards,
		Templates: templates,
		Locales:   presets.Locales,
		Sizes:     presets.Sizes,
		Devices:   presets.Devices,
	})
}

func (l *Loader) library(ctx context.Context) context.Context {
	templates, err := l.Store.ListTemplates(ctx)
	if err != nil {
		return ctx
	}
	return view.WithLibrary(ctx, view.Library{Templates: templates})
}

func (l *Loader) editor(ctx context.Context, id string) context.Context {
	p, err := l.Store.Project(ctx, id)
	if err != nil {
		return ctx
	}

	req := view.RequestOf(ctx)
	locale := req.Param("locale")
	if !p.HasLocale(locale) {
		locale = p.BaseLocale
	}

	// The submission being looked at, before anything else reads the project:
	// the screens, the listing and the readiness count all hang off it, and a
	// page that resolved it later would draw one version's tiles under another
	// version's count.
	//
	// Persisted, not just applied in memory. The version is not a *view* of
	// the document the way the size and the language are — it selects which
	// set is being edited, and every write endpoint loads the project fresh
	// and resolves through the stored VersionID. Left in memory, switching to
	// 1.0 in the sidebar and typing a headline wrote it into 1.1, silently and
	// with nothing on screen changing.
	//
	// One write per switch, and only when it actually differs.
	if v := req.Param("version"); v != "" && v != p.VersionID {
		if _, ok := p.VersionOf(v); ok {
			if saved, err := l.Store.Edit(ctx, p.ID, func(p *model.Project) {
				p.UseVersion(v)
			}); err == nil {
				p = saved
			}
		}
	}

	// The store slot being looked at. Resolved into the settings before
	// anything else reads them, so the spans, the readiness and every preview
	// URL on the page all describe the same target rather than three.
	if t, ok := p.TargetOf(req.Param("size")); ok {
		p.Use(t)
	}

	assets, _ := l.Store.AssetsOf(ctx, p.ID)
	uses := assetUses(p)
	shots := make([]view.Asset, 0, len(assets))
	for _, a := range assets {
		use := uses[a.Doc.ID]
		shots = append(shots, view.Asset{
			ID: a.Doc.ID, Name: a.Name, Width: a.Width, Height: a.Height,
			Used: use != "", Use: use,
		})
	}

	templates, _ := l.Store.ListTemplates(ctx)

	// The target being looked at has to be resolved before the export
	// directories are listed, or they are listed for the wrong one.
	if t, ok := p.TargetOf(req.Param("size")); ok {
		p.Use(t)
	}
	exports := map[string]string{}
	for _, t := range p.Targets() {
		for _, tag := range p.Locales {
			dir := l.Store.ExportDir(p, tag, t.SizeID)
			if exists(dir) {
				exports[view.SetKey(t.SizeID, tag)] = dir
			}
		}
	}

	// Wire 1 of 3: the server puts the document on the context, the page
	// renders from it, and this paint happens with no JavaScript at all. The
	// browser's store is hydrated from the same shape in Mount.
	ctx = clientstore.WithEditor(ctx, p)

	// The same constructor the wasm renderer calls after a local mutation.
	// Everything derived from the document is filled there; what is added here
	// is what only a server can answer.
	e := view.NewEditor(p, locale, req.Param("size"), req.Param("screen"))
	e.Templates = templates
	e.Assets = shots
	e.Exports = exports
	e.ExportsDir = l.Store.ExportsDir(p)
	return view.WithEditor(ctx, e)
}

// selected drops a screen id that is no longer in the set, so a stale link
// scopes the tune panel to the whole project instead of to nothing.
func selected(p model.Project, id string) string {
	if _, ok := p.Screen(id); ok {
		return id
	}
	return ""
}

// readiness is the one reading of how far along a set is. The rail, the status
// line and the review page all call this; two readings would eventually
// disagree about whether the author may export.
func readiness(p model.Project, locale string) model.Readiness {
	r := p.Readiness(locale, func(s model.Screen) bool { return render.ShowsText(s, p.Settings) })
	return r.WithStore(presets.Size(p.Settings.SizeID).Store)
}

// since is a coarse relative time. Coarse on purpose: the exact minute a
// project was touched is never the question, and "3 days ago" is read faster
// than a timestamp.
func since(ms int64) string {
	if ms == 0 {
		return "just now"
	}
	d := time.Since(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d ago"
	}
	return time.UnixMilli(ms).Format("2 Jan 2006")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func segments(path string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if start >= 0 {
				out = append(out, path[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	return out
}

// assetUses says what each original is being used for, or nothing when it is
// spare. What the Media panel reads to decide whether to offer a delete.
//
// Every version, not the one being looked at, and the icon as well as the
// screens. Both were missed the first time: the project's icon is an ordinary
// asset, so it listed as "unused" and offered a delete that would have taken
// the icon away, and a picture used only by last release read as spare on this
// one. The store refuses both — this is the half that has to agree with it, or
// the panel offers buttons that always fail.
func assetUses(p model.Project) map[string]string {
	out := map[string]string{}
	if p.IconAssetID != "" {
		out[p.IconAssetID] = "the app icon"
	}
	// Which versions hold it, named, so a row that cannot be deleted says
	// where to go and delete the screen instead.
	in := map[string][]string{}
	for _, v := range p.Versions {
		for _, screen := range v.Screens {
			if screen.AssetID == "" {
				continue
			}
			held := in[screen.AssetID]
			if len(held) == 0 || held[len(held)-1] != v.Name {
				in[screen.AssetID] = append(held, v.Name)
			}
		}
	}
	for id, versions := range in {
		if out[id] != "" {
			continue // the icon wins: it is the shorter, truer answer
		}
		out[id] = "the set in " + strings.Join(versions, ", ")
	}
	return out
}
