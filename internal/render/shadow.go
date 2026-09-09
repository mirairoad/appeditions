package render

import (
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

// A device frame casts a shadow, and nothing in Go's drawing stack has one.
// The canvas API this port came from gave it away for free (`ctx.shadowBlur`),
// which is why the original could describe the whole frame in ninety lines.
//
// Here it is: fill the silhouette into a small alpha mask, blur the mask, and
// composite one colour through it. The mask covers the frame's bounding box
// plus the blur, not the whole canvas — at 2064×2752 with three devices that
// is the difference between three 400 kB masks and three 22 MB ones.

// shadowSpec is what a caller has to say about the shape casting the shadow.
// The shape itself is drawn by the callback so this code never has to know
// what a device looks like.
type shadowSpec struct {
	// bounds is the shape's axis-aligned extent before the blur is added.
	bounds image.Rectangle
	blur   float64
	offset float64
	col    color.RGBA
	// draw paints the silhouette into a context whose origin has been moved to
	// the mask's top-left corner.
	draw func(*gg.Context)
}

func drawShadow(dst *image.RGBA, spec shadowSpec) {
	radius := int(math.Ceil(spec.blur))
	if radius < 1 {
		return
	}
	// Three box passes approximate a Gaussian closely enough that no one can
	// tell, at a third of the cost of a real one.
	pad := radius * 3
	area := spec.bounds.Inset(-pad).Add(image.Pt(0, int(spec.offset)))
	area = area.Intersect(dst.Bounds().Inset(-pad))
	if area.Empty() {
		return
	}

	mask := gg.NewContext(area.Dx(), area.Dy())
	mask.Translate(float64(-area.Min.X), float64(-area.Min.Y+int(spec.offset)))
	mask.SetRGB(1, 1, 1)
	spec.draw(mask)

	alpha := image.NewAlpha(image.Rect(0, 0, area.Dx(), area.Dy()))
	src := mask.Image().(*image.RGBA)
	for i, n := 0, len(alpha.Pix); i < n; i++ {
		alpha.Pix[i] = src.Pix[i*4+3]
	}
	boxBlur(alpha, radius)

	// Composite: source-over of one flat colour, weighted by the blurred mask
	// and the colour's own alpha.
	clip := area.Intersect(dst.Bounds())
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		for x := clip.Min.X; x < clip.Max.X; x++ {
			a := float64(alpha.Pix[(y-area.Min.Y)*alpha.Stride+(x-area.Min.X)]) / 255 * float64(spec.col.A) / 255
			if a <= 0 {
				continue
			}
			o := dst.PixOffset(x, y)
			dst.Pix[o+0] = blend(dst.Pix[o+0], spec.col.R, a)
			dst.Pix[o+1] = blend(dst.Pix[o+1], spec.col.G, a)
			dst.Pix[o+2] = blend(dst.Pix[o+2], spec.col.B, a)
			dst.Pix[o+3] = blend(dst.Pix[o+3], 255, a)
		}
	}
}

func blend(dst, src uint8, a float64) uint8 {
	return uint8(float64(src)*a + float64(dst)*(1-a) + 0.5)
}

// boxBlur runs three box passes over the alpha channel in place. Separable:
// horizontal then vertical, so the cost is linear in the radius rather than
// quadratic.
func boxBlur(a *image.Alpha, radius int) {
	for range 3 {
		blurAxis(a, radius, true)
		blurAxis(a, radius, false)
	}
}

func blurAxis(a *image.Alpha, radius int, horizontal bool) {
	w, h := a.Rect.Dx(), a.Rect.Dy()
	outer, inner := h, w
	if !horizontal {
		outer, inner = w, h
	}
	line := make([]uint8, inner)
	window := radius*2 + 1
	for o := range outer {
		for i := range inner {
			if horizontal {
				line[i] = a.Pix[o*a.Stride+i]
			} else {
				line[i] = a.Pix[i*a.Stride+o]
			}
		}
		sum := 0
		for i := -radius; i <= radius; i++ {
			sum += int(line[clampInt(i, 0, inner-1)])
		}
		for i := range inner {
			v := uint8(sum / window)
			if horizontal {
				a.Pix[o*a.Stride+i] = v
			} else {
				a.Pix[i*a.Stride+o] = v
			}
			sum += int(line[clampInt(i+radius+1, 0, inner-1)]) - int(line[clampInt(i-radius, 0, inner-1)])
		}
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
