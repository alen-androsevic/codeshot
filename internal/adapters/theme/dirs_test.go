package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGhosttyDirsOnlyNamesWhatExists(t *testing.T) {
	for _, dir := range GhosttyDirs() {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("GhosttyDirs listed %q, which is not a directory here", dir)
		}
	}
}

func TestInstalledListsEmbeddedThemesFirst(t *testing.T) {
	names := Source{}.Installed()
	if len(names) < 2 || names[0] != "codeshot-dark" || names[1] != "codeshot-light" {
		t.Fatalf("Installed = %v, want the two embedded themes", names)
	}
	for _, name := range names {
		if strings.HasSuffix(name, ".conf") {
			t.Errorf("Installed reported %q with its extension", name)
		}
	}
}

func TestInstalledAddsDirectoriesAndDropsDuplicates(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tokyonight", "Catppuccin Mocha"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("background=#000000\nforeground=#ffffff\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A second directory offering a name the first already did: the lookup
	// resolves to one file, so the listing should say the name once.
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(second, "tokyonight"), []byte("background=#000000\nforeground=#ffffff\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	names := Source{Dirs: []string{dir, second}}.Installed()
	counts := map[string]int{}
	for _, name := range names {
		counts[name]++
	}
	if counts["tokyonight"] != 1 {
		t.Errorf("tokyonight listed %d times, want once", counts["tokyonight"])
	}
	if counts["Catppuccin Mocha"] != 1 {
		t.Errorf("Installed = %v, want a theme name with a space in it", names)
	}
	if counts["codeshot-dark"] != 1 {
		t.Errorf("Installed = %v, want the embedded themes still there", names)
	}
}

// TestThemePrecedence pins the order design §7 implies and phase 3 leans on:
// embedded first, then the theme directories, then the name as a path. It was
// untested until a Ghostty directory was layered onto exactly that ordering.
func TestThemePrecedence(t *testing.T) {
	dir := t.TempDir()
	// A directory entry that shadows an embedded name, and one that shadows
	// a file in the working directory.
	write := func(path, bg string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("background = "+bg+"\nforeground = #ffffff\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "codeshot-dark"), "#111111")
	write(filepath.Join(dir, "shared"), "#222222")

	onDisk := filepath.Join(t.TempDir(), "shared")
	write(onDisk, "#333333")

	src := Source{Dirs: []string{dir}}

	// Embedded beats a directory of the same name.
	th, err := src.Theme("codeshot-dark")
	if err != nil {
		t.Fatal(err)
	}
	if th.Background != Default().Background {
		t.Errorf("Background = %v, want the embedded theme to beat a directory of the same name", th.Background)
	}

	// A directory beats the name read as a path.
	th, err = src.Theme("shared")
	if err != nil {
		t.Fatal(err)
	}
	if th.Background.R != 0x22 {
		t.Errorf("Background = %v, want the theme directory's copy", th.Background)
	}

	// A path is the last resort, and still works.
	th, err = src.Theme(onDisk)
	if err != nil {
		t.Fatal(err)
	}
	if th.Background.R != 0x33 {
		t.Errorf("Background = %v, want the file named as a path", th.Background)
	}
}
