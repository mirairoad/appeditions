package handlers

import (
	"archive/zip"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mirairoad/appeditions/internal/exporter"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
	"github.com/mirairoad/appeditions/internal/render"
	"github.com/mirairoad/appeditions/internal/store"
)

// Register mounts the handlers whose bodies are not JSON: a PNG, and a
// multipart upload. Everything else is a typed endpoint under server/apis.
func Register(mux *http.ServeMux, s *store.Store) {
	mux.Handle("GET /api/preview/{project}/{screen}", preview(s))
	mux.Handle("GET /api/template-preview/{template}", templatePreview(s))
	mux.Handle("POST /api/projects/{id}/shots", upload(s))
	mux.Handle("GET /api/projects/{id}/icon", icon(s))
	mux.Handle("GET /api/assets/{asset}", original(s))
	mux.Handle("GET /api/projects/{id}/export.zip", exportZip(s))
	mux.Handle("POST /api/projects/{id}/edit", edit(s))
}

// maxPreviewWidth bounds what a URL can ask to have rendered. Without it the
// tile URL is a request to render an arbitrary number of pixels. The editor's
// own largest is the tune page's 420px tile at 2x, so 840.
const maxPreviewWidth = 2048

// preview renders one screen. This is the editor's entire drawing surface:
// every tile on every page is an <img> pointed here, so there is no canvas in
// the browser and no second implementation of the renderer to keep in step.
func preview(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Project(r.Context(), r.PathValue("project"))
		if err != nil {
			http.Error(w, "no such project", http.StatusNotFound)
			return
		}
		screen, ok := p.Screen(r.PathValue("screen"))
		if !ok {
			http.Error(w, "no such screen", http.StatusNotFound)
			return
		}

		locale := r.URL.Query().Get("locale")
		if !p.HasLocale(locale) {
			locale = p.BaseLocale
		}
		// Which store slot. The page carries it in the tile's URL because this
		// handler loads the project fresh and cannot see which target the page
		// it is drawn on is showing.
		if t, ok := p.TargetOf(r.URL.Query().Get("size")); ok {
			p.Use(t)
		}
		width := clamp(intParam(r, "w", 240), 40, maxPreviewWidth)

		// The tag covers everything the drawing depends on. The editor reloads
		// every tile after any change, so most of these requests are answerable
		// without rendering anything — and the ones that are not are exactly
		// the tiles that actually changed.
		version := model.PreviewTag(p, *screen, screen.Text(locale, p.BaseLocale), width)
		tag := `"` + version + `"`
		w.Header().Set("ETag", tag)
		// Cached hard when the URL names the drawing, exactly as the icon is.
		// A local navigation replaces #outlet wholesale, so every tile is a
		// new <img> element on every page change and every state change —
		// and under no-cache a new element has no bitmap until a revalidation
		// comes back, which is a blank tile for one round trip.
		//
		// Chrome hid this behind its in-memory image cache, which reuses the
		// decoded bitmap for an identical URL without asking the server at
		// all — measured: 0 preview requests across a refresh. WKWebView does
		// not, so the window flashed on every change and the browser did not.
		// Naming the drawing in the URL is what removes the round trip rather
		// than relying on an engine to skip it.
		if r.URL.Query().Get("v") == version {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		body, _, err := exporter.Preview(r.Context(), s, p, screen.ID, locale, width)
		if err != nil {
			slog.Error("preview", "project", p.ID, "screen", screen.ID, "err", err)
			http.Error(w, "could not draw that", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(body) //nolint:errcheck // a broken pipe is the client leaving
	})
}

// templatePreview draws a template over the placeholder screenshot, so the
// gallery shows what applying it produces rather than a description of it.
func templatePreview(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := s.Template(r.Context(), r.PathValue("template"))
		if err != nil {
			http.Error(w, "no such template", http.StatusNotFound)
			return
		}

		settings := model.DefaultSettings()
		if id := r.URL.Query().Get("size_id"); id != "" {
			settings.SizeID = id
		}
		if id := r.URL.Query().Get("device_id"); id != "" {
			settings.DeviceID = id
		}
		settings = t.Settings.Apply(settings)

		// The first variant, because that is the tile a set opens on. A card
		// showing the base look of a template whose variants change every
		// screen would be showing something the author never sees.
		screen := model.Screen{ID: "sample", Overrides: t.Variant(0)}
		sample := t.Sample(0)

		size := presets.Size(settings.SizeID)
		width := clamp(intParam(r, "w", 150), 40, 600)
		height := width * size.H / size.W

		shot := render.Placeholder()
		full := render.Scene(width, height, screen, settings,
			model.Copy{Headline: sample.Headline, Subhead: sample.Subhead},
			render.Sources{model.SourceSelf: shot, model.SourceNext: shot, model.SourcePrev: shot})

		// One store tile, like everywhere else. A panorama template drawn
		// whole is a near-square picture of two phones in one frame — which is
		// not a thing this application ever delivers, and it made the card the
		// odd one out in a grid of portrait tiles. The card asks for each tile
		// separately and shows them side by side, as two files.
		var img image.Image = full
		span := presets.Span(screen, settings)
		if span > 1 {
			part := clamp(intParam(r, "part", 0), 0, span-1)
			img = full.SubImage(image.Rect(part*width, 0, (part+1)*width, height))
		}

		w.Header().Set("Content-Type", "image/png")
		// Named in the URL and cached hard, like the tiles: the Look step
		// re-renders its whole gallery on every change, so under no-cache
		// every card went back to the server before it would paint and the
		// five of them blanked together each time.
		version := model.TemplateTag(t, r.URL.Query().Get("size_id"), r.URL.Query().Get("device_id"), width, intParam(r, "part", 0))
		w.Header().Set("ETag", `"`+version+`"`)
		if r.URL.Query().Get("v") == version {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if r.Header.Get("If-None-Match") == `"`+version+`"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		png.Encode(w, img) //nolint:errcheck // a broken pipe is the client leaving
	})
}

// original serves an uploaded screenshot as it arrived, for the Media panel's
// thumbnails.
//
// Cached hard with no version in the URL, which is safe here and nowhere else:
// the bytes under an asset id cannot change. A new picture is a new asset —
// PutAsset hashes the content and hands back the existing row for a duplicate
// — so the id *is* the version.
func original(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, err := s.Asset(r.Context(), r.PathValue("asset"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		p, err := s.Project(r.Context(), asset.ProjectID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, s.Path(p, asset))
	})
}

// exportZip is the last export as one file, so it can be saved somewhere the
// author chose rather than only living under the data directory.
//
// A zip because a folder is not a thing a browser can be handed, and this is a
// tree — one directory per device and language, which is the shape App Store
// Connect wants them in. The entries keep that shape, so unzipping gives back
// exactly what is on disk.
//
// It streams from what the export already wrote rather than re-rendering:
// re-running the renderer here would be a second export path, and the whole
// design rests on there being one.
func exportZip(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Project(r.Context(), r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		root := s.ExportsDir(p)
		if _, err := os.Stat(root); err != nil {
			http.Error(w, "nothing exported yet", http.StatusNotFound)
			return
		}

		name := p.Slug + "-" + model.Slug(p.Version().Name, "version") + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)

		zw := zip.NewWriter(w)
		defer zw.Close() //nolint:errcheck // the client hanging up is the exit path
		//nolint:errcheck // a broken pipe ends the walk; there is nothing to report to
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			f, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close() //nolint:errcheck // read-only
			_, err = io.Copy(f, src)
			return err
		})
	})
}

// upload takes the dropped files. Multipart, so it cannot be one of the typed
// endpoints: those are JSON in and JSON out, deliberately.
func upload(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Project(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "no such project", http.StatusNotFound)
			return
		}
		// 64 MB in memory before spilling to disk. A store screenshot is a few
		// megabytes and people drop ten at once.
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "could not read the upload", http.StatusBadRequest)
			return
		}

		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			http.Error(w, "no files in the upload", http.StatusBadRequest)
			return
		}

		var assets []store.Asset
		for _, header := range files {
			f, err := header.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			asset, err := s.PutAsset(r.Context(), p, header.Filename, data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			// The bytes under an id can only change here, so this is the only
			// place the cached preview of one has to be dropped.
			s.Forget(asset.Doc.ID)
			assets = append(assets, asset)
		}

		// A named screen means "replace this one"; otherwise the files fill
		// the empty slots in order and then extend the set.
		if screenID := r.FormValue("screen"); screenID != "" && len(assets) > 0 {
			p, err = s.Edit(r.Context(), p.ID, func(p *model.Project) {
				// The composition, not the half: replacing the right tile of a
				// panorama replaces the picture both halves are cut from, and
				// writing it onto the half alone would be undone by the sync
				// that follows every edit.
				if screen, ok := p.Lead(screenID); ok {
					screen.AssetID = assets[0].Doc.ID
				}
			})
		} else {
			p, err = s.AddShots(r.Context(), p.ID, assets)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"id":    p.ID,
			"count": len(assets),
		})
	})
}

// icon serves the project's app icon. The bytes under an asset id never
// change — a new image is a new asset — so this can be cached hard and
// revalidated by id alone.
func icon(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Project(r.Context(), r.PathValue("id"))
		if err != nil || p.IconAssetID == "" {
			http.NotFound(w, r)
			return
		}
		asset, err := s.Asset(r.Context(), p.IconAssetID)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		tag := `"` + asset.Doc.ID + `"`
		w.Header().Set("ETag", tag)
		// Cached hard, because the bytes under an asset id cannot change — a
		// new icon is a new asset. The URL carries the id as ?v=, so a change
		// is a different URL rather than a revalidation.
		//
		// This is what stops the icon blinking on every step change: the rail
		// is re-rendered on each navigation, so the <img> is a new element
		// every time, and under no-cache the browser went back to the server
		// before it would paint.
		if r.URL.Query().Get("v") == asset.Doc.ID {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		http.ServeFile(w, r, s.Path(p, asset))
	})
}

// edit saves the project's name, description and icon in one request.
//
// Multipart rather than one of the typed JSON endpoints, because the icon is a
// file and the alternative — upload the image, then save the text — would
// re-render the page between the two and discard whatever the author had
// typed but not yet saved.
func edit(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Project(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "no such project", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "could not read the form", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "give the project a name", http.StatusBadRequest)
			return
		}

		// The icon is optional: an edit that only renames must not clear it.
		iconID := ""
		// A file input with nothing chosen still sends a part, with an empty
		// filename and no bytes. Without this guard, renaming a project tries
		// to decode zero bytes as an image and fails the whole save.
		//
		// The test is the bytes, not the name: the native window's open panel
		// does not always give a part a filename, and requiring one there
		// dropped the icon without saying anything.
		if file, header, ferr := r.FormFile("icon"); ferr == nil && header.Size > 0 {
			defer file.Close()
			data, rerr := io.ReadAll(file)
			if rerr != nil {
				http.Error(w, rerr.Error(), http.StatusBadRequest)
				return
			}
			name := header.Filename
			if name == "" {
				name = "icon"
			}
			asset, aerr := s.PutAsset(r.Context(), p, name, data)
			if aerr != nil {
				http.Error(w, aerr.Error(), http.StatusBadRequest)
				return
			}
			s.Forget(asset.Doc.ID)
			iconID = asset.Doc.ID
		}

		if _, err := s.Edit(r.Context(), p.ID, func(p *model.Project) {
			p.Name = name
			p.Description = strings.TrimSpace(r.FormValue("description"))
			if iconID != "" {
				p.IconAssetID = iconID
			}
			if r.FormValue("clear_icon") == "1" {
				p.IconAssetID = ""
			}
			applyLocales(p, r)
		}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": p.ID}) //nolint:errcheck
	})
}

// applyLocales reconciles the project's languages with the ticked boxes.
//
// It runs only when the form actually carried the field. A checkbox group
// sends nothing at all when every box is unticked, which is byte-identical to
// a request that was never about languages — so without the marker, any future
// caller posting a name change would silently delete every language but the
// base one.
func applyLocales(p *model.Project, r *http.Request) {
	if r.FormValue("locales_submitted") != "1" {
		return
	}
	want := r.Form["locales"]

	keep := map[string]bool{p.BaseLocale: true}
	for _, tag := range want {
		keep[tag] = true
	}
	// Removing first, and through the model, so the copy written in a dropped
	// language goes with it rather than lingering in the document.
	for _, tag := range slices.Clone(p.Locales) {
		if !keep[tag] {
			p.RemoveLocale(tag)
		}
	}
	// Added in the order the form sent them, which is the order the chips are
	// rendered — so the list stays stable instead of reshuffling on every save.
	for _, tag := range want {
		p.AddLocale(tag)
	}
}

func intParam(r *http.Request, name string, fallback int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return fallback
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
