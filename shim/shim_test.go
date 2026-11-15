// Package shim tests the shell shims.
//
// Both halves are covered, and the half that was not is the half that was
// wrong: recovering the history line needs an interactive shell, which a
// test can start.
package shim

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// interactiveFlags start a shell that keeps history but reads none of the
// user's rc files.
var interactiveFlags = map[string][]string{
	"zsh":  {"-f", "-i"},
	"bash": {"--norc", "-i"},
}

var shells = []string{"zsh", "bash"}

func strip(t *testing.T, shell, line string) string {
	t.Helper()
	path := shellPath(t, shell)
	out, err := exec.Command(path, "-c", `source "$1"; _codeshot_strip_pipe "$2"`, shell, "codeshot."+shell, line).Output()
	if err != nil {
		t.Fatalf("%s: %v", shell, err)
	}
	return string(out)
}

func shellPath(t *testing.T, shell string) string {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("%s is not installed", shell)
	}
	return path
}

func TestStripPipe(t *testing.T) {
	cases := map[string]string{
		"npm test | codeshot shot.png":              "npm test",
		"npm test | codeshot":                       "npm test",
		"git log --oneline -5 | codeshot --theme x": "git log --oneline -5",
		"cat a | sort | codeshot":                   "cat a | sort",
		"npm test|codeshot":                         "npm test",
		"npm test   |   codeshot   shot.png":        "npm test",
		// codeshot's output going on somewhere: its own segment is the one
		// to cut at, not simply the last one.
		"npm test | codeshot --stdout | pngquant - > small.png": "npm test",
		// A segment that merely mentions codeshot is not codeshot's.
		"grep codeshot notes.md | codeshot": "grep codeshot notes.md",
		// No pipe at all: nothing produced these bytes, so there is nothing
		// to recover and the caption stays empty rather than becoming
		// codeshot's own invocation.
		"codeshot --clip < dump.ansi": "",
		"ls -la":                      "",
	}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			for line, want := range cases {
				if got := strip(t, shell, line); got != want {
					t.Errorf("%s: strip(%q) = %q, want %q", shell, line, got, want)
				}
			}
		})
	}
}

// recovered runs script in an interactive shell with the shim sourced and a
// stand-in codeshot on PATH, and returns the argv that stand-in was given,
// one invocation per slice.
func recovered(t *testing.T, shell, script string) [][]string {
	t.Helper()
	path := shellPath(t, shell)
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The stand-in has to drain its standard input: the shim is being tested
	// in a pipeline, and a producer whose reader vanishes dies of SIGPIPE.
	const fake = "#!/bin/sh\n" +
		"for a in \"$@\"; do printf '%s\\n' \"$a\" >> \"$CODESHOT_LOG\"; done\n" +
		"printf '=== \\n' >> \"$CODESHOT_LOG\"\n" +
		"cat > /dev/null\n"
	if err := os.WriteFile(filepath.Join(bin, "codeshot"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	shimFile, err := filepath.Abs("codeshot." + shell)
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "argv")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, interactiveFlags[shell]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("source " + shimFile + "\n" + script + "\n")
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CODESHOT_LOG="+log,
		// Never touch the history of the person running the tests.
		"HISTFILE="+filepath.Join(dir, "history"),
	)
	// An interactive shell reading a script exits noisily; the log is the
	// evidence, not the status.
	_ = cmd.Run()

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the shim never reached codeshot: %v", err)
	}
	var calls [][]string
	var call []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "=== " {
			calls = append(calls, call)
			call = nil
			continue
		}
		call = append(call, line)
	}
	return calls
}

func commandOf(argv []string) (string, bool) {
	for i, a := range argv {
		if a == "--command" && i+1 < len(argv) {
			return argv[i+1], true
		}
	}
	return "", false
}

// TestRecoversTheLineBeingRun is the bug the first pipe shot showed: every
// picture was captioned with the command before the one that made it, and
// the file was named after it too. `fc -ln -1` is the previous line in both
// shells, in a pipeline and out of one.
func TestRecoversTheLineBeingRun(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell, "echo a-decoy-command\nprintf 'x\\n' | codeshot shot.png")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			got, ok := commandOf(calls[0])
			if !ok {
				t.Fatalf("argv = %v, want a --command in it", calls[0])
			}
			if got != `printf 'x\n'` {
				t.Errorf("--command = %q, want the line being run", got)
			}
		})
	}
}

// TestPassesTheRestOfArgvThrough: the shim adds a caption, it does not eat
// the arguments the user typed.
func TestPassesTheRestOfArgvThrough(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell, "echo hi | codeshot shot.png --theme dracula")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			joined := strings.Join(calls[0], " ")
			for _, want := range []string{"shot.png", "--theme", "dracula"} {
				if !strings.Contains(joined, want) {
					t.Errorf("argv = %v, want %q kept", calls[0], want)
				}
			}
		})
	}
}

// TestWrapperModeStandsAside: `codeshot -- cmd` has the command in argv,
// and history would only be a worse answer for it.
func TestWrapperModeStandsAside(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			// The stand-in drains its standard input, which in wrapper mode
			// is the shell's own: without the redirect it swallows the rest
			// of the script this test is written in.
			calls := recovered(t, shell, "codeshot -- echo hi < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			if got, ok := commandOf(calls[0]); ok {
				t.Errorf("wrapper mode argv = %v, want no --command added, got %q", calls[0], got)
			}
		})
	}
}

// TestAnExplicitCommandStandsAside: what the user passed by hand wins over
// anything history could offer.
func TestAnExplicitCommandStandsAside(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell, "echo hi | codeshot --command 'said hi'")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			if got, _ := commandOf(calls[0]); got != "said hi" {
				t.Errorf("--command = %q, want the one the user passed", got)
			}
		})
	}
}

// TestARedirectIsNotAPipeline: `codeshot < dump.ansi` has no producer on a
// pipe, so there is no command to recover and none is invented.
func TestARedirectIsNotAPipeline(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell, "echo saved > dump.ansi\ncodeshot shot.png < dump.ansi")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			if got, ok := commandOf(calls[0]); ok {
				t.Errorf("--command = %q, want none: a redirect has no producing command", got)
			}
		})
	}
}

// TestWrapperModeExpandsAnAlias is the `codeshot -- ll` report: codeshot
// execs the command itself, and an alias lives in the shell and nowhere a
// child process can see it, so `ll` came back "command not found". The shell
// is the only thing that can answer, and the shim is in the shell.
func TestWrapperModeExpandsAnAlias(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell,
				"alias ll='printf hello'\ncodeshot -- ll < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			argv := calls[0]
			joined := strings.Join(argv, " ")
			if !strings.Contains(joined, "-- printf hello") {
				t.Errorf("argv = %v, want the alias expanded behind the --", argv)
			}
			// The caption is what the user typed, not what it stood for:
			// their own scrollback says `ll`.
			if got, ok := commandOf(argv); !ok || got != "ll" {
				t.Errorf("--command = %q, want \"ll\"", got)
			}
		})
	}
}

// TestWrapperModeKeepsTheArgumentsAfterAnAlias: `codeshot -- ll -a` is the
// alias plus a flag of the user's, and the flag belongs to the command.
func TestWrapperModeKeepsTheArgumentsAfterAnAlias(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell,
				"alias ll='printf hello'\ncodeshot --theme dracula -- ll world < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			joined := strings.Join(calls[0], " ")
			if !strings.Contains(joined, "-- printf hello world") {
				t.Errorf("argv = %v, want the alias expanded and `world` kept after it", calls[0])
			}
			if !strings.Contains(joined, "--theme dracula") {
				t.Errorf("argv = %v, want codeshot's own flags kept", calls[0])
			}
			if got, _ := commandOf(calls[0]); got != "ll world" {
				t.Errorf("--command = %q, want the line the user typed", got)
			}
		})
	}
}

// TestWrapperModeLeavesARealCommandAlone: only an alias is substituted, and
// a command that is not one reaches codeshot exactly as it was typed.
func TestWrapperModeLeavesARealCommandAlone(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell, "codeshot -- echo hi < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			if got := strings.Join(calls[0], " "); got != "-- echo hi" {
				t.Errorf("argv = %q, want `-- echo hi` untouched", got)
			}
		})
	}
}

// TestWrapperModeDoesNotExpandAnAliasFurtherIn: only the command word is an
// alias. `codeshot -- git ll` is git's subcommand, not the shell's alias.
func TestWrapperModeDoesNotExpandAnAliasFurtherIn(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell,
				"alias ll='printf hello'\ncodeshot -- echo ll < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			if got := strings.Join(calls[0], " "); got != "-- echo ll" {
				t.Errorf("argv = %q, want `ll` left alone: it is an argument, not the command", got)
			}
		})
	}
}

// TestWrapperModeRunsAnAliasThatIsAScript: an alias with a pipe or a
// redirect in it cannot be split into a command and its arguments, because
// it is not one command. The shell that owns the alias runs it instead.
func TestWrapperModeRunsAnAliasThatIsAScript(t *testing.T) {
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			calls := recovered(t, shell,
				"alias busy='printf one | tr a-z A-Z'\ncodeshot -- busy < /dev/null")
			if len(calls) != 1 {
				t.Fatalf("codeshot ran %d times, want once: %v", len(calls), calls)
			}
			argv := calls[0]
			joined := strings.Join(argv, " ")
			if !strings.Contains(joined, "-- "+shell+" -c") {
				t.Errorf("argv = %v, want the alias handed to %s -c", argv, shell)
			}
			if !strings.Contains(joined, "printf one | tr a-z A-Z") {
				t.Errorf("argv = %v, want the alias body passed through whole", argv)
			}
			if got, _ := commandOf(argv); got != "busy" {
				t.Errorf("--command = %q, want the word the user typed", got)
			}
		})
	}
}
