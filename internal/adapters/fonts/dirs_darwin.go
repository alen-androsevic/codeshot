package fonts

import (
	"os"
	"path/filepath"
)

// fallbackFamilies are consulted in order for a rune the chosen family has
// no glyph for: the monospace faces first, so that a missing box-drawing or
// arrow glyph keeps the grid's rhythm, then the broad-coverage faces that
// carry CJK and symbols. Apple Color Emoji is deliberately absent - it is a
// bitmap font with no outlines to draw, and colour emoji have their own path.
func fallbackFamilies() []string {
	return []string{
		"Menlo",
		"SF Mono",
		".SF NS Mono",
		"Andale Mono",
		"Courier New",
		"Apple Symbols",
		"Arial Unicode MS",
		"PingFang SC",
		"Hiragino Sans",
		"Songti SC",
	}
}

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
