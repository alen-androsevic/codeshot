// Package domain holds codeshot's pure core: the grid a terminal produced, the
// theme it is painted with, and the window it is framed in. Nothing here opens
// a file, loads a font or talks to a terminal, which is what makes the whole
// bytes-to-image pipeline testable without one.
package domain

// ColorKind says how a Color must be resolved against a Theme.
type ColorKind uint8

const (
	// ColorDefault is the terminal's own foreground or background, whatever
	// the theme says those are.
	ColorDefault ColorKind = iota
	// ColorIndexed is a palette entry. 0-15 come from the theme; 16-255 are
	// fixed by the xterm cube and ignore it.
	ColorIndexed
	// ColorRGB is a literal 24-bit colour the program asked for by name, so
	// the theme has no say in it at all.
	ColorRGB
)

// Color is a reference to a colour, not a colour. It stays unresolved until a
// Theme is applied, which is what lets one capture be rendered in any theme.
type Color struct {
	Kind    ColorKind
	Index   uint8
	R, G, B uint8
}

func DefaultColor() Color          { return Color{Kind: ColorDefault} }
func IndexedColor(i uint8) Color   { return Color{Kind: ColorIndexed, Index: i} }
func RGBColor(r, g, b uint8) Color { return Color{Kind: ColorRGB, R: r, G: g, B: b} }
