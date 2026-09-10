// Package file reads a saved ANSI dump. It is the simplest CaptureSource there
// is, and the one that lets the whole pipeline be exercised without a terminal.
package file

import (
	"fmt"
	"os"

	"codeshot/internal/domain"
)

type Source struct {
	Path    string
	Command string
	Cwd     string
	Cols    int
	Rows    int
}

func (s Source) Capture() (domain.Capture, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return domain.Capture{}, fmt.Errorf("reading %s: %w", s.Path, err)
	}
	cwd := s.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return domain.Capture{
		Command: s.Command,
		Cwd:     cwd,
		Cols:    s.Cols,
		Rows:    s.Rows,
		Bytes:   data,
	}, nil
}
