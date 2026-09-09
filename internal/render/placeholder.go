package render

import (
	"image"
	"sync"

	"github.com/fogleman/gg"
)

// Placeholder is a stand-in app screenshot, drawn rather than shipped as a
// bitmap, for template thumbnails and for slots the author has not filled yet.
// Seeing the whole look before any screenshot exists is the point: a template
// picker showing empty rectangles tells you nothing about the template.
var (
	placeholderOnce sync.Once
	placeholder     image.Image
)

func Placeholder() image.Image {
	placeholderOnce.Do(func() { placeholder = drawPlaceholder() })
	return placeholder
}

func drawPlaceholder() image.Image {
	const w, h = 1320.0, 2868.0
	ctx := gg.NewContext(int(w), int(h))

	regular := Face("inter", false, w*0.036)
	medium := Face("inter", false, w*0.042)
	bold := Face("inter", true, w*0.075)
	small := Face("inter", true, w*0.038)
	dst := ctx.Image().(*image.RGBA)

	setHex(ctx, "#fbfaf6")
	ctx.Clear()

	ink := parseHex("#1b1b18")
	drawString(dst, small, w*0.085, h*0.02, "9:41", ink, 0, 1)
	for i, bw := range []float64{0.012, 0.012, 0.03} {
		setHex(ctx, "#1b1b18")
		ctx.DrawRectangle(w*(0.92-float64(i)*0.035)-bw*w, h*0.024, bw*w, h*0.008)
		ctx.Fill()
	}

	drawString(dst, bold, w*0.085, h*0.085, "Grocery list", ink, 0, 1)
	drawString(dst, regular, w*0.085, h*0.132, "Saturday · 8 items", parseHex("#8a8a82"), 0, 1)

	setHex(ctx, "#efece3")
	ctx.DrawRoundedRectangle(w*0.085, h*0.175, w*0.83, h*0.05, w*0.03)
	ctx.Fill()
	drawString(dst, regular, w*0.14, h*0.19, "Search", parseHex("#a5a49b"), 0, 1)

	rows := []struct {
		label string
		done  bool
	}{
		{"Bananas", true}, {"Oat milk", true}, {"Sourdough", false}, {"Eggs", false},
		{"Tomatoes", false}, {"Basil", false}, {"Olive oil", false}, {"Coffee beans", false},
	}
	for i, row := range rows {
		y := h * (0.27 + float64(i)*0.062)
		box := Box{X: w * 0.085, Y: y - w*0.024, W: w * 0.048, H: w * 0.048}
		setHex(ctx, "#c9c7bd")
		ctx.SetLineWidth(w * 0.004)
		roundedRect(ctx, box, w*0.01)
		ctx.Stroke()
		if row.done {
			setHex(ctx, "#1b1b18")
			roundedRect(ctx, box, w*0.01)
			ctx.Fill()
		}
		col := ink
		if row.done {
			col = parseHex("#a5a49b")
		}
		drawString(dst, medium, w*0.17, y-w*0.03, row.label, col, 0, 1)
		setHex(ctx, "#ecebe4")
		ctx.DrawRectangle(w*0.085, y+h*0.028, w*0.83, h*0.0012)
		ctx.Fill()
	}

	setHex(ctx, "#1b1b18")
	ctx.DrawCircle(w*0.85, h*0.9, w*0.07)
	ctx.Fill()
	setHex(ctx, "#fbfaf6")
	ctx.SetLineWidth(w * 0.008)
	ctx.DrawLine(w*0.85-w*0.03, h*0.9, w*0.85+w*0.03, h*0.9)
	ctx.DrawLine(w*0.85, h*0.9-w*0.03, w*0.85, h*0.9+w*0.03)
	ctx.Stroke()

	return ctx.Image()
}
