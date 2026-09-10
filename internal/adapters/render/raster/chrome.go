package raster

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"codeshot/internal/domain"
)

// The macOS traffic lights, each with the slightly darker ring the real ones
// have. Without the ring they read as flat stickers.
var (
	closeFill = color.RGBA{0xFF, 0x5F, 0x57, 0xFF}
	closeRing = color.RGBA{0xE0, 0x44, 0x3E, 0xFF}
	minFill   = color.RGBA{0xFE, 0xBC, 0x2E, 0xFF}
	minRing   = color.RGBA{0xDE, 0xA1, 0x23, 0xFF}
	zoomFill  = color.RGBA{0x28, 0xC8, 0x40, 0xFF}
	zoomRing  = color.RGBA{0x1A, 0xAB, 0x29, 0xFF}
	linuxDot  = color.RGBA{0x6B, 0x70, 0x7B, 0xFF}
)

// drawChrome paints the window body - rounded, in the terminal's own
// background colour, titlebar included, which is what Ghostty's transparent
// titlebar style looks like - and then its buttons and title.
func (r Renderer) drawChrome(img *image.RGBA, l layout, w domain.Window) error {
	fillRoundRect(img, l.window, float64(w.Chrome.Radius*l.scale), rgba(w.Theme.Background))
	if l.titlebar == 0 {
		return nil
	}
	mid := float64(l.window.Min.Y + l.titlebar/2)
	radius := 6 * float64(l.scale)
	switch w.Chrome.Controls {
	case domain.ControlsMacOS:
		for i, pair := range [3][2]color.RGBA{{closeFill, closeRing}, {minFill, minRing}, {zoomFill, zoomRing}} {
			cx := float64(l.window.Min.X + (20+20*i)*l.scale)
			fillCircle(img, cx, mid, radius, pair[1])
			fillCircle(img, cx, mid, radius-float64(l.scale)*0.75, pair[0])
		}
	case domain.ControlsLinux:
		for i := 0; i < 3; i++ {
			cx := float64(l.window.Max.X - (20+20*(2-i))*l.scale)
			fillCircle(img, cx, mid, radius, linuxDot)
		}
	}
	return r.drawTitle(img, l, w)
}

func (r Renderer) drawTitle(img *image.RGBA, l layout, w domain.Window) error {
	if !w.Chrome.ShowTitle || w.Chrome.Title == "" {
		return nil
	}
	face, err := r.fonts.Face(domain.Style{}, 11*float64(l.scale))
	if err != nil {
		return err
	}
	width := font.MeasureString(face, w.Chrome.Title)
	x := l.window.Min.X + (l.window.Dx()-width.Round())/2
	// A title at full foreground strength shouts over the output it labels.
	fg, bg := w.Theme.Foreground, w.Theme.Background
	dim := color.RGBA{
		uint8((int(fg.R) + int(bg.R)) / 2),
		uint8((int(fg.G) + int(bg.G)) / 2),
		uint8((int(fg.B) + int(bg.B)) / 2),
		0xFF,
	}
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(dim),
		Face: face,
		Dot:  fixed.P(x, l.window.Min.Y+l.titlebar/2+4*l.scale),
	}
	d.DrawString(w.Chrome.Title)
	return nil
}

// fillRoundRect fills r with c, rounding every corner. Anti-aliasing comes
// from x/image's rasteriser, so the edges hold up when the shot is scaled.
//
// vector.Rasterizer.Draw does not clip its rectangle argument to dst's
// bounds - it indexes dst.Pix directly from that rectangle's corner, so an r
// that reaches outside dst panics (or, if the arithmetic happens not to
// panic, silently corrupts unrelated rows). r is clipped to dst.Bounds()
// before it reaches the rasteriser; the path itself is still built from the
// unclipped width/height so the curve's shape is unchanged; only the offset
// of that path within the rasteriser's local coordinates shifts to account
// for the part that got clipped away.
func fillRoundRect(dst *image.RGBA, r image.Rectangle, radius float64, c color.RGBA) {
	w, h := float64(r.Dx()), float64(r.Dy())
	radius = math.Min(radius, math.Min(w, h)/2)
	clip := r.Intersect(dst.Bounds())
	if clip.Empty() {
		return
	}
	ox, oy := float64(r.Min.X-clip.Min.X), float64(r.Min.Y-clip.Min.Y)
	ra := vector.NewRasterizer(clip.Dx(), clip.Dy())
	ra.MoveTo(float32(ox+radius), float32(oy))
	ra.LineTo(float32(ox+w-radius), float32(oy))
	ra.QuadTo(float32(ox+w), float32(oy), float32(ox+w), float32(oy+radius))
	ra.LineTo(float32(ox+w), float32(oy+h-radius))
	ra.QuadTo(float32(ox+w), float32(oy+h), float32(ox+w-radius), float32(oy+h))
	ra.LineTo(float32(ox+radius), float32(oy+h))
	ra.QuadTo(float32(ox), float32(oy+h), float32(ox), float32(oy+h-radius))
	ra.LineTo(float32(ox), float32(oy+radius))
	ra.QuadTo(float32(ox), float32(oy), float32(ox+radius), float32(oy))
	ra.ClosePath()
	ra.Draw(dst, clip, image.NewUniform(c), image.Point{})
}

// kappa is the control-point distance that turns four cubic segments into a
// circle no eye can tell from the real thing. Four quadratics give a diamond.
const kappa = 0.5522847498

// fillCircle draws a filled circle, clipped to dst's bounds for the same
// reason fillRoundRect is: see its comment. A circle entirely outside dst
// clips to an empty rectangle and draws nothing.
func fillCircle(dst *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	r := image.Rect(int(cx-radius)-2, int(cy-radius)-2, int(cx+radius)+2, int(cy+radius)+2)
	clip := r.Intersect(dst.Bounds())
	if clip.Empty() {
		return
	}
	ra := vector.NewRasterizer(clip.Dx(), clip.Dy())
	ox, oy := cx-float64(clip.Min.X), cy-float64(clip.Min.Y)
	k := radius * kappa
	ra.MoveTo(float32(ox+radius), float32(oy))
	ra.CubeTo(float32(ox+radius), float32(oy+k), float32(ox+k), float32(oy+radius), float32(ox), float32(oy+radius))
	ra.CubeTo(float32(ox-k), float32(oy+radius), float32(ox-radius), float32(oy+k), float32(ox-radius), float32(oy))
	ra.CubeTo(float32(ox-radius), float32(oy-k), float32(ox-k), float32(oy-radius), float32(ox), float32(oy-radius))
	ra.CubeTo(float32(ox+k), float32(oy-radius), float32(ox+radius), float32(oy-k), float32(ox+radius), float32(oy))
	ra.ClosePath()
	ra.Draw(dst, clip, image.NewUniform(c), image.Point{})
}
