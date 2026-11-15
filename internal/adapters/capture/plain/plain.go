// Package plain turns what a command wrote to codeshot's pty back into what
// it would have written without codeshot in front of it, for a passthrough
// that is not a terminal.
//
// Wrapper mode hands the child a terminal so the picture has colour, and the
// child believes it: `ls -G` colours its names and the pty's line discipline
// turns every \n into \r\n. That is right for the picture and for a person
// watching, and wrong for `codeshot -- ls -G > out.txt`, where bare `ls -G`
// would have written plain names and bare newlines. A file or a pipe gets no
// escape codes, the same as from any other well-behaved command.
package plain

import "io"

type state int

const (
	ground   state = iota
	esc            // after ESC
	escInter       // ESC followed by intermediate bytes, waiting for the final
	csi            // ESC [ ... until a final byte
	osc            // ESC ] ... until BEL or ST
	str            // ESC P, X, ^ or _ ... until ST
	strEsc         // ESC inside an OSC or a string: ST, or a new sequence
)

// Writer strips escape sequences and the pty's \r\n on the way to W. It keeps
// its state between writes, because a sequence or a \r\n can be split across
// two reads of the pty.
type Writer struct {
	W     io.Writer
	state state
	cr    bool // a \r is held back until the next byte says whether it was \r\n
}

func (w *Writer) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p))
	for _, c := range p {
		out = w.step(out, c)
	}
	if len(out) > 0 {
		if _, err := w.W.Write(out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Flush writes a \r still held back at the end of the output.
func (w *Writer) Flush() error {
	if !w.cr {
		return nil
	}
	w.cr = false
	_, err := w.W.Write([]byte{'\r'})
	return err
}

func (w *Writer) step(out []byte, c byte) []byte {
	switch w.state {
	case esc:
		switch {
		case c == '[':
			w.state = csi
		case c == ']':
			w.state = osc
		case c == 'P' || c == 'X' || c == '^' || c == '_':
			w.state = str
		case c >= 0x20 && c <= 0x2F:
			w.state = escInter
		default:
			w.state = ground
		}
		return out
	case escInter:
		if c < 0x20 || c > 0x2F {
			w.state = ground
		}
		return out
	case csi:
		if c >= 0x40 && c <= 0x7E {
			w.state = ground
		}
		return out
	case osc:
		switch c {
		case 0x07:
			w.state = ground
		case 0x1B:
			w.state = strEsc
		}
		return out
	case str:
		if c == 0x1B {
			w.state = strEsc
		}
		return out
	case strEsc:
		if c == '\\' {
			w.state = ground
			return out
		}
		// A bare ESC ends the string and starts a sequence of its own.
		w.state = esc
		return w.step(out, c)
	}

	if w.cr {
		w.cr = false
		if c != '\n' {
			out = append(out, '\r')
		}
	}
	switch c {
	case 0x1B:
		w.state = esc
	case '\r':
		w.cr = true
	default:
		out = append(out, c)
	}
	return out
}
