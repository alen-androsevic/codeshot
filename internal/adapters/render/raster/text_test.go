package raster

import (
	"image"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

// bare is a window with nothing around the grid, so tests can address cells by
// arithmetic instead of hunting for them.
func bare(g domain.Grid, th domain.Theme) domain.Window {
	return domain.Window{
		Frame: domain.Frame{Grid: g},
		Theme: th,
		Chrome: domain.Chrome{
			Controls: domain.ControlsNone,
			Scale:    1,
		},
	}
}

func testTheme() domain.Theme {
	th := domain.Theme{
		Background: domain.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF},
		Foreground: domain.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	}
	th.Palette[1] = domain.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}
	th.Palette[4] = domain.RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF}
	return th
}

func cells(runes string, st domain.Style) []domain.Cell {
	out := make([]domain.Cell, 0, len(runes))
	for _, r := range runes {
		out = append(out, domain.Cell{Rune: r, Style: st, Width: 1})
	}
	return out
}

func render(t *testing.T, w domain.Window) *image.RGBA {
	t.Helper()
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	img, err := New(set, DefaultOptions()).Render(w)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return img.(*image.RGBA)
}

func TestImageIsSizedFromContent(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	// With MinCols=0, the window shrinks to fit the content. "hi" is 2 cells.
	g := domain.Grid{Cols: 10, Lines: [][]domain.Cell{cells("hi", domain.Style{})}}
	img := render(t, bare(g, testTheme()))
	if got, want := img.Bounds().Dx(), 2*m.CellW; got != want {
		t.Errorf("width = %d, want %d (content width)", got, want)
	}
	if got, want := img.Bounds().Dy(), 1*m.CellH; got != want {
		t.Errorf("height = %d, want %d", got, want)
	}
}

func TestMinColsFloorsTheWidth(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	// "hi" is 2 cells, but MinCols=5 floors the width at 5 columns.
	g := domain.Grid{Cols: 10, Lines: [][]domain.Cell{cells("hi", domain.Style{})}}
	w := bare(g, testTheme())
	w.Chrome.MinCols = 5
	img := render(t, w)
	if got, want := img.Bounds().Dx(), 5*m.CellW; got != want {
		t.Errorf("width = %d, want %d (MinCols)", got, want)
	}
}

func TestBackgroundFillsTheImage(t *testing.T) {
	// Use actual content so ContentCols > 0 and the window is sized.
	g := domain.Grid{Cols: 4, Lines: [][]domain.Cell{cells("test", domain.Style{})}}
	img := render(t, bare(g, testTheme()))
	if r, gg, b, a := img.At(1, 1).RGBA(); r != 0 || gg != 0 || b != 0 || a != 0xFFFF {
		t.Errorf("pixel = %d,%d,%d,%d, want opaque black from the theme", r, gg, b, a)
	}
}

func TestCellBackgroundIsPainted(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	st := domain.Style{BG: domain.IndexedColor(4)}
	g := domain.Grid{Cols: 2, Lines: [][]domain.Cell{{
		domain.Cell{Rune: ' ', Width: 1},
		domain.Cell{Rune: ' ', Style: st, Width: 1},
	}}}
	img := render(t, bare(g, testTheme()))
	if r, _, b, _ := img.At(m.CellW+1, 1).RGBA(); r != 0 || b != 0xFFFF {
		t.Errorf("second cell = %d,-,%d, want the palette's blue", r, b)
	}
	if _, _, b, _ := img.At(1, 1).RGBA(); b != 0 {
		t.Error("the blue background bled into the first cell")
	}
}

func TestGlyphsAreDrawn(t *testing.T) {
	blankGrid := domain.Grid{Cols: 3, Lines: [][]domain.Cell{cells("   ", domain.Style{})}}
	textGrid := domain.Grid{Cols: 3, Lines: [][]domain.Cell{cells("WWW", domain.Style{})}}
	blank := countNonBackground(render(t, bare(blankGrid, testTheme())))
	text := countNonBackground(render(t, bare(textGrid, testTheme())))
	if blank != 0 {
		t.Errorf("%d marks on a blank grid, want none", blank)
	}
	if text == 0 {
		t.Error("no marks drawn for WWW")
	}
}

func TestZeroWidthContinuationCellsDrawNoGlyph(t *testing.T) {
	// A wide rune owns two cells; the second must not be drawn over. The
	// continuation cell here carries a non-zero rune on purpose: a fixture
	// where it holds the zero value can't tell "skipped because Width==0"
	// apart from "skipped because the rune looked blank anyway". Only a
	// non-blank rune on a width-0 cell isolates the width check.
	wide := func(continuation domain.Cell) domain.Grid {
		return domain.Grid{Cols: 4, Lines: [][]domain.Cell{{
			{Rune: '日', Width: 2},
			continuation,
			{Rune: ' ', Width: 1},
			{Rune: ' ', Width: 1},
		}}}
	}
	blank := countNonBackground(render(t, bare(wide(domain.Cell{Width: 0}), testTheme())))
	if blank == 0 {
		t.Error("the wide rune was not drawn at all")
	}
	withRune := countNonBackground(render(t, bare(wide(domain.Cell{Rune: 'X', Width: 0}), testTheme())))
	if withRune != blank {
		t.Errorf("continuation cell carrying a rune drew %d marks, want %d (same as a blank continuation cell)", withRune, blank)
	}
}

func TestUnderlineAddsPixelsBelowTheBaseline(t *testing.T) {
	plain := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{})}}
	under := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{}.Set(domain.AttrUnderline))}}
	if countNonBackground(render(t, bare(under, testTheme()))) <= countNonBackground(render(t, bare(plain, testTheme()))) {
		t.Error("underline drew no extra pixels")
	}
}

func TestUnderlineSpansAWideRune(t *testing.T) {
	// The leading and continuation cells of a wide rune both carry the
	// underline attribute (vt/buffer.go copies the leading cell's Style onto
	// the continuation cell), so the rule under a wide rune must cover both
	// cells, not just the first. Cells are blank (space) rather than an
	// actual wide glyph so the count below is decoration pixels only, not
	// glyph antialiasing.
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	st := domain.Style{}.Set(domain.AttrUnderline)
	g := domain.Grid{Cols: 2, Lines: [][]domain.Cell{{
		{Rune: ' ', Style: st, Width: 2},
		{Style: st, Width: 0},
	}}}
	img := render(t, bare(g, testTheme()))
	y := m.Ascent + 2 // baseline + 2*scale, scale is 1 in bare()
	got := 0
	for x := 0; x < 2*m.CellW; x++ {
		if r, gg, b, _ := img.At(x, y).RGBA(); r|gg|b != 0 {
			got++
		}
	}
	if want := 2 * m.CellW; got != want {
		t.Errorf("underline pixels across the wide rune's row = %d, want %d (the full two-cell span)", got, want)
	}
}

func TestGlyphIsDrawnInItsOwnColumn(t *testing.T) {
	// Pins horizontal placement: a lone glyph flanked by blank columns must
	// leave marks only inside its own column's pixel range. A glyph drawn at
	// the wrong x-offset would otherwise go undetected by a pixel count alone.
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	g := domain.Grid{Cols: 3, Lines: [][]domain.Cell{{
		{Rune: ' ', Width: 1},
		{Rune: 'W', Width: 1},
		{Rune: ' ', Width: 1},
	}}}
	img := render(t, bare(g, testTheme()))
	found := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, gg, bb, _ := img.At(x, y).RGBA()
			if r|gg|bb == 0 {
				continue
			}
			found = true
			if x < m.CellW || x >= 2*m.CellW {
				t.Fatalf("mark at (%d,%d), want it confined to the middle column [%d,%d)", x, y, m.CellW, 2*m.CellW)
			}
		}
	}
	if !found {
		t.Fatal("no marks drawn at all")
	}
}

func countNonBackground(img *image.RGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bb, _ := img.At(x, y).RGBA(); r|g|bb != 0 {
				n++
			}
		}
	}
	return n
}

// TestBackgroundRunsCoverExactlyTheirCells pins the run-merging pass the
// design doc calls out by name. Neither golden fixture contained a coloured
// background at all before this, so the whole pass was uncovered.
//
// One honest caveat, established by mutation rather than assumed: removing
// the merging entirely and filling every cell on its own produces
// byte-identical output, because the fill is draw.Src with a uniform source
// and so has no anti-aliased edges to leave a seam between. So no test can
// tell merged fills from per-cell fills, and this one does not pretend to.
// What it does pin is the arithmetic the merging pass computes - where each
// run starts and stops - which is where an actual bug would live: a run that
// stops a cell early leaves an unpainted stripe, one that runs a cell long
// paints over its neighbour, and both are invisible until someone puts a
// coloured background next to another one.
func TestBackgroundRunsCoverExactlyTheirCells(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	red := domain.Style{BG: domain.IndexedColor(1)}
	blue := domain.Style{BG: domain.IndexedColor(4)}

	line := make([]domain.Cell, 10)
	for i := range line {
		line[i] = domain.Cell{Rune: ' ', Width: 1}
	}
	for i := 2; i < 5; i++ {
		line[i].Style = red
	}
	for i := 5; i < 7; i++ {
		line[i].Style = blue
	}
	img := render(t, bare(domain.Grid{Cols: 10, Lines: [][]domain.Cell{line}}, testTheme()))

	want := func(x int) (r, g, b uint32) {
		switch cell := x / m.CellW; {
		case cell >= 2 && cell < 5:
			return 0xFFFF, 0, 0
		case cell >= 5 && cell < 7:
			return 0, 0, 0xFFFF
		default:
			return 0, 0, 0 // the theme's own background
		}
	}
	for y := 0; y < m.CellH; y++ {
		for x := 0; x < 10*m.CellW; x++ {
			wr, wg, wb := want(x)
			r, g, b, _ := img.At(x, y).RGBA()
			if r != wr || g != wg || b != wb {
				t.Fatalf("pixel (%d,%d) in cell %d = %d,%d,%d, want %d,%d,%d: a background run does not line up with its cells",
					x, y, x/m.CellW, r, g, b, wr, wg, wb)
			}
		}
	}
}
