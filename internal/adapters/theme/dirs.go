package theme

import (
	"os"
	"path/filepath"
)

// ghosttyThemeDirs are the places a Ghostty installation keeps the several
// hundred themes it ships. Naming them is what makes `--theme tokyonight`
// work without codeshot shipping copies of other projects' palettes under
// their names - a hand-typed "catppuccin-mocha" that drifts from the real
// one is worse than not having it (design §7).
var ghosttyThemeDirs = []string{
	"/Applications/Ghostty.app/Contents/Resources/ghostty/themes",
	"/opt/homebrew/share/ghostty/themes",
	"/usr/local/share/ghostty/themes",
	"/usr/share/ghostty/themes",
}

// GhosttyDirs is the subset of those directories that exist here. Searching a
// directory that is not there costs nothing, but reporting the list - which
// `codeshot doctor` does - is only useful if it says what was actually found.
func GhosttyDirs() []string {
	var found []string
	for _, dir := range ghosttyThemeDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			found = append(found, dir)
		}
	}
	return found
}

// Installed lists the theme names available from every source, embedded
// first, with duplicates dropped: a name found in two directories resolves
// to the first, so it should be listed once. It is what `codeshot themes`
// prints, rather than the two names that subcommand used to hardcode.
func (s Source) Installed() []string {
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	if entries, err := assets.ReadDir("assets"); err == nil {
		for _, e := range entries {
			add(trimConf(e.Name()))
		}
	}
	for _, dir := range s.Dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				add(e.Name())
			}
		}
	}
	return names
}

func trimConf(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
