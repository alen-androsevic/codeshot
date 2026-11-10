// Package trash moves a file to the platform's trash, where it can be put back.
// It is what lets an explicit name replace an existing shot without the old
// one being gone for good: undo beats confirm, and codeshot never prompts.
package trash

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Move puts path in the trash and returns where it went.
func Move(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return move(abs)
}

// free is a name in dir that nothing is using yet: the file's own, then
// "shot 2.png", "shot 3.png", the way Finder steps around a name.
func free(dir, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	candidate := name
	for n := 2; ; n++ {
		if _, err := os.Lstat(filepath.Join(dir, candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = fmt.Sprintf("%s %d%s", stem, n, ext)
	}
}

// rename moves from to to, copying when they are on different volumes, which
// a file on an external disk and a trash in the home directory can be.
func rename(from, to string) error {
	err := os.Rename(from, to)
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(to)
		return err
	}
	if err := dst.Close(); err != nil {
		os.Remove(to)
		return err
	}
	return os.Remove(from)
}
