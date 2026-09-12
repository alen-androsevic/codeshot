// Package tap keeps a copy of everything on its way past. Both capture
// sources that watch a live stream - the pty wrapper and the pipe - have to
// hand the user their output as it arrives and keep it for the picture, which
// is the same job twice.
package tap

import (
	"bytes"
	"io"
)

// Tap writes everything to a buffer and passes it through to out, until out
// fails. A passthrough that has stopped accepting output must not stop the
// capture: in wrapper mode the child would block on a full pty and never
// finish, and in either mode the picture is still owed. A nil out captures
// silently.
type Tap struct {
	out    io.Writer
	buf    bytes.Buffer
	broken bool
}

func New(out io.Writer) *Tap {
	return &Tap{out: out}
}

func (t *Tap) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if !t.broken && t.out != nil {
		if _, err := t.out.Write(p); err != nil {
			t.broken = true
		}
	}
	return len(p), nil
}

// Bytes is everything that went past.
func (t *Tap) Bytes() []byte { return t.buf.Bytes() }
