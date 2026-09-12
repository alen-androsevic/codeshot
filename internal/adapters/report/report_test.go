package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoredNamesThePath(t *testing.T) {
	var err bytes.Buffer
	Writer{Err: &err}.Stored("/tmp/shots/ls-la.png")
	if err.String() != "Stored codeshot in /tmp/shots/ls-la.png\n" {
		t.Errorf("stderr = %q", err.String())
	}
}

// TestStoredContractsTheHomeDirectory exercises tildify, which no test
// reached before: every other test in the tree stores under t.TempDir(),
// which is never inside $HOME.
func TestStoredContractsTheHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var err bytes.Buffer
	Writer{Err: &err}.Stored(filepath.Join(home, "Codeshots", "ls-la.png"))
	if got := err.String(); got != "Stored codeshot in ~/Codeshots/ls-la.png\n" {
		t.Errorf("stderr = %q, want the home directory contracted to ~", got)
	}
}

// TestStoredDoesNotContractAMerePrefix: a sibling directory whose name starts
// with the home directory's is not inside it.
func TestStoredDoesNotContractAMerePrefix(t *testing.T) {
	home := filepath.Join(t.TempDir(), "alen")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	var err bytes.Buffer
	Writer{Err: &err}.Stored(home + "-backup/shot.png")
	if strings.Contains(err.String(), "~") {
		t.Errorf("stderr = %q, want no contraction of a directory merely sharing the prefix", err.String())
	}
}

// TestStoredSaysNothingWithoutAPath is what --stdout needs: the PNG is going
// to whatever is reading stdout, there is no file, and a line about storing
// one would be a lie in the middle of a machine's input.
func TestStoredSaysNothingWithoutAPath(t *testing.T) {
	var err bytes.Buffer
	Writer{Err: &err}.Stored("")
	if err.Len() != 0 {
		t.Errorf("stderr = %q, want silence when nothing was stored", err.String())
	}
}

func TestWarnIsPrefixed(t *testing.T) {
	var err bytes.Buffer
	Writer{Err: &err}.Warn("something looked wrong")
	if err.String() != "codeshot: something looked wrong\n" {
		t.Errorf("stderr = %q", err.String())
	}
}
