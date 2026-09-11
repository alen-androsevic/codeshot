// Package pty runs a command once, under a pseudo-terminal codeshot owns, and
// captures what it wrote. This is wrapper mode: `codeshot -- npm test`.
//
// The child gets a real terminal, so it keeps its colour, its progress bars
// and its idea of the window width. The user gets a normal run - output live
// as it happens, keystrokes reaching the child, resizes followed - and the
// same bytes they watched go into the picture.
package pty

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	creack "github.com/creack/pty"

	"codeshot/internal/adapters/tty"
	"codeshot/internal/domain"
)

// The environment the child is told it is running in. TERM is set even when
// the caller has one, because the terminal the child is really writing to is
// codeshot's emulator, not whichever one codeshot was started in: a TERM of
// xterm-ghostty sends the child to Ghostty's terminfo for sequences the
// emulator has never heard of. xterm-256color is what the emulator speaks,
// and every terminal a user might be watching from speaks it too.
const (
	term      = "xterm-256color"
	colorterm = "truecolor"
)

type Source struct {
	// Argv is the command and its arguments, exactly as given after --.
	Argv []string
	// Cwd is the directory the prompt line shows; empty means the real one.
	// The child always runs where codeshot was started.
	Cwd string
	// Cols fixes the pty's width. Zero follows the terminal behind Size.
	Cols int
	// Stdin is forwarded to the child, and put into raw mode for the length
	// of the run when it is a terminal. Nil forwards nothing.
	Stdin *os.File
	// Stdout is where the child's output passes through live.
	Stdout io.Writer
	// Size is the terminal the pty's size is copied from and whose resizes it
	// follows. The CLI hands over stderr: stdout is the one people redirect.
	Size *os.File
	// Env is the child's environment; nil means codeshot's own.
	Env []string
}

func (s Source) Capture() (domain.Capture, error) {
	if len(s.Argv) == 0 {
		return domain.Capture{}, errors.New("no command to run")
	}
	command := shellJoin(s.Argv)
	cwd := s.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	size := &windowSize{fixedCols: s.Cols}
	size.follow(s.Size)

	// The resize handler is installed before the child starts, so a resize in
	// the first instant of a run is not lost between the two.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)

	cmd := exec.Command(s.Argv[0], s.Argv[1:]...)
	cmd.Env = childEnv(s.Env)
	ptmx, err := creack.StartWithSize(cmd, size.winsize())
	if err != nil {
		return domain.Capture{ExitCode: startFailureCode(err)}, fmt.Errorf("%s: %w", s.Argv[0], unwrapStart(err))
	}
	defer ptmx.Close()

	// The child is the leader of a session of its own, on the pty, so the
	// signals a user sends codeshot's way do not reach it by themselves: a
	// ^C typed while stdin is not in raw mode lands on codeshot. Passing them
	// on to the child's process group means codeshot outlives them, which is
	// what lets it restore the terminal and still take the picture of an
	// interrupted run.
	forward := make(chan os.Signal, 1)
	signal.Notify(forward, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(forward)

	// The goroutine is waited for, not just told to stop, before ptmx closes:
	// a resize it is part-way through still has ptmx's descriptor in hand.
	stop, stopped := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-stopped }()
	go func() {
		defer close(stopped)
		for {
			select {
			case <-winch:
				size.follow(s.Size)
				creack.Setsize(ptmx, size.winsize())
			case sig := <-forward:
				syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
			case <-stop:
				return
			}
		}
	}()

	if s.Stdin != nil {
		// Raw mode is what lets every keystroke - ^C, arrows, a bare
		// letter - reach the child untranslated, with the pty's own line
		// discipline doing the cooking. Failing to get it is not worth
		// failing the run for; the child still runs, only less
		// interactively.
		restore, _ := tty.Raw(s.Stdin)
		defer restore()
		go forwardStdin(ptmx, s.Stdin)
	}

	var captured bytes.Buffer
	_, err = io.Copy(&tee{out: s.Stdout, buf: &captured}, ptmx)
	// A pty master reports EIO once the last holder of the other end is gone.
	// That is how a pty says end-of-output, not a failure.
	if err != nil && !errors.Is(err, syscall.EIO) {
		return domain.Capture{}, fmt.Errorf("reading from %s: %w", s.Argv[0], err)
	}

	code, err := exitCode(cmd.Wait())
	if err != nil {
		return domain.Capture{}, err
	}
	cols, rows := size.get()
	return domain.Capture{
		Command:  command,
		Cwd:      cwd,
		ExitCode: code,
		Cols:     cols,
		Rows:     rows,
		Bytes:    captured.Bytes(),
	}, nil
}

// windowSize is the pty's size, read on start and again on every SIGWINCH.
// An explicit width survives a resize: --cols asked for that width, and a
// user dragging their window wider has not changed their mind about it.
type windowSize struct {
	mu         sync.Mutex
	fixedCols  int
	cols, rows int
}

func (w *windowSize) follow(f *os.File) {
	cols, rows := 0, 0
	if f != nil {
		cols, rows = tty.Size(f)
	} else {
		cols, rows = tty.Size(os.Stderr)
	}
	if w.fixedCols > 0 {
		cols = w.fixedCols
	}
	w.mu.Lock()
	w.cols, w.rows = cols, rows
	w.mu.Unlock()
}

func (w *windowSize) get() (cols, rows int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cols, w.rows
}

func (w *windowSize) winsize() *creack.Winsize {
	cols, rows := w.get()
	return &creack.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
}

// tee writes everything to the buffer and passes it through to out, until out
// fails. A passthrough that has stopped accepting output must not stop the
// capture: the child would block on a full pty and never finish.
type tee struct {
	out    io.Writer
	buf    *bytes.Buffer
	broken bool
}

func (t *tee) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if !t.broken && t.out != nil {
		if _, err := t.out.Write(p); err != nil {
			t.broken = true
		}
	}
	return len(p), nil
}

// forwardStdin copies stdin into the pty and, when stdin runs out, says so
// the way a terminal does: a pty has no write end to close, so the child
// would otherwise wait forever for input nobody will type. ^D ends input only
// at the start of a line - part-way through one it just hands over what is
// buffered - so a last line with no newline gets a second.
func forwardStdin(ptmx *os.File, stdin *os.File) {
	w := &lastByte{w: ptmx}
	if _, err := io.Copy(w, stdin); err != nil {
		return
	}
	eof := []byte{4}
	if w.seen && w.last != '\n' {
		eof = []byte{4, 4}
	}
	ptmx.Write(eof)
}

type lastByte struct {
	w    io.Writer
	last byte
	seen bool
}

func (l *lastByte) Write(p []byte) (int, error) {
	if len(p) > 0 {
		l.last, l.seen = p[len(p)-1], true
	}
	return l.w.Write(p)
}

// exitCode turns what Wait said into what a shell would have put in $?. A
// child that exited reports its status; one killed by a signal reports 128
// plus the signal, which exec.ExitError.ExitCode on its own gives as -1.
func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return 0, fmt.Errorf("waiting for the command: %w", err)
	}
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal()), nil
	}
	return exit.ExitCode(), nil
}

// startFailureCode is the status a shell gives a command it could not start:
// 127 for one it cannot find, 126 for one it found and may not run.
func startFailureCode(err error) int {
	switch {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return 127
	case errors.Is(err, fs.ErrPermission):
		return 126
	}
	return 1
}

// unwrapStart makes the message read like a shell's. exec phrases a missing
// binary as `exec: "foo": executable file not found in $PATH`, which names
// the command a second time when the caller has already prefixed it.
func unwrapStart(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("command not found")
	}
	return err
}

func childEnv(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if strings.HasPrefix(kv, "TERM=") || strings.HasPrefix(kv, "COLORTERM=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM="+term, "COLORTERM="+colorterm)
}

// shellJoin turns argv back into a command line a shell would accept, which is
// what the prompt line in the picture shows. The quoting codeshot's caller
// typed is gone by the time argv arrives - `-m "fix it"` and `-m 'fix it'` are
// the same argv - so each argument that needs quoting gets single quotes, the
// one form with no characters special inside it.
func shellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if !safeUnquoted(r) {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

func safeUnquoted(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("_-+=:,./@%^", r)
}
