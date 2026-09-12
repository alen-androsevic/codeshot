package clipboard

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCopyHandsOsascriptAFileThatExists covers the awkward part of the macOS
// clipboard: AppleScript's `read` needs a file, so the PNG is written to a
// temporary one that has to be there when osascript runs and gone afterwards.
// A fake runner is used rather than the real one, because a test must not
// walk off with whatever the user had on their clipboard.
func TestCopyHandsOsascriptAFileThatExists(t *testing.T) {
	var gotArgs []string
	var contentsAtRunTime []byte
	var path string

	err := copyPNG([]byte("\x89PNG-pretend"), func(cmd *exec.Cmd) error {
		gotArgs = cmd.Args
		script := cmd.Args[len(cmd.Args)-1]
		start := strings.Index(script, `"`)
		end := strings.LastIndex(script, `"`)
		path = script[start+1 : end]
		contentsAtRunTime, _ = os.ReadFile(path)
		return nil
	})
	if err != nil {
		t.Fatalf("copyPNG: %v", err)
	}
	if gotArgs[0] != "osascript" || gotArgs[1] != "-e" {
		t.Errorf("args = %v, want osascript -e", gotArgs)
	}
	if string(contentsAtRunTime) != "\x89PNG-pretend" {
		t.Errorf("the file osascript was pointed at held %q", contentsAtRunTime)
	}
	if !strings.Contains(gotArgs[2], "«class PNGf»") {
		t.Errorf("script = %q, want the PNG pasteboard type", gotArgs[2])
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("temporary file %q does not end in .png", path)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s outlived the copy; codeshot must not leave PNGs in the temp directory", path)
	}
}

func TestCopyCleansUpAfterAFailedTool(t *testing.T) {
	var path string
	err := copyPNG([]byte("x"), func(cmd *exec.Cmd) error {
		script := cmd.Args[len(cmd.Args)-1]
		path = script[strings.Index(script, `"`)+1 : strings.LastIndex(script, `"`)]
		return errors.New("osascript said no")
	})
	if err == nil {
		t.Fatal("copyPNG hid the tool's failure")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s was left behind after a failure", path)
	}
}

func TestAvailableOnMacOS(t *testing.T) {
	if err := Available(); err != nil {
		t.Errorf("Available = %v, want nil; osascript ships with macOS", err)
	}
}
