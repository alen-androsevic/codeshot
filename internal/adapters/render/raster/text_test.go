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
		Background: domain.RGBA{0x00, 0x00, 0x00, 0xFF},
		Foreground: domain.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
	}
	th.Palette[1] = domain.RGBA{0xFF, 0x00, 0x00, 0xFF}
	th.Palette[4] = domain.RGBA{0x00, 0x00, 0xFF, 0xFF}
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

func TestImageIsSizedFromTheGrid(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	g := domain.Grid{Cols: 10, Lines: [][]domain.Cell{cells("hi", domain.Style{})}}
	img := render(t, bare(g, testTheme()))
	if got, want := img.Bounds().Dx(), 10*m.CellW; got != want {
		t.Errorf("width = %d, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 1*m.CellH; got != want {
		t.Errorf("height = %d, want %d", got, want)
	}
}

func TestBackgroundFillsTheImage(t *testing.T) {
	g := domain.Grid{Cols: 4, Lines: [][]domain.Cell{cells("    ", domain.Style{})}}
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
	// A wide rune owns two cells; the second must not be drawn over.
	wide := domain.Grid{Cols: 4, Lines: [][]domain.Cell{{
		{Rune: '日', Width: 2},
		{Width: 0},
		{Rune: ' ', Width: 1},
		{Rune: ' ', Width: 1},
	}}}
	if countNonBackground(render(t, bare(wide, testTheme()))) == 0 {
		t.Error("the wide rune was not drawn at all")
	}
}

func TestUnderlineAddsPixelsBelowTheBaseline(t *testing.T) {
	plain := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{})}}
	under := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{}.Set(domain.AttrUnderline))}}
	if countNonBackground(render(t, bare(under, testTheme()))) <= countNonBackground(render(t, bare(plain, testTheme()))) {
		t.Error("underline drew no extra pixels")
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
