// Package vt is codeshot's terminal emulator: bytes in, a grid of styled cells
// out. It implements only what command-line programs actually emit - the list
// is in the design doc - and silently ignores the rest, because a shot of the
// output is not a terminal anyone will type into.
package vt

import "codeshot/internal/domain"

// buffer is one screen: a window of rows lines over a line store. The normal
// buffer lets that store grow, which is how scrollback survives; the alternate
// buffer keeps it exactly rows long, because a program that took the whole
// screen has no history worth showing.
type buffer struct {
	cols, rows int
	lines      [][]domain.Cell
	origin     int  // index in lines of the top visible row
	x, y       int  // cursor; y is relative to origin
	wrapNext   bool // the cursor is parked past the last column
	scrollback bool
}

func newBuffer(cols, rows int, scrollback bool) *buffer {
	b := &buffer{cols: cols, rows: rows, scrollback: scrollback}
	for i := 0; i < rows; i++ {
		b.lines = append(b.lines, b.blankLine(domain.Style{}))
	}
	return b
}

func (b *buffer) blankLine(st domain.Style) []domain.Cell {
	line := make([]domain.Cell, b.cols)
	for i := range line {
		line[i] = blank(st)
	}
	return line
}

// blank keeps only the background: an erased cell shows the colour that was
// current when it was erased, but not its underline or its bold.
func blank(st domain.Style) domain.Cell {
	return domain.Cell{Rune: ' ', Width: 1, Style: domain.Style{BG: st.BG}}
}

func (b *buffer) row(y int) []domain.Cell { return b.lines[b.origin+y] }

func (b *buffer) put(r rune, w int, st domain.Style) {
	if w == 0 {
		b.combine(r)
		return
	}
	if b.wrapNext {
		b.carriageReturn()
		b.lineFeed()
	}
	if w == 2 && b.x == b.cols-1 {
		// A double-width rune is never split across the edge; the terminal
		// leaves the last column blank and starts it on the next line.
		b.row(b.y)[b.x] = blank(st)
		b.carriageReturn()
		b.lineFeed()
	}
	line := b.row(b.y)
	line[b.x] = domain.Cell{Rune: r, Style: st, Width: uint8(w)}
	if w == 2 {
		line[b.x+1] = domain.Cell{Width: 0, Style: st}
	}
	b.x += w
	if b.x >= b.cols {
		b.x = b.cols - 1
		b.wrapNext = true
	}
}

// combine hangs a zero-width mark off whatever was printed last. Decomposed
// text - which is how macOS hands over filenames - depends on this.
func (b *buffer) combine(r rune) {
	x := b.x
	if !b.wrapNext {
		x--
	}
	if x < 0 {
		return
	}
	line := b.row(b.y)
	for x > 0 && line[x].Width == 0 {
		x--
	}
	line[x].Combining += string(r)
}

func (b *buffer) carriageReturn() {
	b.x = 0
	b.wrapNext = false
}

func (b *buffer) lineFeed() {
	b.wrapNext = false
	if b.y < b.rows-1 {
		b.y++
		return
	}
	if b.scrollback {
		b.origin++
		b.lines = append(b.lines, b.blankLine(domain.Style{}))
		return
	}
	copy(b.lines, b.lines[1:])
	b.lines[b.rows-1] = b.blankLine(domain.Style{})
}

func (b *buffer) backspace() {
	b.wrapNext = false
	if b.x > 0 {
		b.x--
	}
}

func (b *buffer) tab() {
	b.wrapNext = false
	next := (b.x/8 + 1) * 8
	if next > b.cols-1 {
		next = b.cols - 1
	}
	b.x = next
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (b *buffer) moveTo(x, y int) {
	b.x = clamp(x, 0, b.cols-1)
	b.y = clamp(y, 0, b.rows-1)
	b.wrapNext = false
}

func (b *buffer) move(dx, dy int) { b.moveTo(b.x+dx, b.y+dy) }

func (b *buffer) eraseInLine(mode int, st domain.Style) {
	line := b.row(b.y)
	from, to := 0, b.cols
	switch mode {
	case 0:
		from = b.x
	case 1:
		to = b.x + 1
	}
	for i := from; i < to && i < len(line); i++ {
		line[i] = blank(st)
	}
	b.wrapNext = false
}

func (b *buffer) eraseInDisplay(mode int, st domain.Style) {
	switch mode {
	case 0:
		b.eraseInLine(0, st)
		for y := b.y + 1; y < b.rows; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
	case 1:
		for y := 0; y < b.y; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
		b.eraseInLine(1, st)
	default:
		for y := 0; y < b.rows; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
	}
	b.wrapNext = false
}

// scrollUp pushes lines off the top of the screen. On the normal buffer they
// land in scrollback, exactly as a line feed at the bottom would leave them.
func (b *buffer) scrollUp(n int) {
	for i := 0; i < n; i++ {
		if b.scrollback {
			b.origin++
			b.lines = append(b.lines, b.blankLine(domain.Style{}))
			continue
		}
		copy(b.lines, b.lines[1:])
		b.lines[b.rows-1] = b.blankLine(domain.Style{})
	}
}

func (b *buffer) scrollDown(n int) {
	for i := 0; i < n; i++ {
		top := b.origin
		copy(b.lines[top+1:top+b.rows], b.lines[top:top+b.rows-1])
		b.lines[top] = b.blankLine(domain.Style{})
	}
}

func (b *buffer) grid() domain.Grid {
	end := len(b.lines)
	if !b.scrollback {
		end = b.origin + b.rows
	}
	g := domain.Grid{Cols: b.cols}
	g.Lines = append(g.Lines, b.lines[:end]...)
	return g
}
