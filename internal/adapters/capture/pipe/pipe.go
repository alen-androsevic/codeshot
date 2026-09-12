// Package pipe reads a command's output from standard input:
// `npm test | codeshot shot.png`.
//
// This is the honestly degraded mode. A pipe is not a terminal, so the
// command on the left already decided it was not talking to one and most
// tools drop their colour before codeshot ever sees a byte; nothing here can
// put it back. codeshot also cannot know what the command was, which is what
// the shell shims in shim/ and --command are for. A pipe carries standard
// output alone, so a command that writes to standard error - jest, and most
// test runners - sends the interesting half somewhere codeshot never sees.
// The wrapper has none of these problems. Prefer it when you can.
package pipe

import (
	"bytes"
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
		Bytes:   onlcr(captured.Bytes()),
	}, nil
}

// onlcr turns every lone \n into \r\n, which is the one thing a pipe loses
// that can be given back.
//
// A terminal driver does this on the way out: a program prints \n, the line
// discipline's ONLCR flag turns it into \r\n, and the terminal both drops a
// row and returns to column one. Down a pipe there is no driver, so the \n
// arrives bare and the emulator reads it the only way a lone \n can be read -
// down one row, stay in this column - and the picture stair-steps.
//
// The file source faces the same bytes and only warns, because a saved dump
// might be a genuine pty capture where a lone \n really did mean "stay in
// this column". Here there is no such doubt. The bytes never met a terminal
// driver, so applying what the driver would have applied is not a guess.
//
// A \n that already has its \r is left alone. Doubling it would render the
// same, but the capture is what --debug and any saved dump show, and it
// should hold what the command would have written to a terminal.
func onlcr(b []byte) []byte {
	if bytes.IndexByte(b, '\n') == -1 {
		return b
	}
	out := make([]byte, 0, len(b)+bytes.Count(b, []byte("\n")))
	for i, c := range b {
		if c == '\n' && (i == 0 || b[i-1] != '\r') {
			out = append(out, '\r')
		}
		out = append(out, c)
	}
	return out
}
