// Package pipe reads a command's output from standard input:
// `npm test | codeshot shot.png`.
//
// This is the honestly degraded mode. A pipe is not a terminal, so the
// command on the left already decided it was not talking to one and most
// tools drop their colour before codeshot ever sees a byte; nothing here can
// put it back. codeshot also cannot know what the command was, which is what
// the shell shims in shim/ and --command are for. The wrapper is the answer;
// this is the fallback for a pipeline already written.
package pipe

import (
	"fmt"
	"io"
	"os"

	"codeshot/internal/adapters/capture/tap"
	"codeshot/internal/adapters/tty"
	"codeshot/internal/domain"
)

type Source struct {
	// In is the stream to read to end-of-file.
	In io.Reader
	// Command is the command line the prompt shows. A pipe cannot know it,
	// so it is empty unless --command or a shell shim supplied it, and the
	// prompt line is left out when it is.
	Command string
	// Cwd is the directory the prompt shows; empty means the real one.
	Cwd string
	// Cols fixes the width. Zero follows the terminal behind Size.
	Cols int
	// Stdout is where the input passes through on its way past, so that a
	// pipeline still shows its output. Nil captures silently.
	Stdout io.Writer
	// Size is the terminal whose width the grid takes when Cols is unset.
	Size *os.File
}

func (s Source) Capture() (domain.Capture, error) {
	if s.In == nil {
		return domain.Capture{}, fmt.Errorf("no input to read")
	}
	cwd := s.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	// The width comes from the terminal the user is sitting at, the same as
	// wrapper mode, so the picture matches the window it came from. The
	// producing command wrapped nothing itself - it was writing to a pipe -
	// so this is the only width in the story.
	cols, rows := tty.Size(s.Size)
	if s.Cols > 0 {
		cols = s.Cols
	}

	captured := tap.New(s.Stdout)
	if _, err := io.Copy(captured, s.In); err != nil {
		return domain.Capture{}, fmt.Errorf("reading standard input: %w", err)
	}
	return domain.Capture{
		Command: s.Command,
		Cwd:     cwd,
		Cols:    cols,
		Rows:    rows,
		Bytes:   captured.Bytes(),
	}, nil
}
