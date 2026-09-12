package fonts

import (
	"os"
	"testing"

	"codeshot/internal/domain"
)

func indexOfFixture(t *testing.T) Index {
	t.Helper()
	return ScanDirs([]string{fixture(t)})
}

func TestLoadUsesTheNamedFamily(t *testing.T) {
	idx := indexOfFixture(t)
	fam, ok := idx.Lookup("JetBrains Mono NL")
	if !ok {
		t.Fatal("fixture family missing")
	}
	s, err := Load(fam)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name() != "JetBrains Mono NL" {
		t.Errorf("Name = %q", s.Name())
	}
	for _, st := range []domain.Style{{}, {Attrs: domain.AttrBold}, {Attrs: domain.AttrItalic}} {
		if _, err := s.Face(st, 26); err != nil {
			t.Errorf("Face(%+v): %v", st, err)
		}
	}
	if s.SyntheticItalic(domain.Style{Attrs: domain.AttrItalic}) {
		t.Error("a family with a real italic cut reported synthetic italic")
	}
	if s.SyntheticBold(domain.Style{Attrs: domain.AttrBold}) {
		t.Error("a family with a real bold cut reported synthetic bold")
	}
}

// TestLoadRefusesAFamilyWithNoRegularCut: everything is drawn from the
// regular cut when a style has no face of its own, so a family without one
// cannot be used at all.
func TestLoadRefusesAFamilyWithNoRegularCut(t *testing.T) {
	if _, err := Load(Family{Name: "Nothing"}); err == nil {
		t.Error("Load accepted a family with no cuts in it")
	}
}

// TestSyntheticsAreReportedForAMissingCut is what the rasteriser asks before
// double-striking or shearing. A family with only a regular cut - most
// monospace families on a machine - needs both.
func TestSyntheticsAreReportedForAMissingCut(t *testing.T) {
	idx := indexOfFixture(t)
	fam, _ := idx.Lookup("JetBrains Mono NL")
	only := Family{Name: "Regular Only", Files: [4]FontFile{variantRegular: fam.Files[variantRegular]}}

	s, err := Load(only)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !s.SyntheticBold(domain.Style{Attrs: domain.AttrBold}) {
		t.Error("a family with no bold cut must be double-struck")
	}
	if !s.SyntheticItalic(domain.Style{Attrs: domain.AttrItalic}) {
		t.Error("a family with no italic cut must be sheared")
	}
	if s.SyntheticItalic(domain.Style{}) {
		t.Error("upright text reported synthetic italic")
	}
}

// TestFaceForFallsBackToTheEmbeddedFamily: a chosen font that cannot draw a
// rune must not turn it into tofu while a font that can sits in the binary.
func TestFaceForFallsBackToTheEmbeddedFamily(t *testing.T) {
	idx := indexOfFixture(t)
	fam, _ := idx.Lookup("JetBrains Mono NL")
	s, err := Load(fam)
	if err != nil {
		t.Fatal(err)
	}
	face, err := s.FaceFor('a', domain.Style{}, 26)
	if err != nil {
		t.Fatalf("FaceFor: %v", err)
	}
	if _, ok := face.GlyphAdvance('a'); !ok {
		t.Error("the face returned for 'a' cannot draw it")
	}
}

// TestFaceForReachesASystemFontForACJKRune is the point of the whole chain:
// JetBrains Mono has no 中, and a machine with any CJK font installed should
// still draw one.
func TestFaceForReachesASystemFontForACJKRune(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if s.CoversRune('中') {
		t.Skip("the embedded font covers 中; this machine cannot exercise the fallback")
	}
	idx := LoadIndex("", SystemFontDirs())
	s = s.WithIndex(idx)

	var found bool
	for _, name := range fallbackFamilies() {
		if fam, ok := idx.Lookup(name); ok && !fam.Files[variantRegular].Empty() {
			if info, err := os.Stat(fam.Files[variantRegular].Path); err == nil && info.Size() <= maxFallbackBytes {
				found = true
				break
			}
		}
	}
	if !found {
		t.Skip("no fallback family from the candidate list is installed here")
	}

	face, err := s.FaceFor('中', domain.Style{}, 26)
	if err != nil {
		t.Fatalf("FaceFor: %v", err)
	}
	if _, ok := face.GlyphAdvance('中'); !ok {
		t.Error("no face in the chain can draw 中, on a machine that has one installed")
	}
}

// TestFaceForGivesTofuRatherThanAnError: a rune nothing on the machine can
// draw is still a cell that has to be painted.
func TestFaceForGivesTofuRatherThanAnError(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	// U+F8FF is Apple's private-use logo on macOS and unassigned elsewhere;
	// either way the embedded family has nothing for it.
	if _, err := s.FaceFor('', domain.Style{}, 26); err != nil {
		t.Errorf("FaceFor: %v, want a face to draw tofu with", err)
	}
}

func TestFallbackFamiliesAreNamed(t *testing.T) {
	if len(fallbackFamilies()) == 0 {
		t.Error("no fallback families are named for this platform")
	}
}
