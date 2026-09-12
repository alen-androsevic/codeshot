// Package gallery keeps shots on disk. A bare name lands in the gallery
// directory; anything with a separator in it is a path the caller chose, and
// is honoured as given.
package gallery

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type FS struct {
	Dir string
}

func (g FS) path(name string) string {
	if strings.ContainsRune(name, filepath.Separator) {
		return name
	}
	return filepath.Join(g.Dir, name)
}

func (g FS) Exists(name string) bool {
	_, err := os.Stat(g.path(name))
	return err == nil
}

func (g FS) Store(name string, img image.Image) (string, error) {
	path := g.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return "", fmt.Errorf("encoding %s: %w", path, err)
	}
	// Close is where a buffered write finally reaches the disk, so its error
	// is the one that says the file is not what it should be. It is called
	// exactly once: this used to be deferred as well as returned, which
	// closed the file twice and reported whatever the second close said.
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", path, err)
	}
	return path, nil
}
