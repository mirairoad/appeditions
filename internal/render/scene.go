package render

import (
	"image"
	"math"

	"github.com/fogleman/gg"

	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/internal/presets"
)

// Sources are the screenshots a composition may reach for, keyed by
// model.SourceSelf / SourceNext / SourcePrev. A multi-device arrangement shows
// its neighbours, which is what makes a strip read as a strip.
type Sources map[string]image.Image

func (s Sources) at(key string) image.Image {
	if img, ok := s[key]; ok && img != nil {
		return img
	}
	// Falling back to the current screenshot keeps a one-screen project
	// drawing every frame of a trio instead of two empty slots.
	return s[model.SourceSelf]
}

// DrawBackground fills the whole composition. Gradient angles follow CSS: 180
// runs top to bottom, 135 from the top-left corner.
func DrawBackground(ctx *gg.Context, w, h float64, bg model.Background) {
	if bg.Kind != "gradient" {
		setHex(ctx, bg.Color)
		ctx.DrawRectangle(0, 0, w, h)
		ctx.Fill()
		return
	}
	rad := (bg.Angle - 90) * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	length := math.Abs(w*dx) + math.Abs(h*dy)
	g := gg.NewLinearGradient(
		w/2-dx*length/2, h/2-dy*length/2,
		w/2+dx*length/2, h/2+dy*length/2,
	)
	g.AddColorStop(0, parseHex(bg.From))
	g.AddColorStop(1, parseHex(bg.To))
	ctx.SetFillStyle(g)
	ctx.DrawRectangle(0, 0, w, h)
	ctx.Fill()
}

// drawBackdrop paints the rounded card behind the device band. It sits a
// little above the device and, when the layout bleeds the device off the
// bottom, runs off-canvas too — otherwise its bottom corners appear in the
// middle of a composition that is supposed to run out of the frame.
func drawBackdrop(ctx *gg.Context, W, tileW, h float64, layout model.Layout, hex string) {
	r := tileW * 0.075
	top := h*layout.Device.Top - h*0.05
	bottom := h + r
	if layout.Device.Bottom <= 1 {
		bottom = math.Min(h+r, h*layout.Device.Bottom+h*0.05)
	}
	setHex(ctx, hex)
	ctx.DrawRoundedRectangle(tileW*0.045, top, W-tileW*0.09, bottom-top, r)
	ctx.Fill()
}

// A DevicePlacement is one frame of a composition: where it goes, which
// screenshot fills it, and how far it is turned.
type DevicePlacement struct {
	Box    Box
	Source string
	Angle  float64
}

// ComposeDevices is where every device frame of a composition sits, back to
// front. w and h are one store tile.
//
// It is the single source of device geometry: the renderer draws these boxes
// and the arrangement pickers sketch them, so a picker can never show a
// composition the export does not produce.
func ComposeDevices(layout model.Layout, positionID string, w, h, aspect, deviceScale, tilt float64) []DevicePlacement {
	W := w * float64(layout.Span)
	slot := Box{
		X: w * layout.PadX,
		Y: h * layout.Device.Top,
		W: W - w*layout.PadX*2,
		H: h * (layout.Device.Bottom - layout.Device.Top),
	}

	// Fit one frame inside the slot preserving aspect — or take the layout's
	// fixed width — and scale each placement from there.
	fitW := math.Min(slot.W, slot.H*aspect)
	if layout.Device.Width != nil {
		fitW = w * *layout.Device.Width
	}
	baseW := fitW * deviceScale

	cx := slot.X + slot.W/2
	if layout.Device.Cx != nil {
		cx = W * *layout.Device.Cx
	}
	cy := slot.Y + slot.H/2
	if layout.Device.Cy != nil {
		cy = h * *layout.Device.Cy
	}

	position := presets.PositionOf(positionID)
	out := make([]DevicePlacement, 0, len(position.Placements))
	for _, p := range position.Placements {
		fw := baseW * p.Scale
		fh := fw / aspect
		out = append(out, DevicePlacement{
			Box:    Box{X: cx + p.Dx*W - fw/2, Y: cy + p.Dy*h - fh/2, W: fw, H: fh},
			Source: p.Source,
			Angle:  p.Rotate + tilt,
		})
	}
	return out
}

// Span and ShowsText answer questions about a composition rather than draw
// one, and the browser needs them too — so they live in presets, which
// compiles for wasm. These are here because the renderer's callers already say
// render.Span, and moving a name is not worth a diff across six files.
func Span(screen model.Screen, settings model.Settings) int {
	return presets.Span(screen, settings)
}

func ShowsText(screen model.Screen, settings model.Settings) bool {
	return presets.ShowsText(screen, settings)
}

// Scene draws one screen. w and h are the store *tile* size; a span-2 layout
// draws a composition 2w wide, so the caller sizes the image with [Span].
//
// This is the only renderer. The preview asks for a 300px tile and the export
// asks for 1320×2868, and nothing else differs between them — which is what
// makes the preview a promise rather than an impression.
func Scene(w, h int, screen model.Screen, settings model.Settings, copy model.Copy, sources Sources) *image.RGBA {
	// Inheritance is resolved here and nowhere else. Two places would
	// eventually disagree, and the disagreement would be invisible until an
	// exported PNG came out different from the preview it was approved from.
	s := screen.Overrides.Apply(settings)
	layout := presets.LayoutOf(s.Layout)

	fw, fh := float64(w), float64(h)
	W := fw * float64(layout.Span)

	img := image.NewRGBA(image.Rect(0, 0, int(W), h))
	ctx := gg.NewContextForRGBA(img)

	DrawBackground(ctx, W, fh, s.Background)
	if s.BackdropColor != "" {
		drawBackdrop(ctx, W, fw, fh, layout, s.BackdropColor)
	}
	DrawTextBlock(ctx, img, W, fw, fh, layout, copy, s)

	device := presets.Device(s.DeviceID)
	frame := presets.Frame(s.FrameColorID)
	for _, p := range ComposeDevices(layout, s.PositionID, fw, fh, device.FrameAspect(), s.DeviceScale, s.Tilt) {
		DrawDevice(ctx, img, p.Box, p.Angle, device, frame, sources.at(p.Source))
	}
	return img
}
