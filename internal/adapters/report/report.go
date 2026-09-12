// Package report writes codeshot's one line of output. It goes to stderr, so
// that a wrapper's passthrough on stdout stays exactly what the command
// printed.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type Writer struct {
	Err io.Writer
}

// Stored says where the picture went, unless there is nowhere to name:
// --stdout sends the PNG to whatever is reading stdout, and there is no file
// to report.
func (w Writer) Stored(path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(w.Err, "Stored codeshot in %s\n", tildify(path))
}

func (w Writer) Warn(msg string) {
	fmt.Fprintf(w.Err, "codeshot: %s\n", msg)
}

func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
