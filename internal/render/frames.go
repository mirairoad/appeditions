package render

import (
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"

	"github.com/mirairoad/appeditions/internal/model"
)

// A Box is an axis-aligned rectangle in canvas pixels. Device frames are laid
// out as boxes and then rotated about their own centre, so the box is always
// the un-rotated bounds.
type Box struct{ X, Y, W, H float64 }

func (b Box) center() (float64, float64) { return b.X + b.W/2, b.Y + b.H/2 }

func (b Box) rect() image.Rectangle {
	return image.Rect(int(math.Floor(b.X)), int(math.Floor(b.Y)), int(math.Ceil(b.X+b.W)), int(math.Ceil(b.Y+b.H)))
}

// roundedRect adds a rounded rectangle to the path, clamping the radius so a
// small frame cannot produce a shape with overlapping corners.
func roundedRect(ctx *gg.Context, b Box, r float64) {
	r = math.Max(0, math.Min(r, math.Min(b.W/2, b.H/2)))
	ctx.DrawRoundedRectangle(b.X, b.Y, b.W, b.H, r)
}

// drawCoverTop fits a screenshot into the screen area the way a phone shows
// it: scaled to cover, anchored to the top. Anchoring to the centre would crop
// the status bar, which is the one part of a capture people notice missing.
func drawCoverTop(ctx *gg.Context, img image.Image, b Box) {
	ib := img.Bounds()
	iw, ih := float64(ib.Dx()), float64(ib.Dy())
	if iw == 0 || ih == 0 {
		return
	}
	scale := math.Max(b.W/iw, b.H/ih)
	dw, dh := iw*scale, ih*scale
	x, y := b.X+(b.W-dw)/2, b.Y

	// Resample to the destination size first. gg draws images through
	// draw.BiLinear, which is fine for a small correction and visibly wrong for
	// the 4× downscale a 1290-wide capture needs to reach a 300px preview:
	// bilinear samples four pixels and drops the rest, so thin UI lines shimmer
	// or vanish. CatmullRom weighs the whole footprint.
	dst := image.NewRGBA(image.Rect(0, 0, int(math.Ceil(dw)), int(math.Ceil(dh))))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, ib, xdraw.Src, nil)

	// The integer part goes to DrawImage and the fraction to the matrix, so a
	// device that lands on a half pixel is not nudged onto a whole one.
	ctx.Push()
	ctx.Translate(x-math.Floor(x), y-math.Floor(y))
	ctx.DrawImage(dst, int(math.Floor(x)), int(math.Floor(y)))
	ctx.Pop()
}

// drawNotch paints the camera cutout. It is drawn inside the screen clip, over
// the screenshot, because that is where the real one is.
func drawNotch(ctx *gg.Context, screen Box, d model.Device, frameW float64) {
	switch d.Notch {
	case model.NotchIsland:
		w := frameW * 0.3
		h := frameW * 0.085
		ctx.SetRGB255(8, 8, 10)
		roundedRect(ctx, Box{X: screen.X + (screen.W-w)/2, Y: screen.Y + frameW*0.028, W: w, H: h}, h/2)
		ctx.Fill()
	case model.NotchPunch:
		r := frameW * 0.026
		ctx.SetRGB255(8, 8, 10)
		ctx.DrawCircle(screen.X+screen.W/2, screen.Y+frameW*0.055, r)
		ctx.Fill()
	}
}

// DrawDevice draws one device frame with a screenshot inside it, rotated by
// angle degrees about its own centre. box is the outer frame bounds and the
// caller is responsible for having fitted it to the frame's aspect ratio.
func DrawDevice(ctx *gg.Context, dst *image.RGBA, box Box, angle float64, d model.Device, col model.FrameColor, img image.Image) {
	outerR := d.Radius * box.W
	bezel := d.Bezel * box.W
	screen := Box{X: box.X + bezel, Y: box.Y + bezel, W: box.W - bezel*2, H: box.H - bezel*2}
	screenR := math.Max(0, outerR-bezel)
	cx, cy := box.center()
	rad := angle * math.Pi / 180

	// The shadow is the silhouette of the outer frame, blurred and pushed down
	// — the same numbers the canvas version used, which were tuned by eye
	// against a real App Store listing.
	drawShadow(dst, shadowSpec{
		bounds: rotatedBounds(box, rad),
		blur:   box.W * 0.045,
		offset: box.W * 0.035,
		col:    color.RGBA{R: 15, G: 23, B: 42, A: 77},
		draw: func(m *gg.Context) {
			m.RotateAbout(rad, cx, cy)
			roundedRect(m, box, outerR)
			m.Fill()
		},
	})

	ctx.Push()
	if angle != 0 {
		ctx.RotateAbout(rad, cx, cy)
	}

	if bezel > 0 {
		setHexColor(ctx, col.Body)
	} else {
		// Frameless: there is no body to fill, but something has to sit under a
		// screenshot with rounded corners or the corners show the background
		// through the shadow.
		ctx.SetRGB(1, 1, 1)
	}
	roundedRect(ctx, box, outerR)
	ctx.Fill()

	if bezel > 0 {
		lw := math.Max(1, box.W*0.005)
		ctx.SetLineWidth(lw)
		setHexColor(ctx, col.Edge)
		roundedRect(ctx, Box{X: box.X + lw/2, Y: box.Y + lw/2, W: box.W - lw, H: box.H - lw}, outerR)
		ctx.Stroke()
	}

	roundedRect(ctx, screen, screenR)
	ctx.Clip()
	ctx.SetRGB(1, 1, 1)
	ctx.DrawRectangle(screen.X, screen.Y, screen.W, screen.H)
	ctx.Fill()
	if img != nil {
		drawCoverTop(ctx, img, screen)
	}
	drawNotch(ctx, screen, d, box.W)
	ctx.ResetClip()
	ctx.Pop()
}

func setHexColor(ctx *gg.Context, hex string) { setHex(ctx, hex) }

// rotatedBounds is the axis-aligned extent of a box turned about its centre.
// Used to size the shadow mask; a tilted phone needs a wider one than its own
// box.
func rotatedBounds(b Box, rad float64) image.Rectangle {
	if rad == 0 {
		return b.rect()
	}
	cx, cy := b.center()
	sin, cos := math.Abs(math.Sin(rad)), math.Abs(math.Cos(rad))
	w := b.W*cos + b.H*sin
	h := b.W*sin + b.H*cos
	return Box{X: cx - w/2, Y: cy - h/2, W: w, H: h}.rect()
}
