package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCaptureCarriesTheFileAndTheFlagsThrough(t *testing.T) {
	path := write(t, "dump.ansi", "a.go\r\nb.go\r\n")
	c, err := Source{Path: path, Command: "ls -la", Cwd: "/tmp", Cols: 120, Rows: 24}.Capture()
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if string(c.Bytes) != "a.go\r\nb.go\r\n" {
		t.Errorf("Bytes = %q", c.Bytes)
	}
	if c.Command != "ls -la" || c.Cwd != "/tmp" || c.Cols != 120 || c.Rows != 24 {
		t.Errorf("Capture = %+v, want the fields passed straight through", c)
	}
	if c.ExitCode != 0 {
		t.Errorf("ExitCode = %d; a saved dump never ran a child", c.ExitCode)
	}
}

// TestCaptureFallsBackToTheWorkingDirectory covers the branch that decides
// what the prompt line says when --cwd was not passed. Until this test it was
// the only unexercised line in phase 1's entry point to the whole pipeline.
func TestCaptureFallsBackToTheWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Source{Path: write(t, "dump.ansi", "hi\r\n")}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if c.Cwd != wd {
		t.Errorf("Cwd = %q, want the working directory %q", c.Cwd, wd)
	}
}

func TestCaptureNamesTheFileItCouldNotRead(t *testing.T) {
	_, err := Source{Path: "/nonexistent/dump.ansi"}.Capture()
	if err == nil {
		t.Fatal("Capture read a file that is not there")
	}
	if !strings.Contains(err.Error(), "/nonexistent/dump.ansi") {
		t.Errorf("error = %q, want the path in it", err)
	}
}

// TestCaptureWarnsAboutAPlainRedirect moves with the check itself. The
// warning used to live in the CLI, which read the file a second time to make
// it; it belongs to the source that knows what a saved dump is, because
// neither a pty nor a pipe can ever produce this shape of mistake.
func TestCaptureWarnsAboutAPlainRedirect(t *testing.T) {
	var warned []string
	path := write(t, "dump.ansi", "a.go\nb.go\n")
	if _, err := (Source{Path: path, Warn: func(m string) { warned = append(warned, m) }}).Capture(); err != nil {
		t.Fatal(err)
	}
	if len(warned) != 1 {
		t.Fatalf("warnings = %v, want one", warned)
	}
	if !strings.Contains(warned[0], path) {
		t.Errorf("warning = %q, want the file named", warned[0])
	}
}

// TestCaptureWarnsButDoesNotRewrite is the point of the warning being a
// warning. codeshot renders what a pty master emitted; turning a lone \n into
// \r\n would corrupt a genuine capture that really does mean "move down, stay
// in this column", so the bytes handed on are the bytes on disk.
func TestCaptureWarnsButDoesNotRewrite(t *testing.T) {
	path := write(t, "dump.ansi", "a.go\nb.go\n")
	c, err := Source{Path: path, Warn: func(string) {}}.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if string(c.Bytes) != "a.go\nb.go\n" {
		t.Errorf("Bytes = %q, want the file unchanged", c.Bytes)
	}
}

func TestCaptureStaysQuietAboutARealCapture(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"carriage returns present", "a.go\r\nb.go\r\n"},
		{"a single line has no alignment to lose", "a.go\n"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var warned []string
			path := write(t, "dump.ansi", tc.content)
			if _, err := (Source{Path: path, Warn: func(m string) { warned = append(warned, m) }}).Capture(); err != nil {
				t.Fatal(err)
			}
			if len(warned) != 0 {
				t.Errorf("warnings = %v, want none", warned)
			}
		})
	}
}

// TestCaptureWithNoWarnFunc keeps the zero Source usable. Every other test in
// the tree that builds a file.Source by hand leaves Warn nil.
func TestCaptureWithNoWarnFunc(t *testing.T) {
	if _, err := (Source{Path: write(t, "dump.ansi", "a.go\nb.go\n")}).Capture(); err != nil {
		t.Fatalf("Capture: %v", err)
	}
}
