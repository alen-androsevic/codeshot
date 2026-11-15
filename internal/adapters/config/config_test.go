package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# codeshot's own defaults
theme = Catppuccin Mocha
font = JetBrains Mono
font-size = 15
padding = 20
no-shadow = true
--scale = 3
gallery = "~/Pictures/Codeshots"

# a repeated key: the last one wins, and the key is listed once
theme = tokyonight
bare-flag
`

func TestParseReadsFlagNamesAndValues(t *testing.T) {
	cfg := Parse(strings.NewReader(sample))

	if v, _ := cfg.String("theme"); v != "tokyonight" {
		t.Errorf("theme = %q, want the last one to win", v)
	}
	if v, _ := cfg.String("font"); v != "JetBrains Mono" {
		t.Errorf("font = %q", v)
	}
	if v, ok := cfg.Float("font-size"); !ok || v != 15 {
		t.Errorf("font-size = %v,%v", v, ok)
	}
	if v, ok := cfg.Int("padding"); !ok || v != 20 {
		t.Errorf("padding = %v,%v", v, ok)
	}
	if v, ok := cfg.Bool("no-shadow"); !ok || !v {
		t.Errorf("no-shadow = %v,%v", v, ok)
	}
	// Leading dashes are tolerated: someone will copy a flag in verbatim.
	if v, ok := cfg.Int("scale"); !ok || v != 3 {
		t.Errorf("scale = %v,%v, want --scale read as scale", v, ok)
	}
	if v, _ := cfg.String("gallery"); v != "~/Pictures/Codeshots" {
		t.Errorf("gallery = %q, want the quotes stripped", v)
	}
}

func TestOrderListsEachKeyOnce(t *testing.T) {
	cfg := Parse(strings.NewReader(sample))
	seen := map[string]int{}
	for _, key := range cfg.Order {
		seen[key]++
	}
	if seen["theme"] != 1 {
		t.Errorf("theme appears %d times in Order, want once", seen["theme"])
	}
}

func TestBoolTakesWhatAPersonWouldWrite(t *testing.T) {
	cfg := Parse(strings.NewReader("a = yes\nb = off\nc = 1\nd = maybe\ne\n"))
	for key, want := range map[string]bool{"a": true, "b": false, "c": true, "e": true} {
		got, ok := cfg.Bool(key)
		if !ok || got != want {
			t.Errorf("%s = %v,%v want %v,true", key, got, ok, want)
		}
	}
	if _, ok := cfg.Bool("d"); ok {
		t.Error("Bool accepted \"maybe\"")
	}
	if _, ok := cfg.Bool("missing"); ok {
		t.Error("Bool invented a value")
	}
}

func TestConversionsRefuseNonsense(t *testing.T) {
	cfg := Parse(strings.NewReader("font-size = enormous\npadding =\n"))
	if _, ok := cfg.Float("font-size"); ok {
		t.Error("Float accepted a word")
	}
	if _, ok := cfg.Int("padding"); ok {
		t.Error("Int accepted an empty value")
	}
	if _, ok := cfg.String("padding"); ok {
		t.Error("String returned an empty value as present")
	}
}

// TestUnknownNamesTyposInCodeshotsOwnFile: this file is codeshot's, unlike
// Ghostty's, so a key it does not recognise is likely a mistake worth
// mentioning rather than a setting that is none of its business.
func TestUnknownNamesTyposInCodeshotsOwnFile(t *testing.T) {
	cfg := Parse(strings.NewReader("theme = x\nthme = y\n"))
	unknown := cfg.Unknown(map[string]bool{"theme": true})
	if len(unknown) != 1 || unknown[0] != "thme" {
		t.Errorf("Unknown = %v, want the misspelled key", unknown)
	}
}

func TestLoadFindsTheXDGConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "codeshot", "config")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("theme = tokyonight\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, _ := cfg.String("theme"); v != "tokyonight" {
		t.Errorf("theme = %q", v)
	}
	if cfg.Path != path {
		t.Errorf("Path = %q, want %q", cfg.Path, path)
	}
}

// TestLoadWithNoConfigIsQuiet: not having one is the ordinary case.
func TestLoadWithNoConfigIsQuiet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Values) != 0 || cfg.Path != "" {
		t.Errorf("Load invented %+v", cfg)
	}
}

// TestLoadReportsAMissingExplicitConfig: --config named a file, so its
// absence is a mistake, not the ordinary case.
func TestLoadReportsAMissingExplicitConfig(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Load accepted a --config that is not there")
	}
}

func TestExampleNamesTheDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if !strings.Contains(Example(), "/tmp/xdg/codeshot/config") {
		t.Errorf("Example = %q, want it to name where the file goes", Example())
	}
}
