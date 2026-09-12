package raster

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"codeshot/internal/domain"
)

// drawCells paints backgrounds first, in runs, then glyphs. Merging runs of
// equal background into one rectangle is not an optimisation: filling cell by
// cell leaves visible seams where anti-aliased edges meet.
func (r Renderer) drawCells(img *image.RGBA, l layout, w domain.Window) error {
	for y, line := range w.Frame.Grid.Lines {
		top := l.grid.Y + y*l.metrics.CellH
		r.drawBackgrounds(img, l, w.Theme, line, top)
		if err := r.drawGlyphs(img, l, w.Theme, line, top); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) drawBackgrounds(img *image.RGBA, l layout, th domain.Theme, line []domain.Cell, top int) {
	x := 0
	for x < len(line) {
		_, bg := th.Resolve(line[x].Style)
		run := x + 1
		for run < len(line) {
			if _, next := th.Resolve(line[run].Style); next != bg {
				break
			}
			run++
		}
		if bg != th.Background {
			rect := image.Rect(
				l.grid.X+x*l.metrics.CellW, top,
				l.grid.X+run*l.metrics.CellW, top+l.metrics.CellH,
			)
			draw.Draw(img, rect, image.NewUniform(rgba(bg)), image.Point{}, draw.Src)
		}
		x = run
	}
}

func (r Renderer) drawGlyphs(img *image.RGBA, l layout, th domain.Theme, line []domain.Cell, top int) error {
	baseline := top + l.metrics.Ascent
	for x, cell := range line {
		if cell.Width == 0 || ((cell.Rune == 0 || cell.Rune == ' ') && cell.Combining == "") {
			// Even a width-0 continuation cell must get its decorations: the
			// emulator copies the leading cell's Style onto it (vt/buffer.go),
			// so an underline or strike under a wide rune needs a rectangle
			// drawn at each cell's own offset to cover the whole rune.
			r.drawDecorations(img, l, th, cell, x, top, baseline)
			continue
		}
		// FaceFor, not Face: the rune decides which font draws it, so a
		// glyph the chosen family lacks is picked up by something that has
		// it rather than drawn as tofu.
		face, err := r.fonts.FaceFor(cell.Rune, cell.Style, l.sizePx)
		if err != nil {
			return err
		}
		fg, _ := th.Resolve(cell.Style)
		text := string(cell.Rune) + cell.Combining
		left := l.grid.X + x*l.metrics.CellW
		if r.fonts.SyntheticItalic(cell.Style) {
			r.drawSheared(img, l, face, fg, text, left, baseline, cell.Style)
		} else {
			d := font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(rgba(fg)),
				Face: face,
				Dot:  fixed.P(left, baseline),
			}
			start := d.Dot
			d.DrawString(text)
			if r.fonts.SyntheticBold(cell.Style) {
				d.Dot = start
				d.Dot.X += fixed.I(l.scale)
				d.DrawString(text)
			}
		}
		r.drawDecorations(img, l, th, cell, x, top, baseline)
	}
	return nil
}

// drawDecorations adds the rules SGR asks for but a font cannot supply.
func (r Renderer) drawDecorations(img *image.RGBA, l layout, th domain.Theme, cell domain.Cell, x, top, baseline int) {
	if !cell.Style.Has(domain.AttrUnderline) && !cell.Style.Has(domain.AttrStrike) {
		return
	}
	fg, _ := th.Resolve(cell.Style)
	thickness := l.scale
	left := l.grid.X + x*l.metrics.CellW
	right := left + l.metrics.CellW
	if cell.Style.Has(domain.AttrUnderline) {
		y := baseline + 2*l.scale
		draw.Draw(img, image.Rect(left, y, right, y+thickness), image.NewUniform(rgba(fg)), image.Point{}, draw.Src)
	}
	if cell.Style.Has(domain.AttrStrike) {
		y := baseline - l.metrics.Ascent/3
		draw.Draw(img, image.Rect(left, y, right, y+thickness), image.NewUniform(rgba(fg)), image.Point{}, draw.Src)
	}
}

// shearTangent is the tangent of design §7's 12 degrees, the slant a
// synthetic italic gets when the family has no italic cut of its own.
const shearTangent = 0.2126

// drawSheared draws text slanted, for a family with no italic cut. The glyph
// goes onto a transparent scratch image first and is copied back a row at a
// time, each row shifted by its distance from the baseline: above it to the
// right, below it to the left, so the letter leans while standing on the same
// spot. Rows are shifted by whole pixels rather than resampled, which keeps
// stems crisp and the output byte-identical from one machine to the next.
func (r Renderer) drawSheared(img *image.RGBA, l layout, face font.Face, fg domain.RGBA, text string, left, baseline int, st domain.Style) {
	// A cell of slack on each side, and half a cell above and below, so
	// ascenders, descenders and the lean itself have somewhere to go.
	originX, originY := l.metrics.CellW, l.metrics.CellH
	scratch := image.NewRGBA(image.Rect(0, 0, 3*l.metrics.CellW, 3*l.metrics.CellH))
	d := font.Drawer{
		Dst:  scratch,
		Src:  image.NewUniform(rgba(fg)),
		Face: face,
		Dot:  fixed.P(originX, originY),
	}
	start := d.Dot
	d.DrawString(text)
	if r.fonts.SyntheticBold(st) {
		d.Dot = start
		d.Dot.X += fixed.I(l.scale)
		d.DrawString(text)
	}

	b := scratch.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		dx := int(math.Round(shearTangent * float64(originY-y)))
		draw.Draw(img,
			image.Rect(left-originX+dx, baseline-originY+y, left-originX+dx+b.Dx(), baseline-originY+y+1),
			scratch, image.Pt(b.Min.X, y), draw.Over)
	}
}
