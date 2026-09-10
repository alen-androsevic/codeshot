// Package app holds codeshot's one use case and the ports it drives. Nothing
// here knows what a pty, a font file or a PNG encoder is; that is the point.
package app

import (
	"image"

	"codeshot/internal/domain"
)

// CaptureSource yields everything one command produced, however it got hold
// of it: a pty it owns, a pipe, or a file on disk.
type CaptureSource interface {
	Capture() (domain.Capture, error)
}

// Emulator turns raw bytes into grids of styled cells.
type Emulator interface {
	Emulate(domain.Capture) (domain.Result, error)
}

// PromptSource supplies the bytes of the prompt and command lines that sit
// above the output. They are emulated like any other bytes.
type PromptSource interface {
	Header(domain.Capture) []byte
}

type ThemeSource interface {
	Theme(name string) (domain.Theme, error)
}

type Renderer interface {
	Render(domain.Window) (image.Image, error)
}

// Gallery is where shots are kept. Exists lets the naming rules step around a
// file rather than over it.
type Gallery interface {
	Exists(name string) bool
	Store(name string, img image.Image) (path string, err error)
}

type Reporter interface {
	Stored(path string)
	Warn(msg string)
}
