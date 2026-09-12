package fonts

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture writes the embedded family out as four real font files, plus a
// file that is not a font at all, which is what the assets directory itself
// looks like: two TTFs' worth of licence and provenance text beside them.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := assets.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := assets.ReadFile("assets/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScanDirsIndexesEveryCutOfAFamily(t *testing.T) {
	idx := ScanDirs([]string{fixture(t)})
	fam, ok := idx.Lookup("JetBrains Mono NL")
	if !ok {
		t.Fatalf("family not found; index holds %v", idx.Names())
	}
	for variant, name := range map[int]string{
		variantRegular:    "Regular",
		variantBold:       "Bold",
		variantItalic:     "Italic",
		variantBoldItalic: "BoldItalic",
	} {
		file := fam.Files[variant]
		if file.Empty() {
			t.Errorf("%s cut is missing", name)
			continue
		}
		if want := "JetBrainsMonoNL-" + name + ".ttf"; filepath.Base(file.Path) != want {
			t.Errorf("%s cut came from %s, want %s", name, filepath.Base(file.Path), want)
		}
		if file.Index != 0 {
			t.Errorf("%s cut has index %d, want 0 in a plain .ttf", name, file.Index)
		}
	}
}

// TestScanDirsIgnoresWhatIsNotAFont: a broken or irrelevant file somewhere on
// the machine is no reason to refuse to take a picture.
func TestScanDirsIgnoresWhatIsNotAFont(t *testing.T) {
	dir := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "broken.ttf"), []byte("this is not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := ScanDirs([]string{dir, filepath.Join(dir, "does-not-exist")})
	if _, ok := idx.Lookup("JetBrains Mono NL"); !ok {
		t.Error("a broken file and a missing directory stopped the scan")
	}
}

func TestLookupIgnoresCaseAndSpace(t *testing.T) {
	idx := ScanDirs([]string{fixture(t)})
	for _, name := range []string{"JetBrains Mono NL", "jetbrains mono nl", "  JETBRAINS MONO NL  "} {
		if _, ok := idx.Lookup(name); !ok {
			t.Errorf("Lookup(%q) found nothing", name)
		}
	}
	if _, ok := idx.Lookup("No Such Font"); ok {
		t.Error("Lookup invented a family")
	}
}

// TestScanDirsReadsCollections covers the .ttc case, where one file holds
// several fonts and the index has to say which. Menlo is macOS's, and ships
// all four cuts in one file.
func TestScanDirsReadsCollections(t *testing.T) {
	const menlo = "/System/Library/Fonts/Menlo.ttc"
	if _, err := os.Stat(menlo); err != nil {
		t.Skip("Menlo.ttc is not installed here")
	}
	idx := ScanDirs([]string{"/System/Library/Fonts"})
	fam, ok := idx.Lookup("Menlo")
	if !ok {
		t.Fatal("Menlo not indexed")
	}
	seen := map[int]bool{}
	for _, file := range fam.Files {
		if file.Empty() {
			t.Fatalf("Menlo is missing a cut: %+v", fam.Files)
		}
		if file.Path != menlo {
			t.Errorf("cut came from %s, want %s", file.Path, menlo)
		}
		seen[file.Index] = true
	}
	if len(seen) != 4 {
		t.Errorf("the four cuts have indices %v, want four different fonts inside the collection", seen)
	}
}

// TestScanDirsTakesALightCutAsARegular pins the definitive/assumed rule:
// .SF NS Mono's only cut calls itself Light, and a family with one cut still
// has a regular one as far as drawing is concerned.
func TestScanDirsTakesALightCutAsARegular(t *testing.T) {
	if _, err := os.Stat("/System/Library/Fonts/SFNSMono.ttf"); err != nil {
		t.Skip("SFNSMono.ttf is not installed here")
	}
	idx := ScanDirs([]string{"/System/Library/Fonts"})
	fam, ok := idx.Lookup(".SF NS Mono")
	if !ok {
		t.Skip(".SF NS Mono is not indexed on this machine")
	}
	if fam.Files[variantRegular].Empty() {
		t.Error("a family whose only cut is Light has no regular to draw with")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		subfamily string
		variant   int
		definite  bool
	}{
		{"Regular", variantRegular, true},
		{"", variantRegular, true},
		{"Book", variantRegular, true},
		{"Bold", variantBold, true},
		{"Italic", variantItalic, true},
		{"Bold Italic", variantBoldItalic, true},
		{"Italic Bold", variantBoldItalic, true},
		{"Oblique", variantItalic, true},
		{"Condensed Black Oblique", variantItalic, true},
		{"Light", variantRegular, false},
		{"SemiBold", variantBold, true},
	}
	for _, c := range cases {
		variant, definite := classify(c.subfamily)
		if variant != c.variant || definite != c.definite {
			t.Errorf("classify(%q) = %d,%v want %d,%v", c.subfamily, variant, definite, c.variant, c.definite)
		}
	}
}

// TestLoadIndexUsesTheCache proves the cache is read rather than the machine
// rescanned, by doctoring it with a family no scan could produce. Scanning
// several hundred fonts on every run is exactly what it exists to avoid.
func TestLoadIndexUsesTheCache(t *testing.T) {
	dir := fixture(t)
	cache := filepath.Join(t.TempDir(), "fonts.json")

	if _, ok := LoadIndex(cache, []string{dir}).Lookup("JetBrains Mono NL"); !ok {
		t.Fatal("the first scan found nothing")
	}
	idx, ok := readCache(cache)
	if !ok {
		t.Fatal("no cache was written")
	}
	idx.Families["ghost mono"] = Family{Name: "Ghost Mono"}
	idx.write(cache)

	if _, ok := LoadIndex(cache, []string{dir}).Lookup("Ghost Mono"); !ok {
		t.Error("the cache was ignored; every run would rescan every font on the machine")
	}
}

// TestLoadIndexForgetsFontsThatAreGone is the other half, and the reason the
// cache keeps directory mtimes rather than trusting itself: a font that has
// been uninstalled must stop being offered, or --font would name a file that
// is no longer there.
func TestLoadIndexForgetsFontsThatAreGone(t *testing.T) {
	dir := fixture(t)
	cache := filepath.Join(t.TempDir(), "fonts.json")
	LoadIndex(cache, []string{dir})

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadIndex(cache, []string{dir}).Lookup("JetBrains Mono NL"); ok {
		t.Error("the index still offers a font whose directory is gone")
	}
}

// TestLoadIndexRescansWhenAFontIsInstalled is the cache's whole contract: it
// must never be the reason a font you just installed does not show up.
func TestLoadIndexRescansWhenAFontIsInstalled(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(t.TempDir(), "fonts.json")
	if idx := LoadIndex(cache, []string{dir}); len(idx.Families) != 0 {
		t.Fatalf("an empty directory yielded %v", idx.Names())
	}

	data, err := assets.ReadFile("assets/JetBrainsMonoNL-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "JetBrainsMonoNL-Regular.ttf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadIndex(cache, []string{dir}).Lookup("JetBrains Mono NL"); !ok {
		t.Error("the cache outlived the directory it described; a newly installed font is invisible")
	}
}

func TestLoadIndexRescansWhenTheDirectoriesChange(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "fonts.json")
	empty := t.TempDir()
	LoadIndex(cache, []string{empty})

	if _, ok := LoadIndex(cache, []string{empty, fixture(t)}).Lookup("JetBrains Mono NL"); !ok {
		t.Error("adding a directory did not force a rescan")
	}
}

func TestSystemFontDirsOnlyNamesWhatExists(t *testing.T) {
	for _, dir := range SystemFontDirs() {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("SystemFontDirs listed %q, which is not a directory here", dir)
		}
	}
}
