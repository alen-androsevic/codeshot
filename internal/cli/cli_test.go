package cli

import (
	"bytes"
	"image/png"
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
	code := Run([]string{"render", src, out, "--command", "echo hello"}, nil, &stdout, &stderr)
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
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir(), "--theme", "no-such"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Error("exit 0 for an unknown theme")
	}
	if !strings.Contains(stderr.String(), "theme") {
		t.Errorf("stderr = %q, want the theme named in the error", stderr.String())
	}
}

func TestRenderRejectsAMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "/no/such/file.ansi"}, nil, &stdout, &stderr); code == 0 {
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
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir(), "--scale", "0"}, nil, &stdout, &stderr)
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
	code := Run([]string{"render", writeANSIWithoutCarriageReturns(t), "x.png", "--gallery", t.TempDir()}, nil, &stdout, &stderr)
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
	code := Run([]string{"render", writeANSI(t), "x.png", "--gallery", t.TempDir()}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "carriage return") {
		t.Errorf("stderr = %q, warned about a dump that already has \\r\\n", stderr.String())
	}
}

func TestHelpAndVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("--help exited %d", code)
	}
	if !strings.Contains(stdout.String(), "codeshot") {
		t.Errorf("help = %q", stdout.String())
	}
	stdout.Reset()
	if code := Run([]string{"version"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("version exited %d", code)
	}
}

func TestThemesListsWhatIsEmbedded(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"themes"}, nil, &stdout, &stderr)
	for _, want := range []string{"codeshot-dark", "codeshot-light"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("themes output %q is missing %s", stdout.String(), want)
		}
	}
}

func TestNoArgumentsExplainsItself(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, nil, &stdout, &stderr); code == 0 {
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
	code := Run([]string{"render", writeANSI(t), "--gallery", dir, "--", "name.png"}, nil, &stdout, &stderr)
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
	if code := Run([]string{"render", writeANSI(t), "shot.png", "--gallery", dir}, nil, &stdout, &stderr); code != 0 {
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

// renderedWidth runs render into a fresh gallery and reports how wide the
// PNG came out. The margin is the only thing these tests vary, so a width
// difference is a margin difference.
func renderedWidth(t *testing.T, extra ...string) int {
	t.Helper()
	dir := t.TempDir()
	args := append([]string{"render", writeANSI(t), "shot.png", "--gallery", dir}, extra...)
	var stdout, stderr bytes.Buffer
	if code := Run(args, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	f, err := os.Open(filepath.Join(dir, "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width
}

// TestMarginDefaultsToZeroWithTheShadowOff pins the design doc's rule that
// --margin "defaults to 64 with the shadow on, 0 with it off". The margin
// exists to hold the blur, which spreads about forty pixels and sits
// eighteen lower; with no shadow to hold there is nothing in it, and 64px of
// dead transparent border on every side is not what --no-shadow asks for.
// The flag's default was hardcoded, so it could not tell "not set" from
// "set to 64" - which is the whole difficulty here, and why the last case
// below matters as much as the first two.
func TestMarginDefaultsToZeroWithTheShadowOff(t *testing.T) {
	const scale = 2 // the default
	withShadow := renderedWidth(t)
	noShadow := renderedWidth(t, "--no-shadow")
	if got, want := withShadow-noShadow, 2*64*scale; got != want {
		t.Errorf("--no-shadow narrowed the image by %d, want %d: the margin should fall to 0", got, want)
	}
	explicit := renderedWidth(t, "--no-shadow", "--margin", "64")
	if explicit != withShadow {
		t.Errorf("--no-shadow --margin 64 gave width %d, want %d: an explicit 64 must be honoured, not mistaken for the default", explicit, withShadow)
	}
	zero := renderedWidth(t, "--margin", "0")
	if zero != noShadow {
		t.Errorf("--margin 0 with the shadow on gave width %d, want %d", zero, noShadow)
	}
}

// TestRenderHelpPrintsTheDocumentedUsage covers a subcommand that answered
// --help with exit 2 and Go's raw flag dump: --help was handled only at the
// top level, so inside render() flag.ContinueOnError printed twenty
// single-dash flags with empty descriptions and returned ErrHelp, and the
// hand-written usage block - the one that spells the flags the way the
// README does - was unreachable.
func TestRenderHelpPrintsTheDocumentedUsage(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"render", arg}, nil, &stdout, &stderr); code != 0 {
				t.Errorf("exit %d, want 0; asking for help is not an error", code)
			}
			if !strings.Contains(stdout.String(), "--gallery") {
				t.Errorf("stdout = %q, want the documented double-dash flag list", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want help on stdout and nothing on stderr", stderr.String())
			}
		})
	}
}

// TestColsDefaultsToTheEmulatorsOwnFallback pins the width the emulator uses
// when --cols is not given, and pins it through the rendered image rather
// than by reading the flag back. The flag used to default to 100, duplicating
// vt.Adapter's fallbackCols; the flag now defaults to 0 and the adapter's
// own fallback applies, so that phase 2 - which sizes the pty from stderr -
// can tell "not set" from "set to 100". The visible behaviour must not move
// an inch while that happens, which is what the first assertion is for; the
// second keeps a default of 0 from quietly meaning "zero columns".
func TestColsDefaultsToTheEmulatorsOwnFallback(t *testing.T) {
	unset := renderedWidth(t)
	hundred := renderedWidth(t, "--cols", "100")
	if unset != hundred {
		t.Errorf("width with no --cols = %d, with --cols 100 = %d; the fallback must still be 100 columns", unset, hundred)
	}
	if forty := renderedWidth(t, "--cols", "40"); forty >= unset {
		t.Errorf("--cols 40 gave width %d, want less than the %d the default gives", forty, unset)
	}
}

// runWrapped runs codeshot in wrapper mode against a fresh gallery, with no stdin
// and a stderr that is not a terminal, so the pty is 100x24 whatever
// terminal the suite itself is running in.
func runWrapped(t *testing.T, args ...string) (code int, dir, stdout, stderr string) {
	t.Helper()
	dir = t.TempDir()
	var out, errb bytes.Buffer
	code = Run(append([]string{"--gallery", dir}, args...), nil, &out, &errb)
	return code, dir, out.String(), errb.String()
}

func shots(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestWrapperTakesAPictureOfTheCommand is the goal of phase 2 in one line:
// `codeshot -- ls -la` runs ls, shows its output as it happens, and leaves
// ls-la.png in the gallery, with no dump to make first.
func TestWrapperTakesAPictureOfTheCommand(t *testing.T) {
	code, dir, stdout, stderr := runWrapped(t, "--", "echo", "hello")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if stdout != "hello\r\n" {
		t.Errorf("stdout = %q, want the command's output passed through", stdout)
	}
	if got := shots(t, dir); len(got) != 1 || got[0] != "echo-hello.png" {
		t.Errorf("gallery = %v, want the shot named after the command", got)
	}
	if !strings.Contains(stderr, "Stored codeshot in") {
		t.Errorf("stderr = %q, want the stored line there and not on stdout", stderr)
	}
}

func TestWrapperHonoursAName(t *testing.T) {
	code, dir, _, stderr := runWrapped(t, "shot.png", "--", "true")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if got := shots(t, dir); len(got) != 1 || got[0] != "shot.png" {
		t.Errorf("gallery = %v, want shot.png", got)
	}
}

// TestWrapperLeavesTheCommandsArgumentsAlone: after the --, nothing is
// codeshot's. -la is ls's flag and --help is echo's argument, and neither
// may be parsed, rejected or answered by codeshot on the child's behalf.
func TestWrapperLeavesTheCommandsArgumentsAlone(t *testing.T) {
	code, _, stdout, stderr := runWrapped(t, "--", "echo", "-la", "--help", "--gallery")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if stdout != "-la --help --gallery\r\n" {
		t.Errorf("stdout = %q, want echo to have received its own arguments", stdout)
	}
}

func TestWrapperFlagsReachThePty(t *testing.T) {
	code, _, stdout, stderr := runWrapped(t, "--cols", "40", "--", "stty", "size")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if stdout != "24 40\r\n" {
		t.Errorf("stty size = %q, want --cols to have sized the pty", stdout)
	}
}

// TestWrapperCommandOverridesWhatThePromptShows is for the command line that
// should not be in a picture: `codeshot --command deploy -- deploy
// --token=...` keeps the token out of the image and out of the filename.
func TestWrapperCommandOverridesWhatThePromptShows(t *testing.T) {
	code, dir, _, stderr := runWrapped(t, "--command", "deploy", "--", "echo", "--token=s3cret")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if got := shots(t, dir); len(got) != 1 || got[0] != "deploy.png" {
		t.Errorf("gallery = %v, want the name taken from --command, not the real argv", got)
	}
}

// TestWrapperExitsWithTheChildsCode makes codeshot drop-in: `codeshot --
// make test && deploy` must not deploy when the tests failed. A failing run is
// still worth a picture - often it is the picture people want - so the shot
// is taken either way.
func TestWrapperExitsWithTheChildsCode(t *testing.T) {
	code, dir, _, stderr := runWrapped(t, "--", "sh", "-c", "echo boom; exit 3")
	if code != 3 {
		t.Errorf("exit %d, want the child's 3; stderr: %s", code, stderr)
	}
	if got := shots(t, dir); len(got) != 1 {
		t.Errorf("gallery = %v, want the failing run pictured too", got)
	}
}

func TestWrapperOfAMissingCommandIs127(t *testing.T) {
	code, dir, _, stderr := runWrapped(t, "--", "codeshot-no-such-command")
	if code != 127 {
		t.Errorf("exit %d, want 127 as a shell would", code)
	}
	if !strings.Contains(stderr, "codeshot-no-such-command: command not found") {
		t.Errorf("stderr = %q, want a shell-style message", stderr)
	}
	if got := shots(t, dir); len(got) != 0 {
		t.Errorf("gallery = %v, want no picture of a command that never ran", got)
	}
}

// TestWrapperChildsFailureOutranksCodeshots settles an ambiguity in design
// §10, which says both "exit with the child's code" and "a codeshot failure
// exits 1". When both happen, the child's code wins: a script gating on
// `codeshot -- make test` must see the tests fail, not a theme typo. Only a
// clean child with a failed picture exits 1.
func TestWrapperChildsFailureOutranksCodeshots(t *testing.T) {
	code, _, _, stderr := runWrapped(t, "--theme", "no-such", "--", "sh", "-c", "exit 3")
	if code != 3 {
		t.Errorf("failed child, failed picture: exit %d, want the child's 3", code)
	}
	if !strings.Contains(stderr, "no-such") {
		t.Errorf("stderr = %q, want the picture's failure still reported", stderr)
	}
	if code, _, _, _ := runWrapped(t, "--theme", "no-such", "--", "true"); code != 1 {
		t.Errorf("clean child, failed picture: exit %d, want 1", code)
	}
}

func TestWrapperNeedsACommand(t *testing.T) {
	for _, args := range [][]string{{"shot.png"}, {"shot.png", "--"}} {
		code, _, _, stderr := runWrapped(t, args...)
		if code != 2 {
			t.Errorf("%v: exit %d, want a usage error", args, code)
		}
		if !strings.Contains(stderr, "--") {
			t.Errorf("%v: stderr = %q, want it to point at --", args, stderr)
		}
	}
}

func TestWrapperTakesOneNameAtMost(t *testing.T) {
	if code, _, _, stderr := runWrapped(t, "one.png", "two.png", "--", "true"); code != 2 {
		t.Errorf("exit %d, want a usage error; stderr: %s", code, stderr)
	}
}

func TestWrapperStillAnswersItsOwnHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help", "--", "true"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "-- <command>") {
		t.Errorf("stdout = %q, want the usage block to document the wrapper", stdout.String())
	}
}
