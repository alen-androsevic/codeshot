package pty

import (
	"bytes"
	"os"
	"os/exec"
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
	// An alias is the commonest reason to land here, and it is invisible to
	// codeshot by construction: the shim is what can see it.
	if !strings.Contains(err.Error(), "alias") {
		t.Errorf("error = %q, want it to mention an alias", err)
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

func stdinFrom(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// runWithin fails the test instead of hanging it when a child waits forever
// on a stdin that will never end.
func runWithin(t *testing.T, src Source) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		got, _ := run(t, src)
		done <- got
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("the child never saw the end of its stdin")
		return ""
	}
}

// TestCaptureHandsANonTerminalStdinStraightOver: `codeshot -- cmd < file`
// and `producer | codeshot -- cmd` give the child their stdin directly, not
// through the pty. A pty's line discipline is for a person typing - it
// echoes, it turns a 0x03 byte into SIGINT, and on macOS it truncates any
// line past 1024 bytes - and a file is not a person typing. The input must
// not appear in the picture, and must arrive whole.
func TestCaptureHandsANonTerminalStdinStraightOver(t *testing.T) {
	got := runWithin(t, Source{Argv: []string{"cat"}, Stdin: stdinFrom(t, "hello\n")})
	if got != "hello\r\n" {
		t.Errorf("Bytes = %q, want cat's output once and no echo of its input", got)
	}

	long := strings.Repeat("a", 5000)
	got = runWithin(t, Source{Argv: []string{"sh", "-c", "wc -c | tr -d ' '"}, Stdin: stdinFrom(t, long)})
	if got != "5000\r\n" {
		t.Errorf("wc -c = %q, want all 5000 bytes of a line with no newline", got)
	}

	got = runWithin(t, Source{Argv: []string{"sh", "-c", "wc -c | tr -d ' '"}, Stdin: stdinFrom(t, "a\x03b\x04c")})
	if got != "5\r\n" {
		t.Errorf("wc -c = %q, want control bytes delivered as data, not as signals or EOF", got)
	}
}

// TestCaptureKeepsTheChildsOutputOnThePty: with stdin handed over directly,
// stdout and stderr are still the terminal, which is what decides colour.
func TestCaptureKeepsTheChildsOutputOnThePty(t *testing.T) {
	got := runWithin(t, Source{Argv: []string{"sh", "-c", `[ -t 1 ] && [ -t 2 ] && echo tty`}, Stdin: stdinFrom(t, "")})
	if got != "tty\r\n" {
		t.Errorf("Bytes = %q; the child's output is not on a terminal", got)
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

func TestCaptureCommandOverridesTheCommandLine(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{Argv: []string{"echo", "--token=s3cret"}, Command: "deploy", Stdout: &out}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Command != "deploy" {
		t.Errorf("Command = %q, want the override", c.Command)
	}
}

// sttySettings is the terminal's full configuration as `stty -g` prints it,
// which is an exact enough fingerprint to tell whether raw mode was undone.
func sttySettings(t *testing.T, f *os.File) string {
	t.Helper()
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = f
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestCaptureForwardsKeystrokesAndRestoresTheTerminal is the interactive
// path: a user at a real terminal types to the child through codeshot, and
// gets their terminal back exactly as it was when the child is done.
func TestCaptureForwardsKeystrokesAndRestoresTheTerminal(t *testing.T) {
	keyboard, userTty, err := creack.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer keyboard.Close()
	defer userTty.Close()
	before := sttySettings(t, userTty)

	w := &signalledWriter{first: make(chan struct{})}
	done := make(chan error, 1)
	var got string
	go func() {
		c, err := Source{
			Argv:   []string{"sh", "-c", `echo ready; read line; echo "got $line"`},
			Stdin:  userTty,
			Stdout: w,
		}.Capture()
		got = string(c.Bytes)
		done <- err
	}()
	select {
	case <-w.first:
	case <-time.After(5 * time.Second):
		t.Fatal("the child never started")
	}
	// Without this, a Raw that did nothing would pass the restore check
	// below for free.
	if during := sttySettings(t, userTty); during == before {
		t.Error("the terminal is not in raw mode while the child runs")
	}
	// Enter sends a carriage return, not a newline; the child's own line
	// discipline is what turns it into the end of a line.
	keyboard.Write([]byte("hi\r"))

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the keystrokes never reached the child")
	}
	if !strings.HasSuffix(got, "got hi\r\n") {
		t.Errorf("Bytes = %q, want the child to have read what was typed", got)
	}
	if after := sttySettings(t, userTty); after != before {
		t.Errorf("terminal left as %q, want it restored to %q", after, before)
	}
}
