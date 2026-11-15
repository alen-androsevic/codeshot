package trash

import (
	"os"
	"path/filepath"
)

// move uses ~/.Trash, the Trash Finder shows for the home volume.
func move(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	to := filepath.Join(dir, free(dir, filepath.Base(path)))
	return to, rename(path, to)
}
