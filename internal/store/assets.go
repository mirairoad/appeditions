package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"

	"github.com/mirairoad/howl-go/db"

	"github.com/mirairoad/appeditions/internal/model"
)

// Path is where an asset's bytes live. Named after the row id: the file and
// the document cannot drift apart, and two projects with the same capture keep
// their own copies rather than sharing one that either could delete.
func (s *Store) Path(p model.Project, a Asset) string {
	return filepath.Join(s.AssetsDir(p), a.Doc.ID+a.Ext)
}

// PutAsset stores an original screenshot. The bytes are never modified — every
// transformation happens at render time — so this is a hash, a decode for the
// dimensions, and a write.
func (s *Store) PutAsset(ctx context.Context, p model.Project, name string, data []byte) (Asset, error) {
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	// The same capture dropped twice is the same asset. It happens constantly
	// — a re-export from the simulator into a folder that is then re-dropped —
	// and a second four-megabyte copy helps nobody.
	existing, err := s.Assets.One(ctx, db.Query{Where: db.And(db.Eq("project_id", p.ID), db.Eq("sha", sha))})
	if err == nil {
		if _, statErr := os.Stat(s.Path(p, existing)); statErr == nil {
			return existing, nil
		}
		// The row outlived its file. Fall through and write it again under the
		// same id rather than leaving a project pointing at nothing.
	} else if !errors.Is(err, db.ErrNotFound) {
		return Asset{}, err
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Asset{}, fmt.Errorf("%s is not an image this can read: %w", name, err)
	}

	asset := Asset{
		ProjectID: p.ID,
		Name:      name,
		Ext:       "." + format,
		SHA:       sha,
		Width:     cfg.Width,
		Height:    cfg.Height,
		Bytes:     int64(len(data)),
	}
	if existing.Doc.ID != "" {
		asset = existing
	} else {
		if asset, err = s.Assets.Create(ctx, asset); err != nil {
			return Asset{}, err
		}
	}

	if err := os.MkdirAll(s.AssetsDir(p), 0o755); err != nil {
		return Asset{}, err
	}
	if err := os.WriteFile(s.Path(p, asset), data, 0o644); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

// AssetsOf lists a project's originals, newest first.
func (s *Store) AssetsOf(ctx context.Context, projectID string) ([]Asset, error) {
	return s.Assets.Find(ctx, db.Query{
		Where: db.Eq("project_id", projectID),
		Sort:  db.Desc("meta.created_at"),
	})
}

// Title turns a filename into the headline a fresh screen starts with.
// `02-search-results.png` becomes "02 search results" — not good copy, but
// better than a blank line, and it tells the author which screen is which.
func (a Asset) Title() string {
	name := strings.TrimSuffix(a.Name, filepath.Ext(a.Name))
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	return strings.TrimSpace(name)
}

// previewWidth is the widest a cached preview variant gets.
//
// It has to cover the largest device frame the editor draws, at the 2x the
// tiles are rendered at: the tune page draws a 420px tile, so 840px of frame,
// and a source cached below that is upscaled into the frame and looks soft
// however sharp the render around it is. 1024 covers it with room, and costs
// about 9 MB per cached screenshot rather than the 2 MB 512 cost — worth it
// for the one thing the author is actually judging.
const previewWidth = 1024

// imageCache holds one downscaled copy of each asset. Full-resolution decodes
// are deliberately not cached: they are needed once per export, and ten of
// them resident would be a quarter of a gigabyte to save a hundred
// milliseconds on a button nobody presses twice in a row.
type imageCache struct {
	mu sync.Mutex
	m  map[string]image.Image
}

type byteCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

// iconWidth is the size an app icon is served at.
//
// What is stored is whatever the author had, which for an app icon is the
// 1024px one the stores ask for; the interface draws it at 36px in the
// sidebar. Decoding a megabyte of PNG to paint a 36px chip costs a frame — and
// a navigation replaces #outlet, so that <img> is a new element and that frame
// is paid again on every step change, which is exactly what the icon blinking
// was. 128 covers the largest place it is drawn (64px, at 2x).
const iconWidth = 128

// Icon is the project's icon at a size worth decoding, encoded once and kept.
//
// Cached as bytes rather than as an image, because the caller writes it to a
// response: re-encoding a PNG per request would move the cost from the browser
// to here rather than removing it. One entry per project, and the bytes under
// an asset id never change — a new icon is a new asset.
func (s *Store) Icon(p model.Project, a Asset) ([]byte, error) {
	s.icons.mu.Lock()
	if b, ok := s.icons.m[a.Doc.ID]; ok {
		s.icons.mu.Unlock()
		return b, nil
	}
	s.icons.mu.Unlock()

	full, err := s.decode(s.Path(p, a))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, downscale(full, iconWidth)); err != nil {
		return nil, err
	}

	s.icons.mu.Lock()
	defer s.icons.mu.Unlock()
	if s.icons.m == nil {
		s.icons.m = map[string][]byte{}
	}
	s.icons.m[a.Doc.ID] = buf.Bytes()
	return buf.Bytes(), nil
}

// Source loads an asset sized for a render targeting targetW pixels of frame
// width. Small targets get the cached preview; an export gets the original.
func (s *Store) Source(p model.Project, a Asset, targetW int) (image.Image, error) {
	if targetW > previewWidth {
		return s.decode(s.Path(p, a))
	}

	s.images.mu.Lock()
	if img, ok := s.images.m[a.Doc.ID]; ok {
		s.images.mu.Unlock()
		return img, nil
	}
	s.images.mu.Unlock()

	full, err := s.decode(s.Path(p, a))
	if err != nil {
		return nil, err
	}
	small := downscale(full, previewWidth)

	s.images.mu.Lock()
	defer s.images.mu.Unlock()
	if s.images.m == nil {
		s.images.m = map[string]image.Image{}
	}
	s.images.m[a.Doc.ID] = small
	return small, nil
}

// Forget drops an asset's cached preview. Called when the file behind it is
// replaced, which is the only way the bytes under an id can change.
func (s *Store) Forget(assetID string) {
	s.images.mu.Lock()
	delete(s.images.m, assetID)
	s.images.mu.Unlock()

	s.icons.mu.Lock()
	defer s.icons.mu.Unlock()
	delete(s.icons.m, assetID)
}

func (s *Store) decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

func downscale(src image.Image, width int) image.Image {
	b := src.Bounds()
	if b.Dx() <= width {
		return src
	}
	h := b.Dy() * width / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, width, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// RemoveAsset deletes an original and the file behind it.
//
// Refused while a screen still points at it: the set would keep a slot whose
// picture no longer exists, and every preview of it would fall back to the
// placeholder with nothing on screen saying why. The Media panel says which
// ones are in use, so this is the last line rather than the first.
//
// The row goes before the file. A row with no file renders a placeholder and
// can be deleted again; a file with no row is invisible to the application and
// stays on disk forever.
func (s *Store) RemoveAsset(ctx context.Context, projectID, assetID string) error {
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return err
	}
	asset, err := s.Asset(ctx, assetID)
	if err != nil {
		return err
	}
	if asset.ProjectID != p.ID {
		return errors.New("that picture belongs to another project")
	}
	// The icon is an ordinary asset, which is easy to forget: it is uploaded
	// through a different form and never appears in a set, so nothing else
	// here would have stopped it going.
	if p.IconAssetID == assetID {
		return fmt.Errorf("%s is the project's icon — change it in project settings", asset.Name)
	}
	// Every language, not just the base one: a picture used only as the German
	// capture of one screen is as much in use as the base one beside it.
	for _, v := range p.Versions {
		for _, screen := range v.Screens {
			if screen.AssetID == assetID {
				return fmt.Errorf("%s is still used by %s — remove the screen first", asset.Name, v.Name)
			}
			for locale, id := range screen.Shots {
				if id == assetID {
					return fmt.Errorf("%s is still used by %s in %s — replace it there first", asset.Name, v.Name, locale)
				}
			}
		}
	}

	if err := s.Assets.Delete(ctx, assetID); err != nil {
		return err
	}
	if err := os.Remove(s.Path(p, asset)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
