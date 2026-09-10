package domain

import "testing"

// gridOf builds a Grid from plain strings, one per line, all cells default.
func gridOf(cols int, lines ...string) Grid {
	g := Grid{Cols: cols}
	for _, s := range lines {
		row := make([]Cell, 0, cols)
		for _, r := range s {
			row = append(row, Cell{Rune: r, Width: 1})
		}
		for len(row) < cols {
			row = append(row, Cell{Rune: ' ', Width: 1})
		}
		g.Lines = append(g.Lines, row)
	}
	return g
}

func TestGridText(t *testing.T) {
	g := gridOf(6, "ab", "cd")
	if got, want := g.Text(), "ab\ncd\n"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTrimTrailingBlankRemovesEmptyTail(t *testing.T) {
	g := gridOf(4, "x", "", "")
	if got := g.TrimTrailingBlank().Rows(); got != 1 {
		t.Errorf("rows after trim = %d, want 1", got)
	}
}

func TestTrimTrailingBlankKeepsInteriorBlanks(t *testing.T) {
	g := gridOf(4, "x", "", "y", "")
	if got := g.TrimTrailingBlank().Rows(); got != 3 {
		t.Errorf("rows after trim = %d, want 3", got)
	}
}

func TestTrimTrailingBlankKeepsLinesColouredByBackground(t *testing.T) {
	g := gridOf(4, "x", "")
	g.Lines[1][0].Style.BG = IndexedColor(4)
	if got := g.TrimTrailingBlank().Rows(); got != 2 {
		t.Errorf("a background-painted blank line was trimmed; rows = %d, want 2", got)
	}
}

func TestHeadAndTailCrop(t *testing.T) {
	g := gridOf(4, "a", "b", "c", "d")
	if got := g.Head(2).Text(); got != "a\nb\n" {
		t.Errorf("Head(2) = %q", got)
	}
	if got := g.Tail(2).Text(); got != "c\nd\n" {
		t.Errorf("Tail(2) = %q", got)
	}
}

func TestHeadAndTailAreNoOpsWhenNotSmaller(t *testing.T) {
	g := gridOf(4, "a", "b")
	if g.Head(0).Rows() != 2 || g.Head(9).Rows() != 2 || g.Tail(0).Rows() != 2 || g.Tail(9).Rows() != 2 {
		t.Error("crop to 0 or to more rows than exist must return the grid unchanged")
	}
	// Verify exact boundary: n == len must be a no-op, not crop.
	if g.Head(2).Rows() != 2 || g.Tail(2).Rows() != 2 {
		t.Error("crop to exactly the grid size must return the grid unchanged")
	}
}

func TestJoinConcatenatesAndWidensToTheWidestGrid(t *testing.T) {
	j := Join(gridOf(4, "a"), gridOf(9, "bb"))
	if j.Cols != 9 {
		t.Errorf("Cols = %d, want 9", j.Cols)
	}
	if j.Rows() != 2 || j.Text() != "a\nbb\n" {
		t.Errorf("Join = %q with %d rows", j.Text(), j.Rows())
	}
}

func TestCellIsBlank(t *testing.T) {
	if !(Cell{Rune: ' ', Width: 1}).IsBlank() || !(Cell{}).IsBlank() {
		t.Error("space and zero cell must both count as blank")
	}
	if (Cell{Rune: 'x', Width: 1}).IsBlank() {
		t.Error("a rune is not blank")
	}
	painted := Cell{Rune: ' ', Width: 1}
	painted.Style.BG = IndexedColor(2)
	if painted.IsBlank() {
		t.Error("a space with a background colour is visible, so not blank")
	}
}
