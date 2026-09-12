//go:build !darwin && !windows

package clipboard

import (
	"bytes"
	"errors"
	"os/exec"
)

// tools are tried in order: Wayland first, then X11. Each takes the PNG on
// standard input and is told the type, so what lands on the clipboard is an
// image rather than text.
var tools = []struct {
	name string
	args []string
}{
	{"wl-copy", []string{"--type", "image/png"}},
	{"xclip", []string{"-selection", "clipboard", "-t", "image/png"}},
}

func copyPNG(png []byte, run func(*exec.Cmd) error) error {
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.name); err != nil {
			continue
		}
		cmd := exec.Command(tool.name, tool.args...)
		cmd.Stdin = bytes.NewReader(png)
		return run(cmd)
	}
	return errNoTool
}

func available() error {
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.name); err == nil {
			return nil
		}
	}
	return errNoTool
}

var errNoTool = errors.New("no clipboard tool found; install wl-clipboard (Wayland) or xclip (X11)")
