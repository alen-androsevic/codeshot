// Package prompt synthesises the two lines above the output. In wrapper mode
// the real prompt was drawn by the parent shell before codeshot existed, so
// the one in the picture is codeshot's own - close to what was on screen, and
// honest about being a stand-in.
package prompt

import (
	"strings"

	"codeshot/internal/domain"
	"codeshot/internal/home"
)

// Default is a cwd line and a chevron, in the shape most prompts take. The
// glyph choice is recorded in docs/adr/0001-prompt-glyph.md; use whatever that
// ADR settled on.
const Default = "\x1b[36m{cwd}\x1b[0m\r\n\x1b[92m❯\x1b[0m "

type Template struct {
	Text string
}

// Header returns the bytes that go through the emulator ahead of the output.
// They end in CRLF because the emulator reads what a pty master would emit,
// where a bare line feed does not return to column one.
func (t Template) Header(c domain.Capture) []byte {
	text := t.Text
	if text == "" {
		text = Default
	}
	text = strings.ReplaceAll(text, "{cwd}", home.Tildify(c.Cwd))
	return []byte(text + c.Command + "\r\n")
}
