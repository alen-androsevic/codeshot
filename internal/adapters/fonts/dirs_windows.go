package fonts

import (
	"os"
	"path/filepath"
)

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
