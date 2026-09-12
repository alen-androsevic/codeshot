package ghostty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// real is a Ghostty config of the shape people actually keep, taken from the
// one this was written against: a dual theme, a commented-out font-family,
// separate x and y padding, and a pile of keys codeshot has no business
// reading.
const real = `# ~/.config/ghostty/config — terminal rendering for cmux.
theme = dark:Catppuccin Mocha,light:Catppuccin Latte
font-size = 13
# font-family = JetBrains Mono

macos-option-as-alt = true
cursor-style = block
window-padding-x = 8
window-padding-y = 6
keybind = ctrl+shift+f=csi:102;6u
`

func TestParseReadsWhatCodeshotCanHonour(t *testing.T) {
	cfg := Parse(strings.NewReader(real))
	if cfg.Theme != "Catppuccin Mocha" {
		t.Errorf("Theme = %q, want the dark half of the pair", cfg.Theme)
	}
	if cfg.FontSize != 13 {
		t.Errorf("FontSize = %v", cfg.FontSize)
	}
	if cfg.PaddingX != 8 || cfg.PaddingY != 6 {
		t.Errorf("padding = %d,%d, want 8,6", cfg.PaddingX, cfg.PaddingY)
	}
	if cfg.FontFamily != "" {
		t.Errorf("FontFamily = %q, want a commented-out line ignored", cfg.FontFamily)
	}
}

// TestParseTakesTheDarkHalf records a decision. Ghostty's theme setting can
// follow the system appearance; codeshot cannot, without the same command
// producing different pictures at different times of day.
func TestParseTakesTheDarkHalf(t *testing.T) {
	cases := map[string]string{
		"theme = dark:Catppuccin Mocha,light:Catppuccin Latte": "Catppuccin Mocha",
		"theme = light:Catppuccin Latte,dark:Catppuccin Mocha": "Catppuccin Mocha",
		"theme = tokyonight":  "tokyonight",
		`theme = "Rosé Pine"`: "Rosé Pine",
		// A pair with only a light side still says more than nothing.
		"theme = light:Catppuccin Latte": "Catppuccin Latte",
		"theme =":                        "",
	}
	for line, want := range cases {
		if got := Parse(strings.NewReader(line)).Theme; got != want {
			t.Errorf("%s -> %q, want %q", line, got, want)
		}
	}
}

// TestParsePaddingTakesTheLargerHalf: Ghostty's padding may be a left,right
// pair, and codeshot draws one padding per side. The smaller number is the
// one that would let text run close to the edge.
func TestParsePaddingTakesTheLargerHalf(t *testing.T) {
	cfg := Parse(strings.NewReader("window-padding-x = 8,16\nwindow-padding-y = 4"))
	if cfg.PaddingX != 16 || cfg.PaddingY != 4 {
		t.Errorf("padding = %d,%d, want 16,4", cfg.PaddingX, cfg.PaddingY)
	}
}

// TestParseLeavesNonsenseAlone: this file is not codeshot's, and a key it
// happens to read carrying a value it cannot use is not a reason to refuse
// to take a picture.
func TestParseLeavesNonsenseAlone(t *testing.T) {
	cfg := Parse(strings.NewReader("font-size = enormous\nwindow-padding-x = ??\nfont-size\n"))
	if cfg.FontSize != 0 || cfg.PaddingX != 0 {
		t.Errorf("cfg = %+v, want unusable values left unset", cfg)
	}
}

func TestLoadFindsTheXDGConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "ghostty", "config")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("theme = tokyonight\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, found := Load()
	if !found {
		t.Fatal("Load did not find the config under XDG_CONFIG_HOME")
	}
	if cfg.Theme != "tokyonight" {
		t.Errorf("Theme = %q", cfg.Theme)
	}
	if cfg.Path != path {
		t.Errorf("Path = %q, want %q so doctor can name it", cfg.Path, path)
	}
}

// TestLoadIsQuietWithNoGhostty is the whole contract of this package: a
// machine that has never had Ghostty on it is the ordinary case.
func TestLoadIsQuietWithNoGhostty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if cfg, found := Load(); found {
		t.Errorf("Load found %+v where there is no Ghostty", cfg)
	}
}
