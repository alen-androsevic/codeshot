package domain

// Attr is a set of the appearance bits SGR can turn on for a cell.
type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrStrike
	AttrInverse
	AttrHidden
)

// Style is everything about a cell except which rune is in it.
type Style struct {
	FG, BG Color
	Attrs  Attr
}

func (s Style) Has(a Attr) bool { return s.Attrs&a != 0 }

// Set and Clear return a copy: a Style is a value, so the emulator can keep a
// current style and hand copies of it to thousands of cells without aliasing.
func (s Style) Set(a Attr) Style   { s.Attrs |= a; return s }
func (s Style) Clear(a Attr) Style { s.Attrs &^= a; return s }
