package gallery

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func pixel() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	return img
}

func TestStoreWritesAPNGIntoTheGallery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Codeshots")
	g := FS{Dir: dir}
	path, err := g.Store("shot.png", pixel())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != filepath.Join(dir, "shot.png") {
		t.Errorf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[1:4]) != "PNG" {
		t.Error("the file is not a PNG")
	}
}

func TestStoreCreatesTheGalleryDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "Codeshots")
	if _, err := (FS{Dir: dir}).Store("x.png", pixel()); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

func TestStoreTreatsANameWithASeparatorAsAPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere", "shot.png")
	path, err := (FS{Dir: filepath.Join(dir, "Codeshots")}).Store(target, pixel())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != target {
		t.Errorf("path = %q, want %q", path, target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("nothing written to the requested path: %v", err)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	g := FS{Dir: dir}
	if g.Exists("nope.png") {
		t.Error("Exists lied about a missing file")
	}
	os.WriteFile(filepath.Join(dir, "yes.png"), []byte("x"), 0o644)
	if !g.Exists("yes.png") {
		t.Error("Exists missed a file that is there")
	}
}

// TestStoreTrashesTheFileItReplaces: an explicit name replaces what is there,
// and the old shot goes where it can be put back from, not nowhere.
func TestStoreTrashesTheFileItReplaces(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "shot.png")
	os.WriteFile(old, []byte("old"), 0o644)
	bin := filepath.Join(t.TempDir(), "shot.png")
	var told [2]string
	g := FS{
		Dir:      dir,
		Trash:    func(p string) (string, error) { return bin, os.Rename(p, bin) },
		Replaced: func(p, trashed string) { told = [2]string{p, trashed} },
	}
	if _, err := g.Store("shot.png", pixel()); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got, _ := os.ReadFile(bin); string(got) != "old" {
		t.Errorf("the trash holds %q, want the old shot", got)
	}
	if told != [2]string{old, bin} {
		t.Errorf("Replaced was told %q", told)
	}
}

// TestStoreLeavesAFileItCannotTrash: when the old shot cannot be moved, it
// stays, and the new one is not written over it.
func TestStoreLeavesAFileItCannotTrash(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "shot.png")
	os.WriteFile(old, []byte("old"), 0o644)
	g := FS{Dir: dir, Trash: func(string) (string, error) { return "", errors.New("no trash") }}
	if _, err := g.Store("shot.png", pixel()); err == nil {
		t.Fatal("Store wrote over a file it could not trash")
	}
	if got, _ := os.ReadFile(old); string(got) != "old" {
		t.Errorf("the old shot now holds %q", got)
	}
}
