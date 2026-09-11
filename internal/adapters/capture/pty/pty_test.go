package pty

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	creack "github.com/creack/pty"
)

// notATerminal stands in for a redirected stderr: something to size from
// that has no size, so the pty gets design §8's 100x24.
func notATerminal(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-tty"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func run(t *testing.T, src Source) (string, string) {
	t.Helper()
	var out bytes.Buffer
	if src.Stdout == nil {
		src.Stdout = &out
	}
	if src.Size == nil {
		src.Size = notATerminal(t)
	}
	c, err := src.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	return string(c.Bytes), out.String()
}

// TestCaptureGoesThroughARealPty is the whole reason for this package. The
// terminal driver turns \n into \r\n on the way out of a pty, and a plain
// pipe never does - which is exactly the stair-stepping `render` warns about.
// Seeing \r\n come back is proof the bytes crossed a line discipline.
func TestCaptureGoesThroughARealPty(t *testing.T) {
	got, _ := run(t, Source{Argv: []string{"printf", `a\nb\n`}})
	if got != "a\r\nb\r\n" {
		t.Errorf("Bytes = %q, want the pty's \\r\\n translation", got)
	}
}

func TestCaptureTheChildSeesATerminal(t *testing.T) {
	got, _ := run(t, Source{Argv: []string{"sh", "-c", `[ -t 0 ] && [ -t 1 ] && [ -t 2 ] && echo tty`}})
	if got != "tty\r\n" {
		t.Errorf("Bytes = %q; the child's stdio is not a terminal, so tools will drop their colour", got)
	}
}

// TestCapturePassesThroughWhatItCaptures pins the wrapper's transparency: the
// user watches exactly the bytes that end up in the picture, as they arrive.
func TestCapturePassesThroughWhatItCaptures(t *testing.T) {
	got, passed := run(t, Source{Argv: []string{"sh", "-c", `printf '\033[31mred\033[0m\n'`}})
	if passed != got {
		t.Errorf("passthrough %q, captured %q; they must be the same bytes", passed, got)
	}
	if !strings.Contains(got, "\x1b[31mred") {
		t.Errorf("Bytes = %q, want the escape sequence intact", got)
	}
}

func TestCaptureSizesThePtyFromTheSizeFile(t *testing.T) {
	got, _ := run(t, Source{Argv: []string{"stty", "size"}})
	if got != "24 100\r\n" {
		t.Errorf("stty size = %q, want the 100x24 fallback", got)
	}
}

// TestCaptureHonoursAnExplicitWidth is --cols. The height is left alone:
// it only affects how a TUI lays itself out, and --rows means crop.
func TestCaptureHonoursAnExplicitWidth(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"stty", "size"}, Cols: 80, Stdout: &out, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if string(c.Bytes) != "24 80\r\n" {
		t.Errorf("stty size = %q, want 80 columns", c.Bytes)
	}
	if c.Cols != 80 || c.Rows != 24 {
		t.Errorf("Capture is %dx%d, want the emulator told 80x24", c.Cols, c.Rows)
	}
}

func TestCaptureSetsTermForTheEmulator(t *testing.T) {
	got, _ := run(t, Source{
		Argv: []string{"sh", "-c", `echo "$TERM $COLORTERM"`},
		Env:  []string{"PATH=" + os.Getenv("PATH"), "TERM=xterm-ghostty"},
	})
	if got != "xterm-256color truecolor\r\n" {
		t.Errorf("env = %q", got)
	}
}

func TestCaptureRecordsTheCommandLine(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"echo", "fix the bug", "it's", "a-b"}, Stdout: &out, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if want := `echo 'fix the bug' 'it'\''s' a-b`; c.Command != want {
		t.Errorf("Command = %s, want %s", c.Command, want)
	}
}

func TestCaptureReportsTheChildsExitCode(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"sh", "-c", "exit 3"}, Stdout: &out, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatalf("Capture: %v; a failing child is still a capture", err)
	}
	if c.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", c.ExitCode)
	}
}

// TestCaptureReportsASignalTheWayAShellDoes: 128 plus the signal number, so
// that `codeshot -- cmd; echo $?` says what `cmd; echo $?` would have.
func TestCaptureReportsASignalTheWayAShellDoes(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"sh", "-c", "kill -TERM $$"}, Stdout: &out, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.ExitCode != 128+int(syscall.SIGTERM) {
		t.Errorf("ExitCode = %d, want %d", c.ExitCode, 128+int(syscall.SIGTERM))
	}
}

func TestCaptureOfAMissingCommandIs127(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"codeshot-no-such-command"}, Stdout: &out, Size: notATerminal(t)}.Capture()
	if err == nil {
		t.Fatal("Capture succeeded at running a command that does not exist")
	}
	if !strings.Contains(err.Error(), "codeshot-no-such-command") {
		t.Errorf("error = %q, want the command named", err)
	}
	if c.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127 as a shell would", c.ExitCode)
	}
}

func TestCaptureOfNothingIsAnError(t *testing.T) {
	var out bytes.Buffer
	if _, err := (Source{Stdout: &out, Size: notATerminal(t)}).Capture(); err == nil {
		t.Error("Capture ran an empty argv")
	}
}

// TestCaptureForwardsStdinAndItsEnd: a command reading stdin gets it, and
// when stdin runs out the child sees end-of-file rather than waiting forever
// on a terminal that will never type again. A pty has no "close the write
// end"; a ^D at the start of a line is how a terminal says it.
func TestCaptureForwardsStdinAndItsEnd(t *testing.T) {
	in := filepath.Join(t.TempDir(), "in")
	if err := os.WriteFile(in, []byte("hello\npartial"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	done := make(chan string, 1)
	go func() {
		got, _ := run(t, Source{Argv: []string{"sh", "-c", `wc -l | tr -d ' '`}, Stdin: stdin})
		done <- got
	}()
	select {
	case got := <-done:
		// The line discipline echoes what it is fed, so the input appears
		// too; what matters is that wc saw both lines end and counted one
		// newline, which it only prints after end-of-file.
		if !strings.HasSuffix(got, "1\r\n") {
			t.Errorf("Bytes = %q, want wc's count after the echoed input", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the child never saw end-of-file on its stdin")
	}
}

// signalledWriter tells the test when the child has printed something, which
// is how it knows the child's trap is in place before resizing under it.
type signalledWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	first chan struct{}
	once  sync.Once
}

func (w *signalledWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.once.Do(func() { close(w.first) })
	return w.buf.Write(p)
}

// TestCaptureForwardsAResize follows a SIGWINCH from the terminal codeshot
// runs in down to the child, which is what keeps a TUI laid out for the
// window the user is actually looking at. The capture reports the final size,
// because that is the layout a TUI's last frame was drawn for.
func TestCaptureForwardsAResize(t *testing.T) {
	// A pty pair plays the user's terminal, so there is a real size to read
	// and change.
	userPty, userTty, err := creack.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer userPty.Close()
	defer userTty.Close()
	if err := creack.Setsize(userPty, &creack.Winsize{Rows: 30, Cols: 90}); err != nil {
		t.Fatal(err)
	}

	w := &signalledWriter{first: make(chan struct{})}
	type result struct {
		cols, rows int
		bytes      string
		err        error
	}
	done := make(chan result, 1)
	go func() {
		c, err := Source{
			Argv:   []string{"sh", "-c", `trap 'stty size; exit' WINCH; stty size; while :; do sleep 0.05; done`},
			Stdout: w,
			Size:   userTty,
		}.Capture()
		done <- result{c.Cols, c.Rows, string(c.Bytes), err}
	}()

	select {
	case <-w.first:
	case <-time.After(5 * time.Second):
		t.Fatal("the child printed nothing")
	}
	if err := creack.Setsize(userPty, &creack.Winsize{Rows: 50, Cols: 120}); err != nil {
		t.Fatal(err)
	}
	syscall.Kill(os.Getpid(), syscall.SIGWINCH)

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.bytes != "30 90\r\n50 120\r\n" {
			t.Errorf("the child saw %q, want its first size then the new one", r.bytes)
		}
		if r.cols != 120 || r.rows != 50 {
			t.Errorf("Capture is %dx%d, want the final 120x50", r.cols, r.rows)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the resize never reached the child")
	}
}
