// Package home contracts a path against the user's home directory, so that a
// prompt line and a "Stored codeshot in …" line read the way a person would
// write the path rather than the way the filesystem spells it.
//
// It exists because three packages had grown their own copy of the same
// twelve lines - the prompt, the report and doctor - and they had already
// begun to differ: one contracted an exact home to "~" and one did not.
package home

import (
	"os"
	"path/filepath"
	"strings"
)

// Tildify replaces the user's home directory with ~. A path outside it, or a
// machine with no home directory to speak of, comes back untouched.
func Tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	// The separator matters: /home/alenderson is not inside /home/alen.
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
