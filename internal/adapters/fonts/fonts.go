// Package fonts turns the embedded typeface into the faces and cell metrics
// the rasteriser needs. JetBrains Mono is Ghostty's own default, so shipping it
// is what makes a codeshot resemble a Ghostty window for free.
package fonts

import (
	"embed"
	"fmt"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	"codeshot/internal/domain"
)

//go:embed assets/*.ttf
var assets embed.FS

// Metrics is the cell grid's geometry in whole pixels. Rounding the advance to
// an integer is what keeps a hundred columns from drifting half a glyph wide.
type Metrics struct {
	CellW  int
	CellH  int
	Ascent int
}

type faceKey struct {
	variant int
	size    float64
}

// Set is the four faces of one family, plus a cache of the sized faces built
// from them.
type Set struct {
	fonts [4]*sfnt.Font
	faces map[faceKey]font.Face
	buf   sfnt.Buffer
}

const (
	variantRegular = iota
	variantBold
	variantItalic
	variantBoldItalic
)

func Embedded() (*Set, error) {
	s := &Set{faces: map[faceKey]font.Face{}}
	files := [4]string{
		"assets/JetBrainsMonoNL-Regular.ttf",
		"assets/JetBrainsMonoNL-Bold.ttf",
		"assets/JetBrainsMonoNL-Italic.ttf",
		"assets/JetBrainsMonoNL-BoldItalic.ttf",
	}
	for i, name := range files {
		data, err := assets.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("embedded font %s: %w", name, err)
		}
		f, err := opentype.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("embedded font %s: %w", name, err)
		}
		s.fonts[i] = f
	}
	return s, nil
}

func variant(st domain.Style) int {
	switch {
	case st.Has(domain.AttrBold) && st.Has(domain.AttrItalic):
		return variantBoldItalic
	case st.Has(domain.AttrBold):
		return variantBold
	case st.Has(domain.AttrItalic):
		return variantItalic
	default:
		return variantRegular
	}
}

// SyntheticBold reports whether the rasteriser must fake bold by double
// striking. With the embedded family it never has to; a fallback font in a
// later phase may say otherwise.
func (s *Set) SyntheticBold(st domain.Style) bool {
	return st.Has(domain.AttrBold) && s.fonts[variant(st)] == nil
}

// Face returns a cached face. sizePx is already scaled: DPI is fixed at 72 so
// that one point is one pixel and callers do the scaling arithmetic once.
func (s *Set) Face(st domain.Style, sizePx float64) (font.Face, error) {
	key := faceKey{variant(st), sizePx}
	if f, ok := s.faces[key]; ok {
		return f, nil
	}
	src := s.fonts[key.variant]
	if src == nil {
		src = s.fonts[variantRegular]
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	s.faces[key] = f
	return f, nil
}

func (s *Set) Metrics(sizePx, lineHeight float64) (Metrics, error) {
	f, err := s.Face(domain.Style{}, sizePx)
	if err != nil {
		return Metrics{}, err
	}
	adv, ok := f.GlyphAdvance('M')
	if !ok {
		return Metrics{}, fmt.Errorf("font has no advance for M")
	}
	fm := f.Metrics()
	return Metrics{
		CellW:  int(math.Round(float64(adv) / 64)),
		CellH:  int(math.Round(float64(fm.Height) / 64 * lineHeight)),
		Ascent: int(math.Round(float64(fm.Ascent) / 64)),
	}, nil
}

// CoversRune reports whether the regular face has a glyph for r. Everything
// else renders as tofu, and saying so is better than drawing a lie.
func (s *Set) CoversRune(r rune) bool {
	idx, err := s.fonts[variantRegular].GlyphIndex(&s.buf, r)
	return err == nil && idx != 0
}
