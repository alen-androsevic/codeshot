// Package clipboard puts a PNG on the system clipboard. It is one of the four
// per-OS seams design §5 names, and the only one whose tool is a separate
// program rather than a system call.
package clipboard

import "os/exec"

// Copy puts the PNG bytes on the clipboard as an image, not as text: what
// lands is something a chat window or an issue tracker will paste as a
// picture.
func Copy(png []byte) error {
	return copyPNG(png, run)
}

// Available reports whether there is a clipboard tool to copy with, so the
// CLI can refuse --clip before running a command rather than after.
func Available() error {
	return available()
}

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		return &toolError{tool: cmd.Path, output: string(out), err: err}
	}
	return err
}

type toolError struct {
	tool   string
	output string
	err    error
}

func (e *toolError) Error() string { return e.tool + ": " + trim(e.output) }
func (e *toolError) Unwrap() error { return e.err }

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
