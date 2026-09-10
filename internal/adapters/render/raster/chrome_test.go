package raster

import (
	"image"
	"testing"

	"codeshot/internal/domain"
)

func chromed(g domain.Grid, th domain.Theme, mut func(*domain.Chrome)) domain.Window {
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Shadow = false
	c.Margin = 0
	c.Title = "paradajz danas"
	if mut != nil {
		mut(&c)
	}
	return domain.Window{Frame: domain.Frame{Grid: g}, Theme: th, Chrome: c}
}

func wideGrid() domain.Grid {
	return domain.Grid{Cols: 30, Lines: [][]domain.Cell{cells("hello", domain.Style{})}}
}

func TestCornersAreTransparent(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("top-left corner alpha = %d, want 0: the window is rounded", a)
	}
	b := img.Bounds()
	if _, _, _, a := img.At(b.Max.X-1, b.Max.Y-1).RGBA(); a != 0 {
		t.Error("bottom-right corner is opaque; all four corners are rounded")
	}
}

func TestWindowCentreIsOpaque(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	b := img.Bounds()
	if _, _, _, a := img.At(b.Dx()/2, b.Dy()/2).RGBA(); a != 0xFFFF {
		t.Error("the middle of the window is not opaque")
	}
}

func TestTrafficLightsArePaintedInTheTitlebar(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	// The close button's centre: x=20, y=titlebar/2, at scale 1.
	r, g, b, _ := img.At(20, 14).RGBA()
	if !(r > 0xC000 && g < 0x9000 && b < 0x9000) {
		t.Errorf("close button = %d,%d,%d, want the red traffic light", r, g, b)
	}
	r, g, b, _ = img.At(60, 14).RGBA()
	if !(g > 0x9000 && r < 0x9000) {
		t.Errorf("zoom button = %d,%d,%d, want the green traffic light", r, g, b)
	}
}

func TestControlsNoneDrawsNoButtons(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), func(c *domain.Chrome) {
		c.Controls = domain.ControlsNone
		c.ShowTitle = false
	}))
	if r, g, b, _ := img.At(20, 14).RGBA(); r > 0xC000 && g < 0x9000 && b < 0x9000 {
		t.Error("a red traffic light was drawn although controls are off")
	}
}

func TestTitleIsDrawnAndCentred(t *testing.T) {
	with := render(t, chromed(wideGrid(), testTheme(), nil))
	without := render(t, chromed(wideGrid(), testTheme(), func(c *domain.Chrome) { c.Title = "" }))
	if titlebarMarks(with) <= titlebarMarks(without) {
		t.Error("the title drew no pixels")
	}
}

// titlebarMarks counts pixels in the middle third of the titlebar, away from
// the traffic lights on the left.
func titlebarMarks(img *image.RGBA) int {
	b := img.Bounds()
	n := 0
	for y := 0; y < 28; y++ {
		for x := b.Dx() / 3; x < 2*b.Dx()/3; x++ {
			if r, g, bb, _ := img.At(x, y).RGBA(); r|g|bb != 0 {
				n++
			}
		}
	}
	return n
}
