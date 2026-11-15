package vt

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"codeshot/internal/domain"
)

type parseState uint8

const (
	stGround parseState = iota
	stEsc
	stEscInter // ESC plus intermediate bytes, awaiting the final one
	stCSI
	stCSIIgnore // a CSI already known to be unusable, awaiting its final byte
	stOSC
	stOSCEsc
	stString // a DCS, PM, APC or SOS payload, awaiting its string terminator
	stStringEsc
)

// Emulator turns a byte stream into grids. It is a value with no I/O of its
// own: feed it everything a command printed, then ask for the Result.
type Emulator struct {
	cols, rows int
	main, alt  *buffer
	cur        *buffer
	style      domain.Style
	title      string
	autowrap   bool

	state   parseState
	params  []int
	private byte
	inter   []byte // the intermediate bytes of the escape being parsed
	subparm bool   // the CSI parameter being read is a colon sub-parameter
	subseen bool   // this CSI carried sub-parameters somewhere
	strKind byte   // which of P, ^, _ or X opened the string being swallowed
	osc     []byte
	utf8buf []byte

	// Unknown, if set, is told about every sequence the emulator ignored. The
	// CLI wires it to --debug; nothing else looks at it.
	Unknown func(seq string)
}

func New(cols, rows int) *Emulator {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	e := &Emulator{cols: cols, rows: rows, autowrap: true}
	e.main = newBuffer(cols, rows, true)
	e.alt = newBuffer(cols, rows, false)
	e.cur = e.main
	return e
}

func (e *Emulator) Write(p []byte) (int, error) {
	for _, b := range p {
		e.step(b)
	}
	return len(p), nil
}

// Result is the emulator's whole output. UsedAlt means the stream *ended* on
// the alternate screen: a TUI that exited and restored the normal screen left
// the terminal showing the normal one, and so does codeshot.
func (e *Emulator) Result() domain.Result {
	return domain.Result{
		Main:    e.main.grid(),
		Alt:     e.alt.grid(),
		UsedAlt: e.cur == e.alt,
		Title:   e.title,
	}
}

func (e *Emulator) step(b byte) {
	switch e.state {
	case stGround:
		e.ground(b)
	default:
		e.escape(b)
	}
}

func (e *Emulator) ground(b byte) {
	if len(e.utf8buf) > 0 || b >= 0x80 {
		e.decode(b)
		return
	}
	switch b {
	case 0x1B:
		e.state = stEsc
	case '\r':
		e.cur.carriageReturn()
	case '\n', 0x0B, 0x0C:
		e.cur.lineFeed()
	case '\b':
		e.cur.backspace()
	case '\t':
		e.cur.tab()
	case 0x07, 0x00:
		// A bell rings nothing in a picture, and NUL is padding.
	default:
		if b >= 0x20 {
			e.print(rune(b))
		}
	}
}

// decode gathers a UTF-8 rune, which may arrive split across two Writes.
func (e *Emulator) decode(b byte) {
	e.utf8buf = append(e.utf8buf, b)
	if !utf8.FullRune(e.utf8buf) {
		if len(e.utf8buf) < utf8.UTFMax {
			return
		}
		e.utf8buf = e.utf8buf[:0]
		e.print(utf8.RuneError)
		return
	}
	r, _ := utf8.DecodeRune(e.utf8buf)
	e.utf8buf = e.utf8buf[:0]
	e.print(r)
}

func (e *Emulator) print(r rune) {
	w := runewidth.RuneWidth(r)
	if w > 0 && !e.autowrap && e.cur.wrapNext {
		// With autowrap off the cursor stays put and overwrites in place.
		e.cur.wrapNext = false
	}
	e.cur.put(r, w, e.style)
}
