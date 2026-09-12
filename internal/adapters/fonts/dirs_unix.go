//go:build !darwin && !windows

package fonts

import (
	"os"
	"path/filepath"
)

// fallbackFamilies are consulted in order for a rune the chosen family has
// no glyph for: monospace first, so a missing box-drawing or arrow glyph
// keeps the grid's rhythm, then the Noto faces that carry almost everything
// else. Colour emoji fonts are deliberately absent - they are bitmaps with
// no outlines to draw, and have their own path.
func fallbackFamilies() []string {
	return []string{
		"DejaVu Sans Mono",
		"Liberation Mono",
		"Noto Sans Mono",
		"FreeMono",
		"Noto Sans",
		"Noto Sans CJK SC",
		"Noto Sans Symbols 2",
		"DejaVu Sans",
		"Unifont",
	}
}

// systemFontDirs is one of design §5's four per-OS seams. These are
// fontconfig's usual places, in the order it searches them: the system's
// own, the local administrator's, and the user's.
func systemFontDirs() []string {
	dirs := []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "fonts"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		if os.Getenv("XDG_DATA_HOME") == "" {
			dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"))
		}
		dirs = append(dirs, filepath.Join(home, ".fonts"))
	}
	return dirs
}
