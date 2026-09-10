package vt

import "codeshot/internal/domain"

// Adapter is the emulator behind the app.Emulator port. It is stateless: a
// Capture goes in, a fresh Emulator runs it, a Result comes out.
type Adapter struct {
	// Unknown, if set, receives every sequence the emulator ignored.
	Unknown func(seq string)
}

// Fallback size for a capture that never learned how wide its terminal was -
// a file on disk, or a pipe with no tty behind it.
const (
	fallbackCols = 100
	fallbackRows = 24
)

func (a Adapter) Emulate(c domain.Capture) (domain.Result, error) {
	cols, rows := c.Cols, c.Rows
	if cols <= 0 {
		cols = fallbackCols
	}
	if rows <= 0 {
		rows = fallbackRows
	}
	e := New(cols, rows)
	e.Unknown = a.Unknown
	if _, err := e.Write(c.Bytes); err != nil {
		return domain.Result{}, err
	}
	return e.Result(), nil
}
