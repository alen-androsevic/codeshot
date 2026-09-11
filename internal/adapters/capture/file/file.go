// Package file reads a saved ANSI dump. It is the simplest CaptureSource there
// is, and the one that lets the whole pipeline be exercised without a terminal.
package file

import (
	"bytes"
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
	// Warn receives the one diagnostic a saved dump can earn: that it looks
	// like it was made by a plain shell redirect. It is optional, and a nil
	// Warn means nobody is listening.
	Warn func(string)
}

func (s Source) Capture() (domain.Capture, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return domain.Capture{}, fmt.Errorf("reading %s: %w", s.Path, err)
	}
	s.warnAboutMissingCarriageReturns(data)
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

// warnAboutMissingCarriageReturns says so when a dump looks redirected rather
// than captured, and stops there: the bytes handed on are the bytes on disk.
// Rewriting a lone \n into \r\n would corrupt a genuine capture that really
// does mean "move down, stay in this column" - codeshot renders what a pty
// master emitted, and a warning is as far as it goes to second-guess that.
//
// This check belongs to the file source rather than to the CLI, where it
// started. It is a fact about saved dumps, and neither a pty nor a pipe can
// produce this shape of mistake, so a CLI that owned it would have to ask
// which source it had built before deciding whether to look.
func (s Source) warnAboutMissingCarriageReturns(data []byte) {
	if s.Warn == nil || !looksLikeAPlainRedirect(data) {
		return
	}
	s.Warn(fmt.Sprintf(
		"%s has more than one line and no carriage returns. A plain `cmd > file` "+
			"redirect never passes through a pty, so the terminal driver never turns "+
			"\\n into \\r\\n, and this will stair-step: the cursor moves down but "+
			"never back to column one between lines. Capture through a pty instead, "+
			"e.g. `codeshot -- cmd`.", s.Path))
}

// looksLikeAPlainRedirect reports whether data has more than one line but not
// a single carriage return in it - the signature of a dump assembled by
// `cmd > file` rather than captured through a pty. A single line has no
// alignment to lose, so it is not flagged.
func looksLikeAPlainRedirect(data []byte) bool {
	if bytes.IndexByte(data, '\r') != -1 {
		return false
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	return len(lines) > 1
}
