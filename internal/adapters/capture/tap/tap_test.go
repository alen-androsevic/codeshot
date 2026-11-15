package tap

import (
	"bytes"
	"errors"
	"testing"
)

func TestTapKeepsAndPassesOn(t *testing.T) {
	var out bytes.Buffer
	tap := New(&out)
	tap.Write([]byte("a\r\n"))
	tap.Write([]byte("b\r\n"))
	if got := string(tap.Bytes()); got != "a\r\nb\r\n" {
		t.Errorf("Bytes = %q", got)
	}
	if out.String() != "a\r\nb\r\n" {
		t.Errorf("passthrough = %q", out.String())
	}
}

type brokenWriter struct{ writes int }

func (w *brokenWriter) Write(p []byte) (int, error) {
	w.writes++
	return 0, errors.New("broken pipe")
}

// TestTapOutlivesABrokenPassthrough is the point of the type. `codeshot --
// ls | head -1` leaves nobody reading stdout, and a capture that stopped
// there would hang the child against a full pty and lose the picture that
// was the reason for running at all.
func TestTapOutlivesABrokenPassthrough(t *testing.T) {
	out := &brokenWriter{}
	tap := New(out)
	for _, s := range []string{"a", "b", "c"} {
		if n, err := tap.Write([]byte(s)); n != 1 || err != nil {
			t.Fatalf("Write(%q) = %d, %v; a broken passthrough is not the caller's problem", s, n, err)
		}
	}
	if got := string(tap.Bytes()); got != "abc" {
		t.Errorf("Bytes = %q, want everything kept", got)
	}
	if out.writes != 1 {
		t.Errorf("wrote to the broken passthrough %d times, want it given up after the first", out.writes)
	}
}

func TestTapWithNoPassthrough(t *testing.T) {
	tap := New(nil)
	tap.Write([]byte("quiet"))
	if got := string(tap.Bytes()); got != "quiet" {
		t.Errorf("Bytes = %q", got)
	}
}
