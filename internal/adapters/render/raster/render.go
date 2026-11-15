// Package raster draws a Window with nothing but golang.org/x/image: no
// browser, no SVG toolchain, no terminal. That is what lets the same bytes
// produce the same PNG on a laptop and in CI.
package raster

import (
	"image"
	"image/color"
	"image/draw"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

type Options struct {
	FontSize   float64
	LineHeight float64
}

func DefaultOptions() Options { return Options{FontSize: 13, LineHeight: 1.0} }

type Renderer struct {
	fonts *fonts.Set
	opt   Options
}

func New(f *fonts.Set, opt Options) Renderer { return Renderer{fonts: f, opt: opt} }

// layout is every measurement the drawing code needs, all in device pixels.
type layout struct {
	scale    int
	metrics  fonts.Metrics
	sizePx   float64
	margin   int
	paddingX int
	paddingY int
	titlebar int
	window   image.Rectangle // within the whole image
	grid     image.Point     // top-left of the first cell
	cols     int
	rows     int
}

func (r Renderer) layout(w domain.Window) (layout, error) {
	scale := w.Chrome.Scale
	if scale < 1 {
		scale = 1
	}
	l := layout{scale: scale, sizePx: r.opt.FontSize * float64(scale)}
	m, err := r.fonts.Metrics(l.sizePx, r.opt.LineHeight)
	if err != nil {
		return layout{}, err
	}
	l.metrics = m
	l.margin = w.Chrome.Margin * scale
	l.paddingX = w.Chrome.PaddingX * scale
	l.paddingY = w.Chrome.PaddingY * scale
	l.titlebar = 0
	if w.Chrome.Controls != domain.ControlsNone || w.Chrome.ShowTitle {
		l.titlebar = w.Chrome.TitlebarHeight * scale
	}
	// Shrink the window to content width, but not below MinCols or above the
	// terminal's own width. This keeps narrow outputs compact while still
	// giving wide content room to breathe.
	contentCols := w.Frame.Grid.ContentCols()
	terminalCols := w.Frame.Grid.Cols
	l.cols = contentCols
	if w.Chrome.MinCols > 0 && l.cols < w.Chrome.MinCols {
		l.cols = w.Chrome.MinCols
	}
	if l.cols > terminalCols {
		l.cols = terminalCols
	}
	l.rows = w.Frame.Grid.Rows()
	winW := l.cols*m.CellW + 2*l.paddingX
	winH := l.rows*m.CellH + 2*l.paddingY + l.titlebar
	l.window = image.Rect(l.margin, l.margin, l.margin+winW, l.margin+winH)
	l.grid = image.Pt(l.window.Min.X+l.paddingX, l.window.Min.Y+l.titlebar+l.paddingY)
	return l, nil
}

func (r Renderer) Render(w domain.Window) (image.Image, error) {
	l, err := r.layout(w)
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, l.window.Max.X+l.margin, l.window.Max.Y+l.margin))
	if w.Chrome.Background != nil {
		draw.Draw(img, img.Bounds(), image.NewUniform(rgba(*w.Chrome.Background)), image.Point{}, draw.Src)
	}
	if w.Chrome.Shadow {
		drawShadow(img, l.window, float64(w.Chrome.Radius*l.scale), 40*l.scale, 18*l.scale, 0.35)
	}
	if err := r.drawWindow(img, l, w); err != nil {
		return nil, err
	}
	return img, nil
}

// drawWindow paints the window body and its contents: rounded corners,
// titlebar and traffic lights from drawChrome, then the grid on top.
func (r Renderer) drawWindow(img *image.RGBA, l layout, w domain.Window) error {
	if err := r.drawChrome(img, l, w); err != nil {
		return err
	}
	return r.drawCells(img, l, w)
}

func rgba(c domain.RGBA) color.RGBA { return color.RGBA{c.R, c.G, c.B, c.A} }
