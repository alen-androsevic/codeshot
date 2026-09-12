package fonts

import (
	"os"
	"path/filepath"
)

// systemFontDirs is one of design §5's four per-OS seams. macOS keeps its
// own faces in /System/Library/Fonts, the several hundred it installs
// alongside them in Supplemental, and anything a person or an installer
// added in the two Library/Fonts directories.
func systemFontDirs() []string {
	dirs := []string{
		"/System/Library/Fonts",
		"/System/Library/Fonts/Supplemental",
		"/Library/Fonts",
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
	}
	return dirs
}
