package fonts

import (
	"fmt"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"

	"codeshot/internal/domain"
)

// maxFallbackBytes keeps a fallback from being ruinous to load. The fonts
// worth consulting for a missing glyph are a few megabytes; Apple Color
// Emoji is 192MB, and it is not read here in any case - a bitmap font has
// no outlines for x/image to draw, and emoji go through the colour path.
const maxFallbackBytes = 48 << 20

// Load builds a Set from an indexed family. Cuts the family does not have
// are left empty rather than filled from somewhere else: an empty bold slot
// is what tells the rasteriser to double-strike, and an empty italic slot to
// shear, which is closer to what the family looks like than borrowing a
// stranger's letterforms.
//
// The embedded family stands behind whatever is loaded, as the first
// fallback for a rune the chosen font has no glyph for.
func Load(fam Family) (*Set, error) {
	s := &Set{faces: map[faceKey]font.Face{}, name: fam.Name}
	for i, file := range fam.Files {
		if file.Empty() {
			continue
		}
		f, err := openFont(file)
		if err != nil {
			return nil, fmt.Errorf("font %s: %w", file.Path, err)
		}
		s.fonts[i] = f
	}
	if s.fonts[variantRegular] == nil {
		return nil, fmt.Errorf("font %q has no regular cut", fam.Name)
	}
	embedded, err := Embedded()
	if err != nil {
		return nil, err
	}
	s.fallbacks = []*sfnt.Font{embedded.fonts[variantRegular]}
	return s, nil
}

// openFont reads one cut. Unlike the index, which only wants names, this
// reads the whole file: the faces built from it are drawn with for the length
// of the run, and a *sfnt.Font over a closed file is no use.
func openFont(file FontFile) (*sfnt.Font, error) {
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, err
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	if file.Index < 0 || file.Index >= collection.NumFonts() {
		return nil, fmt.Errorf("font %d is not in a collection of %d", file.Index, collection.NumFonts())
	}
	return collection.Font(file.Index)
}

// Name is the family this Set draws with, for doctor to report.
func (s *Set) Name() string {
	if s.name == "" {
		return "JetBrains Mono NL (embedded)"
	}
	return s.name
}

// SyntheticItalic reports whether the rasteriser must fake italic by
// shearing. A family with a real italic cut says no; one without - which is
// most of the monospace families on a machine - says yes, and design §7's
// 12° shear is what stands in.
func (s *Set) SyntheticItalic(st domain.Style) bool {
	return st.Has(domain.AttrItalic) && s.fonts[variantItalic] == nil && s.fonts[variantBoldItalic] == nil
}

// FaceFor is Face, for a particular rune: it returns the face that has a
// glyph for r, walking the chosen family, then the embedded one, then the
// system's fallback candidates. When nobody has it, the chosen family's own
// face comes back and the rune draws as tofu, which is an honest picture of
// a terminal that had no glyph for it either.
func (s *Set) FaceFor(r rune, st domain.Style, sizePx float64) (font.Face, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	primary, err := s.face(st, sizePx)
	if err != nil {
		return nil, err
	}
	if s.covers(s.fonts[variant(st)], r) || s.covers(s.fonts[variantRegular], r) {
		return primary, nil
	}
	for _, f := range s.fallbackFonts() {
		if !s.covers(f, r) {
			continue
		}
		face, err := s.faceOf(f, sizePx)
		if err != nil {
			continue
		}
		return face, nil
	}
	return primary, nil
}

// fallbackFonts is the chain behind the chosen family, loaded the first time
// a rune needs one. Loading them eagerly would read tens of megabytes for
// every shot; most shots are ASCII and never ask.
func (s *Set) fallbackFonts() []*sfnt.Font {
	if !s.fallbacksLoaded {
		s.fallbacksLoaded = true
		for _, name := range fallbackFamilies() {
			fam, ok := s.index.Lookup(name)
			if !ok {
				continue
			}
			file := fam.Files[variantRegular]
			if file.Empty() {
				continue
			}
			if info, err := os.Stat(file.Path); err != nil || info.Size() > maxFallbackBytes {
				continue
			}
			if f, err := openFont(file); err == nil {
				s.fallbacks = append(s.fallbacks, f)
			}
		}
	}
	return s.fallbacks
}

// WithIndex gives a Set the index its fallback chain is drawn from. Without
// one the chain is the embedded family alone, which is what happens when the
// index could not be built.
func (s *Set) WithIndex(idx Index) *Set {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index = idx
	return s
}

func (s *Set) covers(f *sfnt.Font, r rune) bool {
	if f == nil {
		return false
	}
	idx, err := f.GlyphIndex(&s.buf, r)
	return err == nil && idx != 0
}

// faceOf builds a sized face from a font that is not one of the four cuts,
// caching it the same way. The key is the font's own pointer, since a
// fallback has no variant slot of its own.
func (s *Set) faceOf(f *sfnt.Font, sizePx float64) (font.Face, error) {
	if s.fallbackFaces == nil {
		s.fallbackFaces = map[fallbackKey]font.Face{}
	}
	key := fallbackKey{f, sizePx}
	if face, ok := s.fallbackFaces[key]; ok {
		return face, nil
	}
	face, err := newFace(f, sizePx)
	if err != nil {
		return nil, err
	}
	s.fallbackFaces[key] = face
	return face, nil
}

type fallbackKey struct {
	font *sfnt.Font
	size float64
}
