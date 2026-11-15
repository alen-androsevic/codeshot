package domain

// RGBA is a resolved colour, ready for the rasteriser. Nothing upstream of
// Theme.Resolve is allowed to hold one.
type RGBA struct {
	R, G, B, A uint8
}

// Theme is a terminal colour scheme in the shape Ghostty's theme files use.
type Theme struct {
	Name       string
	Background RGBA
	Foreground RGBA
	Cursor     RGBA
	Palette    [16]RGBA
}

// Resolve turns a palette-relative Style into the two concrete colours a cell
// is painted with. The order of the attribute rules is the order terminals
// apply them: dim blends first, then inverse swaps what dim produced, then
// hidden collapses whatever is left.
func (t Theme) Resolve(s Style) (fg, bg RGBA) {
	fg = t.resolveColor(s.FG, t.Foreground)
	bg = t.resolveColor(s.BG, t.Background)
	if s.Has(AttrDim) {
		fg = blend(fg, bg, 0.5)
	}
	if s.Has(AttrInverse) {
		fg, bg = bg, fg
	}
	if s.Has(AttrHidden) {
		fg = bg
	}
	return fg, bg
}

func (t Theme) resolveColor(c Color, def RGBA) RGBA {
	switch c.Kind {
	case ColorIndexed:
		if c.Index < 16 {
			return t.Palette[c.Index]
		}
		return XTerm256(c.Index)
	case ColorRGB:
		return RGBA{c.R, c.G, c.B, 0xFF}
	default:
		return def
	}
}

// xtermBase is the fixed low half of the 256-colour table. A Theme overrides
// these through its Palette; XTerm256 answers for callers who have no theme.
var xtermBase = [16]RGBA{
	{0x00, 0x00, 0x00, 0xFF}, {0x80, 0x00, 0x00, 0xFF},
	{0x00, 0x80, 0x00, 0xFF}, {0x80, 0x80, 0x00, 0xFF},
	{0x00, 0x00, 0x80, 0xFF}, {0x80, 0x00, 0x80, 0xFF},
	{0x00, 0x80, 0x80, 0xFF}, {0xC0, 0xC0, 0xC0, 0xFF},
	{0x80, 0x80, 0x80, 0xFF}, {0xFF, 0x00, 0x00, 0xFF},
	{0x00, 0xFF, 0x00, 0xFF}, {0xFF, 0xFF, 0x00, 0xFF},
	{0x00, 0x00, 0xFF, 0xFF}, {0xFF, 0x00, 0xFF, 0xFF},
	{0x00, 0xFF, 0xFF, 0xFF}, {0xFF, 0xFF, 0xFF, 0xFF},
}

// XTerm256 resolves any 256-colour index: sixteen base colours, then a
// 6x6x6 cube, then a 24-step grey ramp.
func XTerm256(i uint8) RGBA {
	switch {
	case i < 16:
		return xtermBase[i]
	case i < 232:
		n := int(i) - 16
		level := [6]uint8{0x00, 0x5F, 0x87, 0xAF, 0xD7, 0xFF}
		return RGBA{level[n/36], level[(n/6)%6], level[n%6], 0xFF}
	default:
		v := uint8(8 + 10*(int(i)-232))
		return RGBA{v, v, v, 0xFF}
	}
}

func blend(a, b RGBA, f float64) RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-f) + float64(y)*f + 0.5)
	}
	return RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), a.A}
}
