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

// symbolIndex is an index holding exactly one family, under the name given,
// backed by a real font file. One family is the point: nothing in the
// platform's fixed fallback list is in here, so anything the chain finds it
// found by the rule under test rather than by being Menlo.
func symbolIndex(t *testing.T, name string) Index {
	t.Helper()
	const path = "/System/Library/Fonts/Apple Symbols.ttf"
	if _, err := os.Stat(path); err != nil {
		t.Skip("Apple Symbols is not installed here")
	}
	return Index{Families: map[string]Family{
		key(name): {Name: name, Files: [4]FontFile{variantRegular: {Path: path}}},
	}}
}

// chainCovers reports whether anything in the fallback chain can draw r.
//
// The premise - that the embedded family has no glyph for r, so that finding
// one proves the chain was walked - is measured on a Set with no index at
// all. Covers walks the fallback chain itself, so asking it after the index
// is attached asks whether the font under test is reachable, which is the
// assertion rather than its premise, and turns a real failure into a skip.
func chainCovers(t *testing.T, name string, r rune) bool {
	t.Helper()
	bare, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if bare.Covers(r) {
		t.Skipf("the embedded family already draws %q; this test needs a rune it does not have", r)
	}
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	s = s.WithIndex(symbolIndex(t, name))
	for _, f := range s.fallbackFonts() {
		if s.covers(f, r) {
			return true
		}
	}
	return false
}

// TestFallbackChainReachesAnInstalledNerdFont is the icons-are-tofu report.
// A Nerd Font is not on any platform's fixed fallback list - it cannot be,
// the names are invented by whoever patched the font - so `ll` came out with
// crossed-out boxes where the icons were even for someone who had one
// installed, unless they named it with --font, which would then have to draw
// the text too.
func TestFallbackChainReachesAnInstalledNerdFont(t *testing.T) {
	if !chainCovers(t, "Symbols Nerd Font Mono", '☂') {
		t.Error("an installed Nerd Font is not consulted for a rune nothing else draws")
	}
}

// TestFallbackChainIgnoresAnOrdinaryInstalledFamily is the other half: the
// chain is a short list of known names plus symbol fonts, not everything on
// the machine. Opening several hundred families to find one glyph is what
// this rule exists to avoid.
func TestFallbackChainIgnoresAnOrdinaryInstalledFamily(t *testing.T) {
	if chainCovers(t, "Some Ordinary Face", '☂') {
		t.Error("an unrelated installed family was consulted; the chain has become every font on the machine")
	}
}

func TestSymbolFamilies(t *testing.T) {
	idx := Index{Families: map[string]Family{}}
	for _, name := range []string{
		"Symbols Nerd Font Mono", "JetBrainsMono Nerd Font", "Hack Nerd Font Propo",
		"Menlo", "Helvetica", "Powerline Extra Symbols",
	} {
		idx.Families[key(name)] = Family{Name: name}
	}
	got := idx.SymbolFamilies()
	want := []string{"Symbols Nerd Font Mono", "Hack Nerd Font Propo", "JetBrainsMono Nerd Font", "Powerline Extra Symbols"}
	if len(got) != len(want) {
		t.Fatalf("SymbolFamilies() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SymbolFamilies()[%d] = %q, want %q (a symbols-only font is the one to prefer, then alphabetical)", i, got[i], want[i])
		}
	}
}
