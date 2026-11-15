package fonts

import (
	"sync"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"codeshot/internal/domain"
)

func TestEmbeddedLoadsAllFourFaces(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
	styles := []domain.Style{
		{},
		{Attrs: domain.AttrBold},
		{Attrs: domain.AttrItalic},
		{Attrs: domain.AttrBold | domain.AttrItalic},
	}
	for _, st := range styles {
		if _, err := s.Face(st, 26); err != nil {
			t.Errorf("Face(%+v): %v", st, err)
		}
		if s.SyntheticBold(st) {
			t.Errorf("style %+v reported synthetic bold, but a real face is embedded", st)
		}
	}
}

func TestMetricsAreSquareIshAndPositive(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Metrics(26, 1.0)
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if m.CellW <= 0 || m.CellH <= 0 || m.Ascent <= 0 {
		t.Fatalf("metrics = %+v, want all positive", m)
	}
	if m.CellH <= m.CellW {
		t.Errorf("metrics = %+v, want a cell taller than it is wide", m)
	}
	if m.Ascent >= m.CellH {
		t.Errorf("ascent %d does not fit in cell height %d", m.Ascent, m.CellH)
	}
}

func TestLineHeightStretchesTheCell(t *testing.T) {
	s, _ := Embedded()
	tight, _ := s.Metrics(26, 1.0)
	loose, _ := s.Metrics(26, 1.5)
	if loose.CellH <= tight.CellH {
		t.Errorf("line height 1.5 gave %d, tighter than 1.0's %d", loose.CellH, tight.CellH)
	}
	if loose.CellW != tight.CellW {
		t.Errorf("line height changed the cell width: %d vs %d", loose.CellW, tight.CellW)
	}
}

func TestCoverage(t *testing.T) {
	s, _ := Embedded()
	for _, r := range []rune{'a', 'Z', '0', 'č', '─', '│', '█'} {
		if !s.CoversRune(r) {
			t.Errorf("embedded font does not cover %q", r)
		}
	}
	// U+F000 opens the Nerd Font "Seti-UI/Devicons" range: thousands of
	// filetype and UI icons that only a Nerd-Font-patched build carries.
	// (JetBrains Mono NL does, unusually, ship a handful of genuine Powerline
	// glyphs of its own at U+E0A0-E0A2 and U+E0B0+ -- confirmed by inspecting
	// their glyph indices and contour counts, not a .notdef fallback -- so
	// that range doesn't serve as a "definitely uncovered" probe here.)
	if s.CoversRune('') {
		t.Error("CoversRune claims a Nerd Font icon an unpatched font cannot have")
	}
}

func TestFaceCacheReturnsTheSameFace(t *testing.T) {
	s, _ := Embedded()
	a, _ := s.Face(domain.Style{}, 26)
	b, _ := s.Face(domain.Style{}, 26)
	if a != b {
		t.Error("Face rebuilt an identical face; the cache is not working")
	}
}

// TestFaceSelectsTheCorrectVariant guards against variant() (or the
// s.fonts[key.variant] lookup in Face) picking the wrong one of the four
// embedded TTFs for a style. Advance width can't tell the four cuts apart --
// this is a monospaced family, so 'M' advances by the same 16px in every
// variant -- but a glyph's bounds do differ (italic slants it, bold thickens
// it), so bounds are used as the identity signal instead.
//
// Each expected shape is read directly off s.fonts[wantVariant], built with
// an independent opentype.Face that bypasses variant() and Face()'s cache
// entirely, so the "expected" side of the comparison can't share a bug with
// the code under test.
func TestFaceSelectsTheCorrectVariant(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}

	referenceBounds := func(variantIdx int) fixed.Rectangle26_6 {
		f, err := opentype.NewFace(s.fonts[variantIdx], &opentype.FaceOptions{
			Size:    26,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			t.Fatalf("reference face for variant %d: %v", variantIdx, err)
		}
		b, _, ok := f.GlyphBounds('M')
		if !ok {
			t.Fatalf("reference face for variant %d has no glyph for 'M'", variantIdx)
		}
		return b
	}

	cases := []struct {
		name string
		st   domain.Style
		want int
	}{
		{"regular", domain.Style{}, variantRegular},
		{"bold", domain.Style{Attrs: domain.AttrBold}, variantBold},
		{"italic", domain.Style{Attrs: domain.AttrItalic}, variantItalic},
		{"bold+italic", domain.Style{Attrs: domain.AttrBold | domain.AttrItalic}, variantBoldItalic},
	}
	for _, c := range cases {
		f, err := s.Face(c.st, 26)
		if err != nil {
			t.Fatalf("Face(%s): %v", c.name, err)
		}
		got, _, ok := f.GlyphBounds('M')
		if !ok {
			t.Fatalf("Face(%s) has no glyph for 'M'", c.name)
		}
		if want := referenceBounds(c.want); got != want {
			t.Errorf("Face(%s) drew 'M' with bounds %v, want variant %d's bounds %v -- wrong face selected", c.name, got, c.want, want)
		}
	}
}

// TestSetIsSafeForConcurrentUse is written for the race detector: `faces` is
// a plain map and `buf` a single shared sfnt.Buffer, so two goroutines
// touching a Set corrupt each other silently. Nothing renders concurrently
// in phase 1, but phase 2 runs a pty copy loop on its own goroutine beside
// the pipeline, and an unsynchronised map cache is exactly the sort of fault
// that surfaces once in a hundred runs and is then not believed. Run under
// `go test -race`, which the Taskfile's test task does.
func TestSetIsSafeForConcurrentUse(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	styles := []domain.Style{
		{},
		domain.Style{}.Set(domain.AttrBold),
		domain.Style{}.Set(domain.AttrItalic),
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 24; i++ {
				if _, err := s.Face(styles[i%len(styles)], float64(10+i)); err != nil {
					t.Errorf("Face: %v", err)
					return
				}
				s.CoversRune(rune('a' + i))
				if _, err := s.Metrics(float64(10+i), 1.0); err != nil {
					t.Errorf("Metrics: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
