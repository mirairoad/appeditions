package model

import (
	"fmt"
	"strconv"
	"strings"
)

// A Setting is one control's value as it comes off a form: a key and a string.
//
// The parsing lives in the vocabulary rather than in the endpoint layer because
// both sides run it. The browser applies a control change locally for an
// instant repaint and posts the same {key, value} to the server, which applies
// it again — and "a tilt is a float, a background is a little language" has to
// mean the same thing in both places or the two copies of the project drift.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s Setting) Validate() error {
	if s.Key == "" {
		return fmt.Errorf("key is required")
	}
	return nil
}

// ApplyTo writes the setting into a Settings value. Unknown keys are an error
// rather than a no-op: a typo that silently does nothing is a control that
// looks broken and cannot be debugged from the outside.
func (s Setting) ApplyTo(dst *Settings) error {
	switch s.Key {
	case "background":
		bg, err := ParseBackground(s.Value)
		if err != nil {
			return err
		}
		dst.Background = bg
	case "background_color":
		// The solid picker. It keeps the gradient's colours rather than
		// discarding them, so switching to gradient and back does not lose
		// what was chosen either way.
		dst.Background.Kind = "solid"
		dst.Background.Color = s.Value
	case "background_kind":
		// Solid or gradient, keeping every colour already picked. A kind
		// switch that also reset the colours would make the pair of controls
		// impossible to explore: you could never see what your gradient looked
		// like without retyping it.
		switch s.Value {
		case "gradient":
			dst.Background.Kind = "gradient"
			if dst.Background.From == "" {
				dst.Background.From = orHex(dst.Background.Color, "#eaf2ff")
			}
			if dst.Background.To == "" {
				dst.Background.To = "#ffffff"
			}
			if dst.Background.Angle == 0 {
				dst.Background.Angle = 160
			}
		default:
			dst.Background.Kind = "solid"
			if dst.Background.Color == "" {
				dst.Background.Color = orHex(dst.Background.From, "#eaf2ff")
			}
		}
	case "gradient_from":
		dst.Background.From = s.Value
	case "gradient_to":
		dst.Background.To = s.Value
	case "gradient_angle":
		v, err := strconv.ParseFloat(s.Value, 64)
		if err != nil {
			return fmt.Errorf("the gradient angle must be a number")
		}
		dst.Background.Angle = v
	case "backdrop_color":
		dst.BackdropColor = s.Value
	case "device_id":
		dst.DeviceID = s.Value
	case "frame_color_id":
		dst.FrameColorID = s.Value
	case "position_id":
		dst.PositionID = s.Value
	case "layout":
		dst.Layout = s.Value
	case "size_id":
		dst.SizeID = s.Value
	case "text_color":
		dst.TextColor = s.Value
	case "accent_color":
		dst.AccentColor = s.Value
	case "text_align":
		dst.TextAlign = s.Value
	case "font_id":
		dst.FontID = s.Value
	case "highlights":
		dst.Highlights = splitColors(s.Value)
	case "tilt", "device_scale", "headline_scale", "subhead_scale", "headline_tracking":
		v, err := strconv.ParseFloat(s.Value, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number", s.Key)
		}
		switch s.Key {
		case "tilt":
			dst.Tilt = v
		case "device_scale":
			dst.DeviceScale = v
		case "headline_scale":
			dst.HeadlineScale = v
		case "subhead_scale":
			dst.SubheadScale = v
		case "headline_tracking":
			dst.HeadlineTracking = v
		}
	default:
		// The marker colours are one control per colour, numbered, so the
		// list can grow without the client sending the whole array back.
		if index, ok := highlightIndex(s.Key); ok {
			for len(dst.Highlights) <= index {
				dst.Highlights = append(dst.Highlights, "#ffe27a")
			}
			dst.Highlights[index] = s.Value
			return nil
		}
		return fmt.Errorf("unknown setting %q", s.Key)
	}
	return nil
}

// OverrideKey is the name [Overrides.Clear] knows this control by. Most keys
// are the same on both sides; the exceptions are the two controls that write a
// value the model stores under a different name — the colour picker behind
// "background", and each individual marker swatch, which all belong to the one
// highlights list.
func (s Setting) OverrideKey() string {
	switch s.Key {
	case "background_color", "background_kind", "gradient_from", "gradient_to", "gradient_angle":
		return "background"
	}
	if _, ok := highlightIndex(s.Key); ok {
		return "highlights"
	}
	return s.Key
}

// Pin writes the setting into a screen's overrides. Separate from ApplyTo
// because the two mean different things: this one pins an exception, and a
// value written here stops following the set until it is cleared.
func (s Setting) Pin(o *Overrides, base Settings) error {
	// Resolve against the screen's current effective settings, so a control
	// that only carries one number (a highlight colour, say) pins the whole
	// list it belongs to and not an empty one.
	resolved := o.Apply(base)
	if err := s.ApplyTo(&resolved); err != nil {
		return err
	}

	switch s.Key {
	case "background", "background_color", "background_kind",
		"gradient_from", "gradient_to", "gradient_angle":
		o.Background = &resolved.Background
	case "backdrop_color":
		o.BackdropColor = &resolved.BackdropColor
	case "device_id":
		o.DeviceID = &resolved.DeviceID
	case "frame_color_id":
		o.FrameColorID = &resolved.FrameColorID
	case "position_id":
		o.PositionID = &resolved.PositionID
	case "layout":
		o.Layout = &resolved.Layout
	case "text_color":
		o.TextColor = &resolved.TextColor
	case "accent_color":
		o.AccentColor = &resolved.AccentColor
	case "text_align":
		o.TextAlign = &resolved.TextAlign
	case "font_id":
		o.FontID = &resolved.FontID
	case "tilt":
		o.Tilt = &resolved.Tilt
	case "device_scale":
		o.DeviceScale = &resolved.DeviceScale
	case "headline_scale":
		o.HeadlineScale = &resolved.HeadlineScale
	case "subhead_scale":
		o.SubheadScale = &resolved.SubheadScale
	case "headline_tracking":
		o.HeadlineTracking = &resolved.HeadlineTracking
	case "size_id":
		// Deliberately not overridable: a set has one canvas size, and a
		// screen that disagreed would export at a size the store rejects.
		return fmt.Errorf("the export size is set for the whole project")
	default:
		if _, ok := highlightIndex(s.Key); ok || s.Key == "highlights" {
			highlights := resolved.Highlights
			o.Highlights = &highlights
			return nil
		}
		return fmt.Errorf("unknown setting %q", s.Key)
	}
	return nil
}

func highlightIndex(key string) (int, bool) {
	rest, ok := strings.CutPrefix(key, "highlight_")
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	return i, err == nil
}

func splitColors(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseBackground reads the little language the swatches post:
// `solid:#eaf2ff` or `gradient:#6366f1:#a855f7:135`. A background is two
// shapes in one field, and encoding it in the value is what keeps the client
// from needing a model of it.
func ParseBackground(value string) (Background, error) {
	parts := strings.Split(value, ":")
	switch {
	case len(parts) == 2 && parts[0] == "solid":
		return Solid(parts[1]), nil
	case len(parts) == 4 && parts[0] == "gradient":
		angle, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return Background{}, fmt.Errorf("gradient angle must be a number")
		}
		return Gradient(parts[1], parts[2], angle), nil
	case strings.HasPrefix(value, "#"):
		return Solid(value), nil
	}
	return Background{}, fmt.Errorf("unreadable background %q", value)
}

// orHex is a hex colour or a fallback, for the moment a background changes
// kind and the field the new kind reads has never been set.
func orHex(v, fallback string) string {
	if strings.HasPrefix(v, "#") {
		return v
	}
	return fallback
}
