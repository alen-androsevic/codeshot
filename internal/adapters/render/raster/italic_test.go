package raster

import (
	"image"
	"os"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

// regularOnly is the embedded family with only its regular cut, which is what
// most monospace families on a machine look like: one weight, no italic. It
// is the only way to reach the synthetic italic path, since the embedded set
// itself has a real italic cut and never needs shearing.
func regularOnly(t *testing.T) *fonts.Set {
	t.Helper()
	const path = "../../fonts/assets/JetBrainsMonoNL-Regular.ttf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("embedded font asset not readable from here: %v", err)
	}
	// Slot 0 is the regular cut; the other three are left empty on purpose.
	set, err := fonts.Load(fonts.Family{
		Name:  "Regular Only",
		Files: [4]fonts.FontFile{0: {Path: path}},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return set
}

func oneGlyph(r rune, st domain.Style) domain.Window {
	chrome := domain.DefaultChrome()
	chrome.Scale = 1
	chrome.Shadow = false
	chrome.Margin = 0
	chrome.ShowTitle = false
	chrome.Controls = domain.ControlsNone
	chrome.PaddingX, chrome.PaddingY = 6, 6
	return domain.Window{
		Frame: domain.Frame{Grid: domain.Grid{
			Cols:  1,
			Lines: [][]domain.Cell{{{Rune: r, Width: 1, Style: st}}},
		}},
		Chrome: chrome,
		Theme: domain.Theme{
			Background: domain.RGBA{A: 0xFF},
			Foreground: domain.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		},
	}
}

// inkTopAndBottom is the leftmost inked pixel on the highest and lowest rows
// that have any ink at all.
func inkTopAndBottom(t *testing.T, img image.Image) (top, bottom int) {
	t.Helper()
	top, bottom = -1, -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r > 0x4000 && g > 0x4000 && bl > 0x4000 {
				if top == -1 {
					top = x
				}
				bottom = x
				break
			}
		}
	}
	if top == -1 {
		t.Fatal("nothing was drawn")
	}
	return top, bottom
}

// TestSyntheticItalicLeans covers design §7's 12 degree shear, which was
// unreachable until a family without an italic cut could be loaded. A sheared
// glyph's top leans right of its foot; an upright one does not.
func TestSyntheticItalicLeans(t *testing.T) {
	set := regularOnly(t)
	if !set.SyntheticItalic(domain.Style{Attrs: domain.AttrItalic}) {
		t.Fatal("a family with no italic cut did not ask for shearing")
	}
	r := New(set, DefaultOptions())

	upright, err := r.Render(oneGlyph('H', domain.Style{}))
	if err != nil {
		t.Fatal(err)
	}
	slanted, err := r.Render(oneGlyph('H', domain.Style{Attrs: domain.AttrItalic}))
	if err != nil {
		t.Fatal(err)
	}

	uTop, uBottom := inkTopAndBottom(t, upright)
	sTop, sBottom := inkTopAndBottom(t, slanted)

	if uTop != uBottom {
		t.Errorf("upright H leans: top ink at x=%d, bottom at x=%d", uTop, uBottom)
	}
	if sTop <= sBottom {
		t.Errorf("sheared H does not lean: top ink at x=%d, bottom at x=%d, want the top to the right", sTop, sBottom)
	}
	if sTop <= uTop {
		t.Errorf("sheared top ink at x=%d, upright at x=%d; the shear moved nothing", sTop, uTop)
	}
}

// TestSyntheticItalicStaysInItsColumn: the lean must not push a glyph out of
// the picture, which is what the scratch image's slack is for.
func TestSyntheticItalicStaysInItsColumn(t *testing.T) {
	set := regularOnly(t)
	r := New(set, DefaultOptions())
	img, err := r.Render(oneGlyph('H', domain.Style{Attrs: domain.AttrItalic}))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for _, x := range []int{b.Min.X, b.Max.X - 1} {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if cr > 0x4000 && cg > 0x4000 && cb > 0x4000 {
				t.Fatalf("ink at the image edge (%d,%d); the shear escaped the window", x, y)
			}
		}
	}
}
