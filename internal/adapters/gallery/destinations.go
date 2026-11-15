package gallery

import (
	"bytes"
	"image"
	"image/png"
	"io"
)

// Writer is --stdout: the PNG goes to a stream instead of the gallery, for a
// caller piping it somewhere. It reports no path, because there is no file to
// tell anyone about and the stored line would land in whatever is reading.
type Writer struct {
	W io.Writer
}

// Exists is always false: a stream has nothing already in it to step around.
func (Writer) Exists(string) bool { return false }

func (w Writer) Store(_ string, img image.Image) (string, error) {
	if err := png.Encode(w.W, img); err != nil {
		return "", err
	}
	return "", nil
}

// Clip is --clip: the picture goes to the clipboard and no file is written.
// Copy is the clipboard adapter's, taken as a function so this stays testable
// without touching the machine's real clipboard.
type Clip struct {
	Copy func(png []byte) error
}

func (Clip) Exists(string) bool { return false }

func (c Clip) Store(_ string, img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	if err := c.Copy(buf.Bytes()); err != nil {
		return "", err
	}
	return "the clipboard", nil
}
