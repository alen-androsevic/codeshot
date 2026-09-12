package fonts

import (
	"os"
	"path/filepath"
)

// fallbackFamilies are consulted in order for a rune the chosen family has
// no glyph for: monospace first, then the faces Windows ships that carry
// symbols and CJK.
func fallbackFamilies() []string {
	return []string{
		"Consolas",
		"Cascadia Mono",
		"Courier New",
		"Segoe UI Symbol",
		"Microsoft YaHei",
		"Arial Unicode MS",
	}
}

// systemFontDirs is one of design §5's four per-OS seams. Windows is not a
// platform codeshot is tested on - there is no ConPTY capture yet - but the
// font index has no reason to be the thing that stops it building.
func systemFontDirs() []string {
	var dirs []string
	if root := os.Getenv("SystemRoot"); root != "" {
		dirs = append(dirs, filepath.Join(root, "Fonts"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
	}
	return dirs
}
