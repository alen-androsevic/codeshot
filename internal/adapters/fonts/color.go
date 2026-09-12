package fonts

import (
	"image"
	"os"

	"golang.org/x/image/font/sfnt"

	"codeshot/internal/adapters/fonts/sbix"
)

// colorFont is a colour bitmap font and the parse of it that turns a rune
// into the glyph id its strikes are keyed by.
type colorFont struct {
	strikes *sbix.Font
	cmap    *sfnt.Font
	file    *os.File
}

// Covers reports whether anything in the chain - the chosen family, the
// embedded one, the platform's fallbacks - can draw r as an outline. It is
// what tells the rasteriser to look for a colour bitmap instead: colour is
// the last resort before tofu, so text that a monospace font can draw stays
// monochrome and keeps the grid's rhythm.
func (s *Set) Covers(r rune) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.covers(s.fonts[variantRegular], r) {
		return true
	}
	for _, f := range s.fallbackFonts() {
		if s.covers(f, r) {
			return true
		}
	}
	return false
}

// ColorGlyph is the bitmap for a rune from a colour font on this machine, at
// roughly px pixels. Apple Color Emoji keeps PNGs in an sbix table, which
// golang.org/x/image cannot read at all - it parses the font and then draws
// nothing - so the bitmap is decoded here and composited by the rasteriser.
//
// Emoji built from several runes - a skin tone modifier, a zero-width-joiner
// sequence - are not handled: the grid is runes, and one rune is what this
// gets. The base emoji is what appears.
func (s *Set) ColorGlyph(r rune, px int) (image.Image, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.colorGlyphs == nil {
		s.colorGlyphs = map[colorKey]image.Image{}
	}
	key := colorKey{r, px}
	if img, ok := s.colorGlyphs[key]; ok {
		return img, img != nil
	}
	img := s.decodeColor(r, px)
	s.colorGlyphs[key] = img
	return img, img != nil
}

type colorKey struct {
	r  rune
	px int
}

func (s *Set) decodeColor(r rune, px int) image.Image {
	for _, font := range s.colorFonts() {
		gid, err := font.cmap.GlyphIndex(&s.buf, r)
		if err != nil || gid == 0 {
			continue
		}
		img, err := font.strikes.Glyph(int(gid), px)
		if err != nil || img == nil {
			continue
		}
		return img
	}
	return nil
}

// colorFonts opens the platform's colour fonts the first time an uncovered
// rune asks for one. Apple Color Emoji is 192MB and is read through the file
// rather than into memory, a strike at a time.
func (s *Set) colorFonts() []colorFont {
	if s.colorLoaded {
		return s.color
	}
	s.colorLoaded = true
	for _, name := range emojiFamilies() {
		fam, ok := s.index.Lookup(name)
		if !ok {
			continue
		}
		file := fam.Files[variantRegular]
		if file.Empty() {
			continue
		}
		f, err := os.Open(file.Path)
		if err != nil {
			continue
		}
		strikes, err := sbix.New(f, file.Index)
		if err != nil {
			f.Close()
			continue
		}
		collection, err := sfnt.ParseCollectionReaderAt(f)
		if err != nil {
			f.Close()
			continue
		}
		parsed, err := collection.Font(file.Index)
		if err != nil {
			f.Close()
			continue
		}
		s.color = append(s.color, colorFont{strikes: strikes, cmap: parsed, file: f})
	}
	return s.color
}
