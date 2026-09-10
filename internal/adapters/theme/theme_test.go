package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeshot/internal/domain"
)

const sample = `# a comment
background = #101214
foreground = c8ccd4
cursor-color = #4d9fe8
palette = 0=#1b1e24
palette = 9 = #ff6f78
selection-background = #333333
unknown-key = whatever
`

func TestParseReadsGhosttyThemeFiles(t *testing.T) {
	th, err := Parse(strings.NewReader(sample), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if th.Name != "sample" {
		t.Errorf("Name = %q", th.Name)
	}
	if (th.Background != domain.RGBA{0x10, 0x12, 0x14, 0xFF}) {
		t.Errorf("Background = %v", th.Background)
	}
	if (th.Foreground != domain.RGBA{0xC8, 0xCC, 0xD4, 0xFF}) {
		t.Errorf("Foreground = %v, want the hash to be optional", th.Foreground)
	}
	if (th.Cursor != domain.RGBA{0x4D, 0x9F, 0xE8, 0xFF}) {
		t.Errorf("Cursor = %v", th.Cursor)
	}
	if (th.Palette[0] != domain.RGBA{0x1B, 0x1E, 0x24, 0xFF}) {
		t.Errorf("Palette[0] = %v", th.Palette[0])
	}
	if (th.Palette[9] != domain.RGBA{0xFF, 0x6F, 0x78, 0xFF}) {
		t.Errorf("Palette[9] = %v, want spaces around the index to be tolerated", th.Palette[9])
	}
}

func TestParseRejectsGarbageColours(t *testing.T) {
	if _, err := Parse(strings.NewReader("background = nonsense"), "x"); err == nil {
		t.Error("Parse accepted a colour that is not a colour")
	}
}

func TestParseIgnoresOutOfRangePaletteIndices(t *testing.T) {
	if _, err := Parse(strings.NewReader("palette = 300=#ffffff"), "x"); err == nil {
		t.Error("Parse accepted palette index 300")
	}
}

func TestDefaultThemeIsComplete(t *testing.T) {
	th := Default()
	if th.Name != "codeshot-dark" {
		t.Errorf("Name = %q, want codeshot-dark", th.Name)
	}
	if th.Background.A != 0xFF || th.Foreground.A != 0xFF {
		t.Error("default theme has transparent background or foreground")
	}
	for i, c := range th.Palette {
		if c.A != 0xFF {
			t.Errorf("palette entry %d is unset", i)
		}
	}
}

func TestSourceFindsEmbeddedThemesByName(t *testing.T) {
	var s Source
	for _, name := range []string{"codeshot-dark", "codeshot-light"} {
		if _, err := s.Theme(name); err != nil {
			t.Errorf("Theme(%q): %v", name, err)
		}
	}
}

func TestSourceFallsBackToAPathOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mine.conf")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	var s Source
	th, err := s.Theme(path)
	if err != nil {
		t.Fatalf("Theme(path): %v", err)
	}
	if th.Name != "mine" {
		t.Errorf("Name = %q, want the file's base name without its extension", th.Name)
	}
}

func TestSourceSearchesConfiguredDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tokyonight"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Source{Dirs: []string{dir}}
	if _, err := s.Theme("tokyonight"); err != nil {
		t.Errorf("Theme: %v", err)
	}
}

func TestSourceReportsAnUnknownTheme(t *testing.T) {
	var s Source
	if _, err := s.Theme("no-such-theme"); err == nil {
		t.Error("Theme accepted a name it cannot resolve")
	}
}
