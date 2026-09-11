// Package tty answers the questions wrapper mode has to ask of the terminal
// codeshot was started in: is this a terminal, how big is it, and can it be
// put into raw mode for as long as a child is running.
//
// It is not a port. Design §5 lists Tty among the driven ports, but nothing in
// internal/app ever asks for a terminal's size - only the CLI and the pty
// source do, both on the edge - so it is a plain package the edge uses. The
// per-OS work is golang.org/x/term's, which already carries the build tags.
package tty

import (
	"os"

	"golang.org/x/term"
)

// The size a pty gets when there is no terminal to copy one from. Design §8.
const (
	fallbackCols = 100
	fallbackRows = 24
)

func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Size is the character grid of the terminal behind f, or 100x24 when f is
// not a terminal. Not having one is never an error: it is what a redirected
// stream, a CI job and a test all look like, and it only means codeshot has
// to guess.
func Size(f *os.File) (cols, rows int) {
	cols, rows, err := term.GetSize(int(f.Fd()))
	if err != nil || cols <= 0 || rows <= 0 {
		return fallbackCols, fallbackRows
	}
	return cols, rows
}

// Raw puts the terminal behind f into raw mode and returns the function that
// puts it back. Off a terminal it does nothing and returns a restore that
// does nothing, so a caller can defer it unconditionally. The restore is safe
// to call more than once, which matters because a caller restoring on a
// signal and again on the way out would otherwise restore a state the first
// call already replaced.
func Raw(f *os.File) (restore func(), err error) {
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	done := false
	return func() {
		if done {
			return
		}
		done = true
		term.Restore(fd, old)
	}, nil
}
