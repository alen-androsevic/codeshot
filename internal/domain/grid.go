package domain

import "strings"

// Cell is one terminal cell. Width is 1 for an ordinary rune, 2 for the
// leading half of a wide one, and 0 for its trailing half, which holds no
// glyph and is skipped when drawing. Combining holds any marks that hang off
// the rune - a string rather than a slice so that Cell stays comparable.
type Cell struct {
	Rune      rune
	Combining string
	Style     Style
	Width     uint8
}

// IsBlank reports whether the cell would leave no mark. A space with a
// background colour is not blank: terminals paint it, so codeshot must too.
// Neither is a space carrying a combining mark - the renderer draws the mark,
// and a cell the renderer would draw must never be one the domain discards.
// Underline and strikethrough also leave marks on otherwise blank cells.
func (c Cell) IsBlank() bool {
	if c.Rune != 0 && c.Rune != ' ' {
		return false
	}
	if c.Combining != "" {
		return false
	}
	if c.Style.BG.Kind != ColorDefault {
		return false
	}
	decorations := AttrInverse | AttrUnderline | AttrStrike
	return !c.Style.Has(decorations)
}

// Grid is a rectangular block of cells: what an emulator produced, or a piece
// of it. Lines may be shorter than Cols; missing cells read as blank.
type Grid struct {
	Cols  int
	Lines [][]Cell
}

func (g Grid) Rows() int { return len(g.Lines) }

// ContentCols returns the width of the widest line after trimming trailing
// blank cells. A blank cell is one with no glyph, no combining marks, and no
// visual decoration (background, inverse). This may be less than Cols when
// lines have trailing whitespace.
func (g Grid) ContentCols() int {
	max := 0
	for _, line := range g.Lines {
		// Find the last non-blank cell.
		end := len(line)
		for end > 0 && line[end-1].IsBlank() {
			end--
		}
		// Count column positions up to that point.
		w := 0
		for i := 0; i < end; i++ {
			if line[i].Width > 0 {
				w += int(line[i].Width)
			}
		}
		if w > max {
			max = w
		}
	}
	return max
}

// TrimTrailingBlank drops empty lines from the bottom. A capture almost always
// ends with the newline the program printed last, and that newline would
// otherwise become an empty row at the foot of the image.
func (g Grid) TrimTrailingBlank() Grid {
	end := len(g.Lines)
	for end > 0 && lineIsBlank(g.Lines[end-1]) {
		end--
	}
	g.Lines = g.Lines[:end]
	return g
}

func lineIsBlank(line []Cell) bool {
	for _, c := range line {
		if !c.IsBlank() {
			return false
		}
	}
	return true
}

// Head keeps the first n lines, Tail the last n. Both are no-ops for n <= 0 or
// n >= the number of lines, so callers can pass an unset option straight in.
func (g Grid) Head(n int) Grid {
	if n <= 0 || n >= len(g.Lines) {
		return g
	}
	g.Lines = g.Lines[:n]
	return g
}

func (g Grid) Tail(n int) Grid {
	if n <= 0 || n >= len(g.Lines) {
		return g
	}
	g.Lines = g.Lines[len(g.Lines)-n:]
	return g
}

// Text renders the grid as plain text, trailing blanks stripped per line. It
// exists for tests and for --debug; nothing in the pipeline consumes it.
func (g Grid) Text() string {
	var b strings.Builder
	for _, line := range g.Lines {
		var row strings.Builder
		for _, c := range line {
			if c.Width == 0 {
				continue
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			row.WriteRune(r)
			row.WriteString(c.Combining)
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// Join stacks grids vertically, taking the widest column count. It is how the
// prompt header gets glued on top of a command's output.
func Join(grids ...Grid) Grid {
	out := Grid{}
	for _, g := range grids {
		if g.Cols > out.Cols {
			out.Cols = g.Cols
		}
		out.Lines = append(out.Lines, g.Lines...)
	}
	return out
}
