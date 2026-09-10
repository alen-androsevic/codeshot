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

// ghosttyConfig is what a Ghostty *config* file usually looks like: it names
// a theme rather than spelling one out, so it carries no literal colours at
// all. It is by far the likeliest wrong thing to be handed to --theme,
// because it lives at the path (~/.config/ghostty/config) a user is most
// likely to remember.
const ghosttyConfig = `theme = tokyonight
font-family = JetBrains Mono
font-size = 13
window-padding-x = 10
keybind = cmd+k=clear_screen
`

// TestParseRejectsAFileWithNoColours guards against the worst failure mode
// this parser has: every key it does not recognise is skipped, so before this
// check any file at all parsed "successfully" into the zero Theme. A zero
// Theme has a fully transparent background and a black foreground, so
// codeshot rendered a shadow, three traffic lights and a title floating on
// nothing, wrote it out and exited 0. A silently wrong picture is worse than
// an error, so a theme must at least say what its background and foreground
// are.
func TestParseRejectsAFileWithNoColours(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"a ghostty config", ghosttyConfig},
		{"an unrelated file", "hello, world\n"},
		{"a foreground with no background", "foreground = #ffffff\n"},
		{"a background with no foreground", "background = #000000\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			th, err := Parse(strings.NewReader(c.body), "x")
			if err == nil {
				t.Fatalf("Parse accepted a file with no usable colours, giving %+v", th)
			}
			if !strings.Contains(err.Error(), "ground") {
				t.Errorf("error = %q, want it to name the missing background or foreground", err)
			}
		})
	}
}

// TestSourceDistinguishesAMalformedThemeFromAMissingOne pins the second half
// of the same problem. Source.Theme's directory scan treated any error as
// "not here, keep looking", so a theme that was found and then failed to
// parse came back as the generic "not embedded, not in any theme directory,
// and not a readable file" - which sends the user hunting for a file that is
// sitting right where they put it. Found-but-invalid has to be reported as
// itself, immediately.
func TestSourceDistinguishesAMalformedThemeFromAMissingOne(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tokyonight"), []byte(ghosttyConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Source{Dirs: []string{dir}}
	_, err := s.Theme("tokyonight")
	if err == nil {
		t.Fatal("Theme accepted a file in a theme directory that carries no colours")
	}
	if strings.Contains(err.Error(), "not in any theme directory") {
		t.Errorf("error = %q, want the parse failure itself, not the not-found message", err)
	}
	if !strings.Contains(err.Error(), "ground") {
		t.Errorf("error = %q, want it to say what was wrong with the file", err)
	}
}

// TestSourceReportsAMalformedFileGivenAsAPath is the same distinction for the
// last resort in the chain, where the name is tried as a path.
func TestSourceReportsAMalformedFileGivenAsAPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(ghosttyConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Source{}.Theme(path)
	if err == nil {
		t.Fatal("Theme accepted a path that carries no colours")
	}
	if strings.Contains(err.Error(), "not a readable file") {
		t.Errorf("error = %q, want the parse failure itself; the file read fine", err)
	}
}
