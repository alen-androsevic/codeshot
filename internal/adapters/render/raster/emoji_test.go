package raster

import (
	"image"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

// emojiRenderer is a renderer whose font set can reach the machine's colour
// bitmap fonts. Without one there is nothing to test but tofu.
func emojiRenderer(t *testing.T) Renderer {
	t.Helper()
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	set = set.WithIndex(fonts.LoadIndex("", fonts.SystemFontDirs()))
	if _, ok := set.ColorGlyph('🎉', 32); !ok {
		t.Skip("no colour emoji font is installed here")
	}
	return New(set, DefaultOptions())
}

// wideGlyph is one rune in a cell run two columns wide, which is what the
// emulator produces for an emoji.
func wideGlyph(r rune) domain.Window {
	w := oneGlyph(r, domain.Style{})
	w.Frame.Grid = domain.Grid{
		Cols: 2,
		Lines: [][]domain.Cell{{
			{Rune: r, Width: 2},
			{Rune: 0, Width: 0},
		}},
	}
	return w
}

// TestEmojiIsDrawnInColour is 3.5 seen from the picture: the rune that has
// been a documented tofu limitation since phase 1 arrives with its colours.
func TestEmojiIsDrawnInColour(t *testing.T) {
	r := emojiRenderer(t)

	img, err := r.Render(wideGlyph('🎉'))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	colours := distinctColours(img)
	if colours < 8 {
		t.Errorf("the picture holds %d colours, want an emoji's worth", colours)
	}

	// A letter through the same path stays two colours: ink and background,
	// give or take the anti-aliasing, and nothing like an emoji's spread.
	plain, err := r.Render(wideGlyph('H'))
	if err != nil {
		t.Fatal(err)
	}
	if distinctColours(plain) >= colours {
		t.Errorf("'H' drew %d colours and 🎉 drew %d; the bitmap path did not run",
			distinctColours(plain), colours)
	}
}

// TestEmojiStaysInsideItsCells: an emoji is square and its cell run is not,
// so it is fitted to the smaller of the two. Overflow would land on the text
// beside it.
//
// The check is for emoji ink in the padding, not for any pixel at the image
// edge: the window's rounded corners are anti-aliased, so partial alpha along
// the edge is the corner being drawn correctly rather than anything escaping.
func TestEmojiStaysInsideItsCells(t *testing.T) {
	r := emojiRenderer(t)
	window := wideGlyph('🎉')
	img, err := r.Render(window)
	if err != nil {
		t.Fatal(err)
	}
	background := window.Theme.Background
	b := img.Bounds()
	// The padding is 6 either side; the rows are taken from the middle so
	// that the corner arcs are nowhere near them.
	for y := b.Min.Y + 12; y < b.Max.Y-12; y++ {
		for x := b.Min.X + 1; x < b.Min.X+6; x++ {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			if ca == 0 {
				continue
			}
			if uint8(cr>>8) != background.R || uint8(cg>>8) != background.G || uint8(cb>>8) != background.B {
				t.Fatalf("emoji ink at (%d,%d) in the padding, left of the first cell", x, y)
			}
		}
	}
}

func distinctColours(img image.Image) int {
	seen := map[uint64]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[uint64(r)<<48|uint64(g)<<32|uint64(bl)<<16|uint64(a)] = true
		}
	}
	return len(seen)
}
