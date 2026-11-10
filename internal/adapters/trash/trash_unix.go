//go:build !darwin

package trash

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// move follows the freedesktop.org trash spec, so the file manager can put
// the file back: the file goes in files/, and a .trashinfo in info/ says
// where it came from.
func move(path string) (string, error) {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		data = filepath.Join(home, ".local", "share")
	}
	files := filepath.Join(data, "Trash", "files")
	info := filepath.Join(data, "Trash", "info")
	for _, dir := range []string{files, info} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
	}
	name := free(files, filepath.Base(path))
	// The info file is written first and exclusively: it is how two programs
	// trashing the same name at once avoid taking the same slot.
	record, err := os.OpenFile(filepath.Join(info, name+".trashinfo"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(url.PathEscape(path), "%2F", "/")
	_, err = fmt.Fprintf(record, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", escaped, time.Now().Format("2006-01-02T15:04:05"))
	if cerr := record.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(record.Name())
		return "", err
	}
	to := filepath.Join(files, name)
	if err := rename(path, to); err != nil {
		os.Remove(record.Name())
		return "", err
	}
	return to, nil
}
