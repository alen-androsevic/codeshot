package tty

import (
	"os"
	"path/filepath"
	"testing"
)

// notATerminal is what codeshot's stderr looks like under `2> log`, in CI,
// or from inside `go test`: an ordinary file.
func notATerminal(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-tty"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestSizeFallsBackTo100By24 pins design §8: "without a tty it is 100×24".
// A redirected stderr is an ordinary thing to do in front of a wrapper, and
// it must never be an error - it only means codeshot has to guess.
func TestSizeFallsBackTo100By24(t *testing.T) {
	cols, rows := Size(notATerminal(t))
	if cols != 100 || rows != 24 {
		t.Errorf("Size = %dx%d, want the 100x24 fallback", cols, rows)
	}
}

func TestIsTerminalSaysNoToAFile(t *testing.T) {
	if IsTerminal(notATerminal(t)) {
		t.Error("IsTerminal said an ordinary file was a terminal")
	}
}

// TestRawIsANoOpOffATerminal matters because stdin is very often not a
// terminal - `codeshot -- cmd < input.txt`, or any CI job - and raw mode is
// something to do to a terminal, not a precondition for running. The restore
// must still be safe to call, so callers can defer it unconditionally.
func TestRawIsANoOpOffATerminal(t *testing.T) {
	restore, err := Raw(notATerminal(t))
	if err != nil {
		t.Fatalf("Raw on a file = %v, want a quiet no-op", err)
	}
	restore()
	restore()
}

// TestANilFileIsNotATerminal lets a caller that has no *os.File to offer -
// the CLI's tests hand it a bytes.Buffer for stderr - pass nil and get the
// fallback, rather than the size of whatever terminal is running the suite.
func TestANilFileIsNotATerminal(t *testing.T) {
	if cols, rows := Size(nil); cols != 100 || rows != 24 {
		t.Errorf("Size(nil) = %dx%d, want the fallback", cols, rows)
	}
	if IsTerminal(nil) {
		t.Error("IsTerminal(nil) = true")
	}
	restore, err := Raw(nil)
	if err != nil {
		t.Fatal(err)
	}
	restore()
}
