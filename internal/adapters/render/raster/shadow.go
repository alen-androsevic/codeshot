package raster

import (
	"image"
	"image/color"
)

// drawShadow paints a soft dark shape behind the window. It builds the
// window's silhouette as an alpha mask, blurs it with three box passes - which
// is close enough to a gaussian that no eye will argue - and composites black
// through the result.
func drawShadow(dst *image.RGBA, window image.Rectangle, radius float64, blur, offsetY int, alpha float64) {
	if blur < 1 {
		blur = 1
	}
	bounds := dst.Bounds()
	mask := image.NewAlpha(bounds)
	silhouette := image.NewRGBA(bounds)
	fillRoundRect(silhouette, window.Add(image.Pt(0, offsetY)), radius, color.RGBA{0, 0, 0, 0xFF})
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			mask.SetAlpha(x, y, color.Alpha{A: silhouette.RGBAAt(x, y).A})
		}
	}
	radiusPx := blur / 3
	for i := 0; i < 3; i++ {
		mask = boxBlur(mask, radiusPx)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			a := float64(mask.AlphaAt(x, y).A) * alpha
			if a < 1 {
				continue
			}
			dst.SetRGBA(x, y, blendOver(dst.RGBAAt(x, y), color.RGBA{0, 0, 0, uint8(a)}))
		}
	}
}

// boxBlur runs one separable pass. Two nested loops each way beat one clever
// loop nobody can read.
func boxBlur(src *image.Alpha, radius int) *image.Alpha {
	if radius < 1 {
		return src
	}
	b := src.Bounds()
	tmp := image.NewAlpha(b)
	out := image.NewAlpha(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum, n := 0, 0
			for dx := -radius; dx <= radius; dx++ {
				if x+dx < b.Min.X || x+dx >= b.Max.X {
					continue
				}
				sum += int(src.AlphaAt(x+dx, y).A)
				n++
			}
			tmp.SetAlpha(x, y, color.Alpha{A: uint8(sum / n)})
		}
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum, n := 0, 0
			for dy := -radius; dy <= radius; dy++ {
				if y+dy < b.Min.Y || y+dy >= b.Max.Y {
					continue
				}
				sum += int(tmp.AlphaAt(x, y+dy).A)
				n++
			}
			out.SetAlpha(x, y, color.Alpha{A: uint8(sum / n)})
		}
	}
	return out
}

// blendOver is source-over compositing for a straight-alpha RGBA image.
func blendOver(dst, src color.RGBA) color.RGBA {
	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA == 0 {
		return color.RGBA{}
	}
	mix := func(s, d uint8) uint8 {
		return uint8((float64(s)*sa + float64(d)*da*(1-sa)) / outA)
	}
	return color.RGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), uint8(outA * 255)}
}
