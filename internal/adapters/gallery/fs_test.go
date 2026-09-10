package gallery

import (
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
