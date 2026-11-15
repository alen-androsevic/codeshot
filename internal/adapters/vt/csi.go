package vt

import "fmt"

// escape drives everything after ESC. Every state here exists for one
// reason: a sequence codeshot does not implement must still be consumed to
// its own end. Dropping back to ground on the first byte that makes no sense
// leaves the rest of the sequence to be printed as literal text, which puts
// characters in the picture the terminal never showed - a worse outcome than
// not understanding the sequence at all. So each shape a real program emits
// gets a state that knows where it ends: a CSI's final byte, an OSC's
// terminator, an intermediate-byte escape's single following byte, and a
// string escape's ST.
func (e *Emulator) escape(b byte) {
	switch e.state {
	case stEsc:
		switch {
		case b == '[':
			e.state = stCSI
			e.params = e.params[:0]
			e.private = 0
			e.subparm = false
			e.subseen = false
		case b == ']':
			e.state = stOSC
			e.osc = e.osc[:0]
		case b >= 0x20 && b <= 0x2F:
			// A charset designator and its relatives: ESC ( B, ESC ) 0,
			// ESC # 8. ncurses, less and vim all emit them, and `tput sgr0`
			// puts ESC ( B in front of its SGR reset, so getting this wrong
			// leaks a stray B out of every terminfo-driven colour reset.
			e.inter = append(e.inter[:0], b)
			e.state = stEscInter
		case b == 'P' || b == '^' || b == '_' || b == 'X':
			// DCS, PM, APC and SOS each open a string that runs until ST.
			// The payload is arbitrary text - a terminfo query reply, a tmux
			// passthrough, a kitty graphics blob - and none of it is screen
			// content.
			e.strKind = b
			e.state = stString
		default:
			e.report(fmt.Sprintf("ESC %c", b))
			e.state = stGround
		}
	case stEscInter:
		if b >= 0x20 && b <= 0x2F {
			e.inter = append(e.inter, b)
			return
		}
		e.report(fmt.Sprintf("ESC %s %c", e.inter, b))
		e.state = stGround
	case stCSI, stCSIIgnore:
		e.csiByte(b)
	case stOSC, stOSCEsc:
		e.oscByte(b)
	case stString, stStringEsc:
		e.stringByte(b)
	}
}

func (e *Emulator) csiByte(b byte) {
	if e.state == stCSIIgnore {
		if b >= 0x40 && b <= 0x7E {
			e.report(fmt.Sprintf("CSI (unparseable) %c", b))
			e.state = stGround
		}
		return
	}
	switch {
	case b >= '0' && b <= '9':
		// Digits belonging to a sub-parameter are dropped rather than
		// accumulated; see the colon case below for why.
		if e.subparm {
			return
		}
		if len(e.params) == 0 {
			e.params = append(e.params, 0)
		}
		e.params[len(e.params)-1] = e.params[len(e.params)-1]*10 + int(b-'0')
	case b == ';':
		e.subparm = false
		e.params = append(e.params, 0)
	case b == ':':
		// A colon introduces sub-parameters of the parameter before it -
		// SGR 4:3 for a curly underline, 58:2::r:g:b for an underline
		// colour - and rustc, delta and anything kitty-aware emit them.
		// codeshot draws none of the variants they select, so the base
		// parameter is kept and the sub-parameters are discarded. They must
		// not become parameters in their own right: 4:3 would then reach SGR
		// as [4 3] and turn on italic as well as underline.
		e.subparm = true
		e.subseen = true
	case b >= '<' && b <= '?':
		e.private = b
	case b >= 0x20 && b <= 0x2F:
		// Intermediate bytes; codeshot needs none of the sequences that use
		// them, but they must not end the sequence either.
	case b >= 0x40 && b <= 0x7E:
		if e.subseen {
			// The sequence itself is honoured, but the variant its
			// sub-parameters asked for is not, and --debug should say so.
			e.report(fmt.Sprintf("CSI %v %c sub-parameters", e.params, b))
		}
		e.csi(b)
		e.state = stGround
	default:
		// A byte that belongs nowhere in a CSI - a stray control byte, a DEL.
		// The sequence is already lost, but its final byte still marks where
		// it ends, so keep reading until then.
		e.state = stCSIIgnore
	}
}

// stringByte swallows a DCS, PM, APC or SOS payload up to its string
// terminator. Like oscByte, it treats an ESC not followed by a backslash as
// the end of a malformed string rather than trying to resynchronise.
func (e *Emulator) stringByte(b byte) {
	switch {
	case e.state == stStringEsc:
		e.state = stGround
		if b == '\\' {
			e.report(fmt.Sprintf("ESC %c string", e.strKind))
		}
	case b == 0x07:
		// xterm accepts BEL in place of ST for string escapes, as for OSC.
		e.state = stGround
		e.report(fmt.Sprintf("ESC %c string", e.strKind))
	case b == 0x1B:
		e.state = stStringEsc
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
