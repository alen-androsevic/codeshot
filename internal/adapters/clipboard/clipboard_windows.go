package clipboard

import (
	"errors"
	"os/exec"
)

func copyPNG([]byte, func(*exec.Cmd) error) error { return errUnsupported }

func available() error { return errUnsupported }

var errUnsupported = errors.New("the clipboard is not supported on this platform")
