package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeANSI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.ansi")
	body := "\x1b[1;32mhello\x1b[0m\r\nsecond line\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderWritesAShot(t *testing.T) {
	src := writeANSI(t)
	out := filepath.Join(t.TempDir(), "shot.png")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", src, out, "--command", "echo hello"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("no image written: %v", err)
	}
	if !strings.Contains(stderr.String(), "Stored codeshot in") {
		t.Errorf("stderr = %q, want the stored line", stderr.String())
	}
}

func TestRenderRejectsAnUnknownTheme(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir(), "--theme", "no-such"}, &stdout, &stderr)
	if code == 0 {
		t.Error("exit 0 for an unknown theme")
	}
	if !strings.Contains(stderr.String(), "theme") {
		t.Errorf("stderr = %q, want the theme named in the error", stderr.String())
	}
}

func TestRenderRejectsAMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "/no/such/file.ansi"}, &stdout, &stderr); code == 0 {
		t.Error("exit 0 for a missing input file")
	}
}

// TestRenderRejectsScaleBelowOne pins the controller ruling from the plan:
// app.Service.Run treats a zero Chrome.Scale as "no chrome supplied" and
// replaces the whole struct with defaults, so `--scale 0` would otherwise
// silently discard every other window flag the caller passed. Nothing in
// the render pipeline itself would fail on a bad scale - only this flag
// check stands between the user and that silent data loss - so the exit
// code must be exactly 2 (a usage error), not merely nonzero.
func TestRenderRejectsScaleBelowOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir(), "--scale", "0"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit %d, want 2 for a usage error", code)
	}
	if !strings.Contains(stderr.String(), "scale") {
		t.Errorf("stderr = %q, want --scale named in the error", stderr.String())
	}
}

func writeANSIWithoutCarriageReturns(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.ansi")
	// No \r anywhere: this is what a plain `cmd > file` redirect produces,
	// since it never passes through a pty to pick up the translation a real
	// terminal session would have applied.
	body := "line one\nline two\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRenderWarnsAboutMissingCarriageReturns covers the landmine a plain
// shell redirect walks straight into: without a single \r across more than
// one line, the emulator can never return to column one between lines and
// the picture stair-steps, with nothing in the render pipeline itself ever
// failing to explain why. The warning is the only thing standing between
// that and a silently wrong picture.
func TestRenderWarnsAboutMissingCarriageReturns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSIWithoutCarriageReturns(t), "x.png", "--gallery", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "carriage return") {
		t.Errorf("stderr = %q, want a warning about missing carriage returns", stderr.String())
	}
	if !strings.Contains(stderr.String(), "pty") {
		t.Errorf("stderr = %q, want the warning to say how to capture through a pty", stderr.String())
	}
}

// TestRenderDoesNotWarnForANormalCRLFDump guards against a false alarm on
// every ordinary capture: writeANSI's dump already has \r\n line endings,
// same as a real pty would produce, so nothing should be said about it.
func TestRenderDoesNotWarnForANormalCRLFDump(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "carriage return") {
		t.Errorf("stderr = %q, warned about a dump that already has \\r\\n", stderr.String())
	}
}

func TestHelpAndVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Errorf("--help exited %d", code)
	}
	if !strings.Contains(stdout.String(), "codeshot") {
		t.Errorf("help = %q", stdout.String())
	}
	stdout.Reset()
	if code := Run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Errorf("version exited %d", code)
	}
}

func TestThemesListsWhatIsEmbedded(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"themes"}, &stdout, &stderr)
	for _, want := range []string{"codeshot-dark", "codeshot-light"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("themes output %q is missing %s", stdout.String(), want)
		}
	}
}

func TestNoArgumentsExplainsItself(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code == 0 {
		t.Error("exit 0 with no arguments")
	}
	if !strings.Contains(stderr.String(), "render") {
		t.Errorf("stderr = %q, want a hint about the render subcommand", stderr.String())
	}
}

// TestBareDoubleDashTerminatesTheFlags pins the argument separator. split
// classified "--" as a flag, and takesValue defaults to true for anything it
// does not recognise, so "--" swallowed the argument after it: `codeshot
// render hi.ansi -- name.png` exited 0 and quietly wrote codeshot.png into
// the gallery instead, the name the user asked for having been eaten. This
// is also the syntax phase 2 is built around (`codeshot shot.png -- npm
// test`), so it has to mean "the flags end here" and nothing else.
func TestBareDoubleDashTerminatesTheFlags(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSI(t), "--gallery", dir, "--", "name.png"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "name.png")); err != nil {
		t.Errorf("the name after -- was not used: %v", err)
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) != 1 {
		t.Errorf("gallery holds %v, want name.png alone", entries)
	}
}

// TestTheSuiteNeverWritesOutsideItsOwnGallery is a standing guard on the
// tests above rather than on the CLI: an invocation that forgets --gallery
// falls back to defaultGallery(), which is $HOME/Codeshots - the user's real
// one. That is not a test failure anywhere, it just silently litters a
// directory full of the user's own files, so nothing catches it. Pointing
// HOME at a temporary directory and checking it stays empty catches it.
func TestTheSuiteNeverWritesOutsideItsOwnGallery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", writeANSI(t), "shot.png", "--gallery", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the run created %v under HOME; a test must never write to the user's own gallery", entries)
	}
}
