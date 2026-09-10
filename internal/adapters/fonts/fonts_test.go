package fonts

import (
	"testing"

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
