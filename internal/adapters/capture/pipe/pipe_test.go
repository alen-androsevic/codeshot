package pipe

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func notATerminal(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-tty"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestCaptureReadsStdinToTheEnd(t *testing.T) {
	var out bytes.Buffer
	c, err := Source{In: strings.NewReader("a\r\nb\r\n"), Stdout: &out, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if string(c.Bytes) != "a\r\nb\r\n" {
		t.Errorf("Bytes = %q", c.Bytes)
	}
	if out.String() != "a\r\nb\r\n" {
		t.Errorf("passthrough = %q; a pipeline still has to show its output", out.String())
	}
}

// TestCaptureOfNothingIsStillACapture: design §10 says an empty capture is a
// valid image - prompt and command line, nothing under them.
func TestCaptureOfNothingIsStillACapture(t *testing.T) {
	c, err := Source{In: strings.NewReader(""), Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(c.Bytes) != 0 {
		t.Errorf("Bytes = %q, want empty", c.Bytes)
	}
}

func TestCaptureTakesItsWidthFromTheTerminal(t *testing.T) {
	c, err := Source{In: strings.NewReader(""), Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Cols != 100 || c.Rows != 24 {
		t.Errorf("Capture is %dx%d, want the 100x24 fallback with no terminal", c.Cols, c.Rows)
	}

	c, err = Source{In: strings.NewReader(""), Cols: 72, Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Cols != 72 {
		t.Errorf("Cols = %d, want --cols to win", c.Cols)
	}
}

func TestCaptureCarriesTheCommandAndCwd(t *testing.T) {
	c, err := Source{In: strings.NewReader(""), Command: "npm test", Cwd: "/tmp", Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Command != "npm test" || c.Cwd != "/tmp" {
		t.Errorf("Capture = %+v", c)
	}
	if c.ExitCode != 0 {
		t.Errorf("ExitCode = %d; a pipe never learns the producer's status", c.ExitCode)
	}
}

func TestCaptureFallsBackToTheWorkingDirectory(t *testing.T) {
	wd, _ := os.Getwd()
	c, err := Source{In: strings.NewReader(""), Size: notATerminal(t)}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Cwd != wd {
		t.Errorf("Cwd = %q, want %q", c.Cwd, wd)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input error") }

func TestCaptureReportsAFailedRead(t *testing.T) {
	if _, err := (Source{In: failingReader{}, Size: notATerminal(t)}).Capture(); err == nil {
		t.Error("Capture ignored a failed read")
	}
	if _, err := (Source{Size: notATerminal(t)}).Capture(); err == nil {
		t.Error("Capture invented input from a nil reader")
	}
}
