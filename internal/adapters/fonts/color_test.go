package fonts

import (
	"image"
	"testing"

	"codeshot/internal/domain"
)

// emojiSet is the embedded family with the machine's real font index behind
// it, which is the only way to reach a colour font.
func emojiSet(t *testing.T) *Set {
	t.Helper()
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	s = s.WithIndex(LoadIndex("", SystemFontDirs()))
	if len(emojiFamilies()) == 0 {
		t.Skip("no colour bitmap font format is supported on this platform")
	}
	if _, ok := s.ColorGlyph('🎉', 32); !ok {
		t.Skip("no colour emoji font is installed here")
	}
	return s
}

// TestCoversIsFalseForAnEmoji is what sends the rasteriser looking for a
// bitmap. JetBrains Mono has no 🎉 and neither has any outline fallback.
func TestCoversIsFalseForAnEmoji(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	s = s.WithIndex(LoadIndex("", SystemFontDirs()))
	if s.Covers('🎉') {
		t.Error("an outline font claims to draw 🎉; colour would never be reached")
	}
	if !s.Covers('a') {
		t.Error("Covers says the embedded font cannot draw 'a'")
	}
}

// TestColorGlyphDecodesAnEmoji is 3.5 in one assertion: the rune that has
// been a documented tofu limitation since phase 1 now has a picture.
func TestColorGlyphDecodesAnEmoji(t *testing.T) {
	s := emojiSet(t)
	img, ok := s.ColorGlyph('🎉', 32)
	if !ok || img == nil {
		t.Fatal("no bitmap for 🎉")
	}
	b := img.Bounds()
	if b.Dx() < 16 || b.Dy() < 16 {
		t.Errorf("bounds = %v, want a real bitmap", b)
	}
	if !manyColours(img) {
		t.Error("the bitmap has one colour in it; a party popper has several")
	}
}

func manyColours(img image.Image) bool {
	b := img.Bounds()
	first := img.At(b.Min.X, b.Min.Y)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != first {
				return true
			}
		}
	}
	return false
}

func TestColorGlyphIsCached(t *testing.T) {
	s := emojiSet(t)
	first, _ := s.ColorGlyph('🎉', 32)
	second, _ := s.ColorGlyph('🎉', 32)
	if first != second {
		t.Error("the same emoji at the same size was decoded twice")
	}
}

// TestColorGlyphOfOrdinaryTextIsNothing: a rune an outline font can draw must
// not be swapped for a bitmap, and one nothing has is still nothing.
func TestColorGlyphOfOrdinaryTextIsNothing(t *testing.T) {
	s := emojiSet(t)
	if _, ok := s.ColorGlyph('a', 32); ok {
		t.Error("'a' came back as a colour bitmap")
	}
}

// TestColorGlyphWithoutAnIndexIsQuiet: an index that could not be built
// leaves emoji as tofu, exactly as before, rather than failing a render.
func TestColorGlyphWithoutAnIndexIsQuiet(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.ColorGlyph('🎉', 32); ok {
		t.Error("a Set with no index produced a colour glyph from nowhere")
	}
	if s.SyntheticItalic(domain.Style{Attrs: domain.AttrItalic}) {
		t.Error("the embedded family has an italic cut and should not be sheared")
	}
}
