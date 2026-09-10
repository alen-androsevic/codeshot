package raster

import (
	"image"
	"image/draw"

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
			if cell.Width != 0 {
				r.drawDecorations(img, l, th, cell, x, top, baseline)
			}
			continue
		}
		face, err := r.fonts.Face(cell.Style, l.sizePx)
		if err != nil {
			return err
		}
		fg, _ := th.Resolve(cell.Style)
		d := font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(rgba(fg)),
			Face: face,
			Dot:  fixed.P(l.grid.X+x*l.metrics.CellW, baseline),
		}
		start := d.Dot
		d.DrawString(string(cell.Rune) + cell.Combining)
		if r.fonts.SyntheticBold(cell.Style) {
			d.Dot = start
			d.Dot.X += fixed.I(l.scale)
			d.DrawString(string(cell.Rune) + cell.Combining)
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
