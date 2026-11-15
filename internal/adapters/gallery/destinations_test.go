package gallery

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func red() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	return img
}

func TestWriterEncodesToTheStream(t *testing.T) {
	var out bytes.Buffer
	path, err := Writer{W: &out}.Store("ignored.png", red())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want none; there is no file to name", path)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatalf("what was written is not a PNG: %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Errorf("bounds = %v", img.Bounds())
	}
}

func TestWriterStepsAroundNothing(t *testing.T) {
	if (Writer{}).Exists("anything.png") {
		t.Error("a stream reported an existing file")
	}
	if (Clip{}).Exists("anything.png") {
		t.Error("the clipboard reported an existing file")
	}
}

func TestClipCopiesAPNGAndNamesItself(t *testing.T) {
	var got []byte
	path, err := Clip{Copy: func(b []byte) error { got = b; return nil }}.Store("ignored.png", red())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != "the clipboard" {
		t.Errorf("path = %q; it is what the stored line says the picture went to", path)
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Errorf("what went to the clipboard is not a PNG: %v", err)
	}
}

func TestClipReportsAFailedCopy(t *testing.T) {
	_, err := Clip{Copy: func([]byte) error { return errors.New("no clipboard tool") }}.Store("x.png", red())
	if err == nil {
		t.Error("Store swallowed a clipboard failure")
	}
}
