package vt

import "fmt"

// escape drives everything after ESC. The states are deliberately few: a CSI
// with its numeric parameters, an OSC string, and a catch-all that swallows
// two-byte escapes whole.
func (e *Emulator) escape(b byte) {
	switch e.state {
	case stEsc:
		switch b {
		case '[':
			e.state = stCSI
			e.params = e.params[:0]
			e.private = 0
		case ']':
			e.state = stOSC
			e.osc = e.osc[:0]
		default:
			e.report(fmt.Sprintf("ESC %c", b))
			e.state = stGround
		}
	case stCSI:
		e.csiByte(b)
	case stOSC, stOSCEsc:
		e.oscByte(b)
	}
}

func (e *Emulator) csiByte(b byte) {
	switch {
	case b >= '0' && b <= '9':
		if len(e.params) == 0 {
			e.params = append(e.params, 0)
		}
		e.params[len(e.params)-1] = e.params[len(e.params)-1]*10 + int(b-'0')
	case b == ';':
		e.params = append(e.params, 0)
	case b >= '<' && b <= '?':
		e.private = b
	case b >= 0x20 && b <= 0x2F:
		// Intermediate bytes; codeshot needs none of the sequences that use
		// them, but they must not end the sequence either.
	case b >= 0x40 && b <= 0x7E:
		e.csi(b)
		e.state = stGround
	default:
		e.state = stGround
	}
}

// param returns parameter i, or def when it is absent or zero - the
// convention every CSI sequence in this file follows.
func (e *Emulator) param(i, def int) int {
	if i >= len(e.params) || e.params[i] == 0 {
		return def
	}
	return e.params[i]
}

func (e *Emulator) csi(final byte) {
	if e.private != 0 {
		e.mode(final)
		return
	}
	switch final {
	case 'A':
		e.cur.move(0, -e.param(0, 1))
	case 'B':
		e.cur.move(0, e.param(0, 1))
	case 'C':
		e.cur.move(e.param(0, 1), 0)
	case 'D':
		e.cur.move(-e.param(0, 1), 0)
	case 'E':
		e.cur.moveTo(0, e.cur.y+e.param(0, 1))
	case 'F':
		e.cur.moveTo(0, e.cur.y-e.param(0, 1))
	case 'G', '`':
		e.cur.moveTo(e.param(0, 1)-1, e.cur.y)
	case 'H', 'f':
		e.cur.moveTo(e.param(1, 1)-1, e.param(0, 1)-1)
	case 'd':
		e.cur.moveTo(e.cur.x, e.param(0, 1)-1)
	case 'J':
		e.cur.eraseInDisplay(e.param(0, 0), e.style)
	case 'K':
		e.cur.eraseInLine(e.param(0, 0), e.style)
	case 'S':
		e.cur.scrollUp(e.param(0, 1))
	case 'T':
		e.cur.scrollDown(e.param(0, 1))
	case 'm':
		e.sgr()
	default:
		e.report(fmt.Sprintf("CSI %v %c", e.params, final))
	}
}

func (e *Emulator) report(seq string) {
	if e.Unknown != nil {
		e.Unknown(seq)
	}
}
