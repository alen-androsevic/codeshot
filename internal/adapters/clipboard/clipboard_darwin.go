package clipboard

import (
	"fmt"
	"os"
	"os/exec"
)

// copyPNG goes through osascript because pbcopy cannot do this. pbcopy puts
// everything on the pasteboard as plain text unless it is EPS or RTF, so a
// PNG piped into it arrives as a wall of binary text rather than an image.
// AppleScript's «class PNGf» is the PNG pasteboard type, and its `read` takes
// a file, which is why the bytes go to a temporary one first.
func copyPNG(png []byte, run func(*exec.Cmd) error) error {
	f, err := os.CreateTemp("", "codeshot-*.png")
	if err != nil {
		return fmt.Errorf("making a temporary file for the clipboard: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(png); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", f.Name(), err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	script := fmt.Sprintf("set the clipboard to (read (POSIX file %q) as «class PNGf»)", f.Name())
	return run(exec.Command("osascript", "-e", script))
}

// available is always nil on macOS: osascript ships with the system.
func available() error { return nil }
