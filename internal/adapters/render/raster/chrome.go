package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
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
	sizePx := 11 * float64(l.scale)
	face, err := r.fonts.Face(domain.Style{}, sizePx)
	if err != nil {
		return err
	}
	// The title is drawn rune by rune, like the grid below it, because the
	// title defaults to the command: `codeshot -- printf '🎉 中文'` put both
	// of those in the titlebar, where a single face drew them as tofu while
	// the output underneath had them right.
	measure := func(s string) int { return r.measureTitle(s, sizePx, face) }
	title, ok := fitTitle(measure, w.Chrome.Title, l.window.Dx()-2*titleInset(w.Chrome.Controls, l.scale))
	if !ok {
		return nil
	}
	x := l.window.Min.X + (l.window.Dx()-measure(title))/2
	// A title at full foreground strength shouts over the output it labels.
	fg, bg := w.Theme.Foreground, w.Theme.Background
	dim := color.RGBA{
		uint8((int(fg.R) + int(bg.R)) / 2),
		uint8((int(fg.G) + int(bg.G)) / 2),
		uint8((int(fg.B) + int(bg.B)) / 2),
		0xFF,
	}
	baseline := l.window.Min.Y + l.titlebar/2 + 4*l.scale
	for _, ru := range title {
		x += r.drawTitleRune(img, ru, sizePx, face, dim, x, baseline)
	}
	return nil
}

// titleEm is the square a colour glyph fills in the titlebar: one em, so an
// emoji is the size of the letters beside it.
func titleEm(sizePx float64) int { return int(math.Round(sizePx)) }

// titleFace is the face for one rune of the title, which may not be the
// title's own: the same per-rune fallback the grid uses.
func (r Renderer) titleFace(ru rune, sizePx float64, fallback font.Face) font.Face {
	if f, err := r.fonts.FaceFor(ru, domain.Style{}, sizePx); err == nil {
		return f
	}
	return fallback
}

func (r Renderer) measureTitle(s string, sizePx float64, fallback font.Face) int {
	total := 0
	for _, ru := range s {
		total += r.titleAdvance(ru, sizePx, fallback)
	}
	return total
}

func (r Renderer) titleAdvance(ru rune, sizePx float64, fallback font.Face) int {
	if !r.fonts.Covers(ru) {
		if _, ok := r.fonts.ColorGlyph(ru, titleEm(sizePx)); ok {
			return titleEm(sizePx)
		}
	}
	adv, ok := r.titleFace(ru, sizePx, fallback).GlyphAdvance(ru)
	if !ok {
		return 0
	}
	return adv.Round()
}

// drawTitleRune draws one rune and reports how far along to move.
func (r Renderer) drawTitleRune(img *image.RGBA, ru rune, sizePx float64, fallback font.Face, dim color.RGBA, x, baseline int) int {
	if !r.fonts.Covers(ru) {
		if bitmap, ok := r.fonts.ColorGlyph(ru, titleEm(sizePx)); ok {
			em := titleEm(sizePx)
			// A sixth of the em below the baseline, roughly where a
			// descender would reach, so it sits on the line rather than
			// floating above it.
			top := baseline - em + em/6
			xdraw.CatmullRom.Scale(img, image.Rect(x, top, x+em, top+em), bitmap, bitmap.Bounds(), draw.Over, nil)
			return em
		}
	}
	face := r.titleFace(ru, sizePx, fallback)
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(dim),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(string(ru))
	adv, ok := face.GlyphAdvance(ru)
	if !ok {
		return 0
	}
	return adv.Round()
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

// titleInset is how much of each side of the titlebar the window controls
// claim. The band the title may use has to be symmetric about the window's
// centre, because the title is centred: reserving only the side the controls
// actually sit on would simply push the title into that side as it grew.
//
// The furthest control's outer edge is 66px from its side of the window (its
// centre is at 60, its radius 6), and 10px more keeps the title from
// touching it. Both control styles put their outermost button the same
// distance from their own edge, so one number covers each.
func titleInset(c domain.Controls, scale int) int {
	if c == domain.ControlsNone {
		return 10 * scale
	}
	return 76 * scale
}

// fitTitle shortens title until it fits in avail pixels, marking the cut with
// an ellipsis. Chrome.Title defaults to the command, so this is the ordinary
// case and not an exotic one: without it a title of any length was centred
// unmeasured, painting over the traffic lights on one side and running off
// the image on the other.
//
// It reports false when not even the ellipsis fits, and then no title is
// drawn at all - a lone "…" sitting on top of the traffic lights tells the
// reader less than an empty titlebar does.
//
// measure is passed in rather than a face because a title's width is no
// longer one face's business: a rune may be drawn by a fallback font or by a
// colour bitmap, and a title measured with the wrong widths is a title drawn
// off centre.
//
// The search drops one rune at a time and measures again, which is quadratic
// in the length of the title. Titles are command lines, so the constant is
// tiny; a title arriving from somewhere less friendly would want a binary
// search instead.
func fitTitle(measure func(string) int, title string, avail int) (string, bool) {
	if avail <= 0 {
		return "", false
	}
	if measure(title) <= avail {
		return title, true
	}
	const ellipsis = "\u2026"
	if measure(ellipsis) > avail {
		return "", false
	}
	runes := []rune(title)
	for n := len(runes) - 1; n > 0; n-- {
		if s := string(runes[:n]) + ellipsis; measure(s) <= avail {
			return s, true
		}
	}
	return ellipsis, true
}
