package vt

import (
	"fmt"
	"strings"
)

// mode handles the private sequences - CSI ? n h and CSI ? n l. Only three
// matter to a still picture: the alternate screen, autowrap, and the cursor,
// which codeshot never draws anyway.
func (e *Emulator) mode(final byte) {
	set := final == 'h'
	if final != 'h' && final != 'l' {
		e.report(fmt.Sprintf("CSI ? %v %c", e.params, final))
		return
	}
	for _, p := range e.params {
		switch p {
		case 47, 1047, 1049:
			e.setAlt(set)
		case 7:
			e.autowrap = set
		case 25:
			// Cursor visibility: a shot has no cursor to hide.
		default:
			e.report(fmt.Sprintf("CSI ? %d %c", p, final))
		}
	}
}

// setAlt switches buffers. Entering clears the alternate screen, which is what
// every terminal does and what every TUI assumes.
func (e *Emulator) setAlt(on bool) {
	if on {
		if e.cur == e.alt {
			return
		}
		e.alt = newBuffer(e.cols, e.rows, false)
		e.cur = e.alt
		return
	}
	e.cur = e.main
}

// oscByte gathers an operating system command. Only the title strings - OSC 0,
// 1 and 2 - are kept; the rest, including the shell integration marks a prompt
// may emit, are dropped without ceremony.
func (e *Emulator) oscByte(b byte) {
	switch {
	case e.state == stOSCEsc:
		e.state = stGround
		if b == '\\' {
			e.finishOSC()
		}
	case b == 0x07:
		e.state = stGround
		e.finishOSC()
	case b == 0x1B:
		e.state = stOSCEsc
	default:
		e.osc = append(e.osc, b)
	}
}

func (e *Emulator) finishOSC() {
	s := string(e.osc)
	e.osc = e.osc[:0]
	kind, text, ok := strings.Cut(s, ";")
	if !ok {
		return
	}
	switch kind {
	case "0", "1", "2":
		e.title = text
	default:
		e.report("OSC " + kind)
	}
}
