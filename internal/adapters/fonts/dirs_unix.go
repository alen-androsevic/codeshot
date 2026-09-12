//go:build !darwin && !windows

package fonts

import (
	"os"
	"path/filepath"
)

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
