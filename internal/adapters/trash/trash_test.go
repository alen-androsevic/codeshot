//go:build !windows

package trash

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMoveKeepsBothOfTwoFilesWithOneName: trashing shot.png twice must not
// lose the first one.
func TestMoveKeepsBothOfTwoFilesWithOneName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	work := t.TempDir()
	shot := filepath.Join(work, "shot.png")

	var moved []string
	for _, content := range []string{"first", "second"} {
		if err := os.WriteFile(shot, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		to, err := Move(shot)
		if err != nil {
			t.Fatalf("Move: %v", err)
		}
		if _, err := os.Stat(shot); !os.IsNotExist(err) {
			t.Errorf("%s is still there after Move", shot)
		}
		got, err := os.ReadFile(to)
		if err != nil || string(got) != content {
			t.Errorf("%s holds %q, %v; want %q", to, got, err, content)
		}
		moved = append(moved, to)
	}
	if moved[0] == moved[1] {
		t.Errorf("both went to %s", moved[0])
	}
	if filepath.Base(moved[1]) != "shot 2.png" {
		t.Errorf("second is %s, want shot 2.png", filepath.Base(moved[1]))
	}
}
