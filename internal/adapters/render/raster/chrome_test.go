package raster

import (
	"image"
	"image/color"
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

// TestLinuxControlsAtImageEdgeDoNotPanic pins a real crash a code review
// found: with no margin, no padding, and a one-column grid, the Linux dots'
// unclamped bounding rectangle in fillCircle reached past the image, and
// vector.Rasterizer.Draw indexes dst.Pix from that rectangle's corner
// without checking it first - "runtime error: slice bounds out of range
// [-48:]" against this exact configuration before the fix.
func TestLinuxControlsAtImageEdgeDoNotPanic(t *testing.T) {
	g := domain.Grid{Cols: 1, Lines: [][]domain.Cell{cells("x", domain.Style{})}}
	render(t, chromed(g, testTheme(), func(c *domain.Chrome) {
		c.Controls = domain.ControlsLinux
		c.Margin = 0
		c.Padding = 0
	}))
}

// TestFillCircleEntirelyOffImageDoesNotPanic covers the primitive directly,
// independent of any chrome layout: a circle whose bounding box never
// touches dst at all must clip away to nothing rather than reach for pixels
// that don't exist.
func TestFillCircleEntirelyOffImageDoesNotPanic(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillCircle(dst, -1000, -1000, 6, color.RGBA{0xFF, 0, 0, 0xFF})
	fillCircle(dst, 1000, 1000, 6, color.RGBA{0xFF, 0, 0, 0xFF})
	fillCircle(dst, 5, 5, 100, color.RGBA{0xFF, 0, 0, 0xFF})
}

// TestClampingDoesNotShiftInBoundsDrawing guards against a clamp that
// "fixes" the panic by silently moving the drawing instead of cropping it:
// a fully in-bounds window (this is the same fixture and pixel positions
// TestTrafficLightsArePaintedInTheTitlebar already checks) must still put
// the traffic lights exactly where an unclamped fillCircle always did. This
// case never actually clips (clip == r, offset zero), so it is a coarse
// regression guard; TestFillCirclePartiallyClippedKeepsCorrectPosition below
// is the one that exercises a non-zero clip offset.
func TestClampingDoesNotShiftInBoundsDrawing(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	r, g, b, _ := img.At(20, 14).RGBA()
	if !(r > 0xC000 && g < 0x9000 && b < 0x9000) {
		t.Errorf("close button = %d,%d,%d, want the red traffic light at its usual position", r, g, b)
	}
	r, g, b, _ = img.At(60, 14).RGBA()
	if !(g > 0x9000 && r < 0x9000) {
		t.Errorf("zoom button = %d,%d,%d, want the green traffic light at its usual position", r, g, b)
	}
}

// TestFillCirclePartiallyClippedKeepsCorrectPosition draws the same circle
// twice: once on a canvas large enough to hold it whole (so fillCircle's
// internal clip is a no-op, ox/oy stay 0) and once on a canvas whose bounds
// chop off the circle's left side (so clip.Min != r.Min and the path must be
// shifted to compensate). The visible region must be pixel-identical between
// the two - proof the clip crops rather than translates the drawing, which a
// panic-only test cannot distinguish from a subtly wrong offset. That
// comparison alone would miss a *uniform* offset bug (one applied the same
// way regardless of clipping, e.g. an extra "+4" baked into both ox and oy):
// it shifts "whole" and "chopped" together, so they would still agree with
// each other while both being wrong. assertCircleShape below closes that
// gap by pinning the circle to fillCircle's actual cx/cy/radius contract.
func TestFillCirclePartiallyClippedKeepsCorrectPosition(t *testing.T) {
	cx, cy, radius := 2.0, 10.0, 8.0
	col := color.RGBA{0xFF, 0, 0, 0xFF}

	whole := image.NewRGBA(image.Rect(-20, 0, 20, 20))
	fillCircle(whole, cx, cy, radius, col)

	chopped := image.NewRGBA(image.Rect(0, 0, 20, 20))
	fillCircle(chopped, cx, cy, radius, col)

	assertSamePosition(t, whole, chopped, image.Rect(0, 0, 20, 20))
	assertCircleShape(t, chopped, cx, cy, radius, col)
}

// assertCircleShape checks that (approximately) exactly the disk of radius
// around (cx,cy) is painted col, by sampling just inside and just outside
// the boundary in each of the four cardinal directions (skipping any sample
// that falls outside img's own bounds). A single centre-point check cannot
// tell a correctly-placed circle from one shifted by a few pixels - points
// near the centre stay inside a slightly shifted circle too - but a small
// shift moves boundary points across the inside/outside line, which is what
// this samples for.
func assertCircleShape(t *testing.T, img *image.RGBA, cx, cy, radius float64, col color.RGBA) {
	t.Helper()
	b := img.Bounds()
	check := func(x, y int, wantInside bool) {
		p := image.Pt(x, y)
		if !p.In(b) {
			return
		}
		isCol := img.RGBAAt(x, y) == col
		if isCol != wantInside {
			t.Errorf("(%d,%d) painted=%v, want painted=%v (circle at (%.0f,%.0f) r=%.0f is misplaced)", x, y, isCol, wantInside, cx, cy, radius)
		}
	}
	for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		// +-1 sits inside the anti-aliasing fringe (the four-Bezier
		// approximation isn't a perfect circle either), so pixels there can
		// come back partially transparent even when correctly placed; +-2
		// clears that fringe.
		check(int(cx+d[0]*(radius-2)), int(cy+d[1]*(radius-2)), true)
		check(int(cx+d[0]*(radius+2)), int(cy+d[1]*(radius+2)), false)
	}
}

// TestFillRoundRectEntirelyOffImageDoesNotPanic is fillCircle's off-image
// panic test, for the sibling primitive: Task 13's shadow calls
// fillRoundRect with the window rect offset downward, which the same
// unclamped-Draw defect would panic on.
func TestFillRoundRectEntirelyOffImageDoesNotPanic(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillRoundRect(dst, image.Rect(-1000, -1000, -900, -900), 4, color.RGBA{0, 0, 0xFF, 0xFF})
	fillRoundRect(dst, image.Rect(900, 900, 1000, 1000), 4, color.RGBA{0, 0, 0xFF, 0xFF})
}

// TestFillRoundRectPartiallyClippedKeepsCorrectPosition is
// TestFillCirclePartiallyClippedKeepsCorrectPosition's sibling for
// fillRoundRect. The rect (5,5,50,50) starts inside dst's (0,0,40,40) bounds
// but runs off its right and bottom edges: unlike a rect that's simply
// narrower than dst - where the rasteriser's own mask buffer, sized to the
// rect's natural width, happens to end exactly where the correct edge does,
// silently absorbing a clip-independent offset bug - here dst's own edge is
// what bounds the buffer, so a shift is not coincidentally re-clipped away;
// it visibly moves the left/top edges, which this can watch fall inside
// dst's bounds.
func TestFillRoundRectPartiallyClippedKeepsCorrectPosition(t *testing.T) {
	rect := image.Rect(5, 5, 50, 50)
	radius := 6.0
	col := color.RGBA{0x11, 0x22, 0x33, 0xFF}

	whole := image.NewRGBA(image.Rect(-10, -10, 60, 60))
	fillRoundRect(whole, rect, radius, col)

	chopped := image.NewRGBA(image.Rect(0, 0, 40, 40))
	fillRoundRect(chopped, rect, radius, col)

	assertSamePosition(t, whole, chopped, image.Rect(0, 0, 40, 40))
	assertRoundRectShape(t, chopped, rect, col)
}

// assertRoundRectShape samples just inside and just outside the left and
// top edges, on the mid-line of the opposite axis so corner rounding
// doesn't complicate the inside/outside call, skipping samples outside
// img's own bounds. Like assertCircleShape, this catches a shift a single
// deep-interior point would not: a rect shifted a few pixels still reads as
// "filled" well away from its edges.
func assertRoundRectShape(t *testing.T, img *image.RGBA, r image.Rectangle, col color.RGBA) {
	t.Helper()
	b := img.Bounds()
	midX, midY := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	check := func(x, y int, wantInside bool) {
		p := image.Pt(x, y)
		if !p.In(b) {
			return
		}
		isCol := img.RGBAAt(x, y) == col
		if isCol != wantInside {
			t.Errorf("(%d,%d) painted=%v, want painted=%v (rect %v is misplaced)", x, y, isCol, wantInside, r)
		}
	}
	check(r.Min.X+2, midY, true)
	check(r.Min.X-2, midY, false)
	check(midX, r.Min.Y+2, true)
	check(midX, r.Min.Y-2, false)
}

// assertSamePosition compares two renders of the same shape - one on a
// canvas large enough to hold it whole, one on a canvas that clips it - over
// the region they both cover. The rasteriser's anti-aliasing math runs
// against a different-sized internal buffer in each case (20x20 vs 12x20,
// say), so a handful of edge pixels legitimately round to +-1/255 apart;
// that is floating-point accumulation order, not a positional error. An
// actual wrong-sign offset bug moves the whole shape by whole pixels and
// produces large, widespread differences, which this tolerance does not
// hide.
func assertSamePosition(t *testing.T, whole, chopped *image.RGBA, region image.Rectangle) {
	t.Helper()
	const tolerance = 2
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			w, c := whole.RGBAAt(x, y), chopped.RGBAAt(x, y)
			d := func(a, b uint8) int {
				if a > b {
					return int(a - b)
				}
				return int(b - a)
			}
			if diff := d(w.R, c.R) + d(w.G, c.G) + d(w.B, c.B) + d(w.A, c.A); diff > tolerance {
				t.Fatalf("pixel (%d,%d) = %v, want %v (clip shifted the drawing instead of cropping it)", x, y, c, w)
			}
		}
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
