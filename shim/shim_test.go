// Package shim tests the shell shims. Only the part that can be tested is:
// recovering the history line needs an interactive shell, which a test does
// not have, but stripping the `| codeshot ...` off the end of one is a pure
// string operation and is where the mistakes would be.
package shim

import (
	"os/exec"
	"testing"
)

func strip(t *testing.T, shell, file, line string) string {
	t.Helper()
	path, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("%s is not installed", shell)
	}
	out, err := exec.Command(path, "-c", `source "$1"; _codeshot_strip_pipe "$2"`, shell, file, line).Output()
	if err != nil {
		t.Fatalf("%s: %v", shell, err)
	}
	return string(out)
}

func TestStripPipe(t *testing.T) {
	cases := map[string]string{
		"npm test | codeshot shot.png":              "npm test",
		"npm test | codeshot":                       "npm test",
		"git log --oneline -5 | codeshot --theme x": "git log --oneline -5",
		"cat a | sort | codeshot":                   "cat a | sort",
		"npm test|codeshot":                         "npm test",
		"npm test   |   codeshot   shot.png":        "npm test",
		// No pipe at all: nothing to strip, and the shim only consults
		// history when standard input is not a terminal anyway.
		"ls -la": "ls -la",
	}
	for _, shell := range []string{"zsh", "bash"} {
		for _, file := range []string{"codeshot." + shell} {
			t.Run(shell, func(t *testing.T) {
				for line, want := range cases {
					if got := strip(t, shell, file, line); got != want {
						t.Errorf("%s: strip(%q) = %q, want %q", shell, line, got, want)
					}
				}
			})
		}
	}
}
