package render

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
)

// parseHex reads #rgb, #rrggbb and #rrggbbaa. An unreadable colour comes back
// opaque black rather than an error: a stored look with one bad swatch should
// draw wrong, not refuse to draw.
func parseHex(s string) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6, 8:
	default:
		return color.RGBA{A: 255}
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return color.RGBA{A: 255}
	}
	if len(s) == 6 {
		return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
	}
	return color.RGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}
}

func setHex(ctx *gg.Context, hex string) {
	c := parseHex(hex)
	ctx.SetRGBA255(int(c.R), int(c.G), int(c.B), int(c.A))
}
