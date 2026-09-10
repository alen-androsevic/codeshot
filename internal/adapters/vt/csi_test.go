package vt

import (
	"testing"

	"codeshot/internal/domain"
)

func TestCursorPositionIsOneBased(t *testing.T) {
	// CUP row 2, column 3.
	if got := mainText(t, 8, 3, "\x1b[2;3Hx"); got != "\n  x\n" {
		t.Errorf("got %q, want x at row 2 column 3", got)
	}
}

func TestCursorPositionDefaultsToHome(t *testing.T) {
	if got := mainText(t, 8, 3, "abc\x1b[HX"); got != "Xbc\n" {
		t.Errorf("got %q, want the home position overwritten", got)
	}
}

func TestRelativeCursorMoves(t *testing.T) {
	if got := mainText(t, 10, 4, "abcd\x1b[2D\x1b[1AX"); got != "abXd\n" {
		t.Errorf("left-then-up: got %q, want the c overwritten and the d intact", got)
	}
	if got := mainText(t, 10, 4, "a\x1b[2BX"); got != "a\n\n X\n" {
		t.Errorf("down two: got %q", got)
	}
	if got := mainText(t, 10, 4, "\x1b[4CX"); got != "    X\n" {
		t.Errorf("forward four: got %q", got)
	}
}

// TestRelativeCursorMovesDefaultToOne pins the "no parameter means one cell"
// branch of A/B/C/D independently of TestRelativeCursorMoves, which only ever
// supplies an explicit count. The cursor starts and ends away from every
// edge, so clamping cannot make a step of 0 look like a step of 1 - only the
// default itself can explain the exact text each assertion checks for.
func TestRelativeCursorMovesDefaultToOne(t *testing.T) {
	if got := mainText(t, 10, 7, "\x1b[4;5H\x1b[AX"); got != "\n\n    X\n" {
		t.Errorf("bare A: got %q, want the cursor one row up from row 4", got)
	}
	if got := mainText(t, 10, 7, "\x1b[4;5H\x1b[BX"); got != "\n\n\n\n    X\n" {
		t.Errorf("bare B: got %q, want the cursor one row down from row 4", got)
	}
	if got := mainText(t, 10, 7, "\x1b[4;5H\x1b[CX"); got != "\n\n\n     X\n" {
		t.Errorf("bare C: got %q, want the cursor one column right of column 5", got)
	}
	if got := mainText(t, 10, 7, "\x1b[4;5H\x1b[DX"); got != "\n\n\n   X\n" {
		t.Errorf("bare D: got %q, want the cursor one column left of column 5", got)
	}
}

func TestCursorMovesClampToTheScreen(t *testing.T) {
	if got := mainText(t, 6, 2, "\x1b[99;99Hx"); got != "\n     x\n" {
		t.Errorf("got %q, want the cursor clamped to the last cell", got)
	}
}

func TestColumnAbsolute(t *testing.T) {
	if got := mainText(t, 8, 2, "abcdef\x1b[3GX"); got != "abXdef\n" {
		t.Errorf("got %q, want column 3 overwritten", got)
	}
}

func TestEraseInLine(t *testing.T) {
	if got := mainText(t, 8, 2, "abcdef\x1b[4G\x1b[K"); got != "abc\n" {
		t.Errorf("EL 0: got %q, want the tail erased", got)
	}
	if got := mainText(t, 8, 2, "abcdef\x1b[4G\x1b[1K"); got != "    ef\n" {
		t.Errorf("EL 1: got %q, want the head erased", got)
	}
	if got := mainText(t, 8, 2, "abcdef\x1b[2K"); got != "" {
		t.Errorf("EL 2: got %q, want the line gone", got)
	}
}

func TestEraseInDisplay(t *testing.T) {
	if got := mainText(t, 6, 3, "aaa\nbbb\x1b[1;2H\x1b[J"); got != "a\n" {
		t.Errorf("ED 0: got %q, want everything from the cursor gone", got)
	}
	if got := mainText(t, 6, 3, "aaa\nbbb\x1b[2J"); got != "" {
		t.Errorf("ED 2: got %q, want a blank screen", got)
	}
}

func TestScrollUpMovesTheScreenAndKeepsTheLine(t *testing.T) {
	// SU scrolls the visible area; on the normal buffer the line that leaves
	// the top is kept, because that is what scrollback is. So the proof that
	// the screen moved is where a write to row 1 now lands: on "b", not "a".
	got := mainText(t, 6, 3, "a\r\nb\r\nc\x1b[S\x1b[1;1HX")
	if got != "a\nX\nc\n" {
		t.Errorf("got %q, want a kept and the b overwritten", got)
	}
}

func TestScrollDownPushesTheBottomLineOff(t *testing.T) {
	// SD has nowhere to keep what falls off the bottom, so "c" is gone.
	got := mainText(t, 6, 3, "a\r\nb\r\nc\x1b[T\x1b[1;1HX")
	if got != "X\na\nb\n" {
		t.Errorf("got %q, want a blank line inserted at the top and c dropped", got)
	}
}

// TestUnknownSequencesAreReportedNotDrawn covers the design doc's promise
// that everything outside the supported set is "ignored silently and logged
// under --debug". Both halves matter and they are easy to get half right: an
// emulator that recognises the introducer but not the shape of what follows
// logs the sequence and then lets its tail spill into the grid as literal
// text, which is worse than not knowing about it at all - the picture then
// carries characters the terminal never showed.
//
// The cases below are the shapes real programs emit that a
// parameters-then-final-byte parser gets wrong, and each is drawn from
// something in daily use, not invented: `tput sgr0` emits ESC ( B before its
// SGR reset, so every terminfo-based colour reset used to leak a stray B;
// rustc, delta and anything kitty-aware use colon sub-parameters; and a DCS
// string is how a terminal answers a capability query.
func TestUnknownSequencesAreReportedNotDrawn(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"unknown CSI final byte", "a\x1b[5;9Zb", "ab\n"},
		{"charset designator", "ok\x1b(B\x1b[m plain", "ok plain\n"},
		{"colon sub-parameters", "a\x1b[4:3mb\x1b[0mc", "abc\n"},
		{"colon-form underline colour", "a\x1b[58:2::255:0:0mb", "ab\n"},
		{"DCS string terminated by ST", "a\x1bP1$r0m\x1b\\b", "ab\n"},
		{"APC string terminated by ST", "a\x1b_Gf=100\x1b\\b", "ab\n"},
		{"PM string terminated by ST", "a\x1b^private\x1b\\b", "ab\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := New(20, 2)
			var seen []string
			e.Unknown = func(s string) { seen = append(seen, s) }
			e.Write([]byte(c.input))
			if got := e.Result().Main.TrimTrailingBlank().Text(); got != c.want {
				t.Errorf("got %q, want %q: the sequence must be swallowed, not printed", got, c.want)
			}
			if len(seen) == 0 {
				t.Error("nothing reported through Unknown; an ignored sequence must still be logged")
			}
		})
	}
}

// TestUnrecognisableCSIIsSwallowedToItsFinalByte pins the CSI-ignore state
// separately from the table above, because a CSI whose parameter area holds a
// byte no parser can make sense of is the one case where the sequence has
// already gone wrong before its final byte arrives. A parser that gives up
// and returns to ground at that point prints the rest of the sequence -
// its final byte included - as text.
func TestUnrecognisableCSIIsSwallowedToItsFinalByte(t *testing.T) {
	e := New(20, 2)
	e.Write([]byte("a\x1b[1\x7f2mb"))
	if got := e.Result().Main.TrimTrailingBlank().Text(); got != "ab\n" {
		t.Errorf("got %q, want the whole CSI swallowed up to its final byte", got)
	}
}

// TestColonSubParametersDoNotBecomeParameters checks that a colon-form
// underline still underlines. Swallowing 4:3 by treating the colon as a
// plain separator would hand SGR the parameter list [4 3] - underline *and*
// italic - so the grid would come out styled with an attribute the stream
// never asked for. The digits after a colon belong to the parameter before
// it, and codeshot renders only the base attribute.
func TestColonSubParametersDoNotBecomeParameters(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("\x1b[4:3mx"))
	cell := e.Result().Main.Lines[0][0]
	if !cell.Style.Has(domain.AttrUnderline) {
		t.Error("4:3 did not underline; the base parameter was lost")
	}
	if cell.Style.Has(domain.AttrItalic) {
		t.Error("4:3 turned on italic; the sub-parameter 3 was read as a parameter of its own")
	}
}

// TestEraseFillsWithTheCurrentBackground pins behaviour real programs depend
// on and nothing checked: an erase paints the cells it clears in whatever
// background colour is current, so `ESC[41m ESC[2J` leaves a red screen, not
// a default-coloured one. less's status bar, and any TUI that paints a panel
// by setting a background and clearing, is built on this. blank() keeps the
// background and drops every other attribute, so the check below is two
// assertions, not one: the colour survives and the bold does not.
func TestEraseFillsWithTheCurrentBackground(t *testing.T) {
	red := domain.IndexedColor(1)
	t.Run("erase in display", func(t *testing.T) {
		e := New(4, 2)
		e.Write([]byte("\x1b[1;41m\x1b[2J"))
		for y, line := range e.Result().Main.Lines {
			for x, cell := range line {
				if cell.Style.BG != red {
					t.Fatalf("cell (%d,%d) background = %+v, want the red that was current when it was erased", x, y, cell.Style.BG)
				}
				if cell.Style.Has(domain.AttrBold) {
					t.Fatalf("cell (%d,%d) kept its bold; an erased cell holds the background and nothing else", x, y)
				}
			}
		}
	})
	t.Run("erase in line", func(t *testing.T) {
		e := New(4, 2)
		e.Write([]byte("abcd\x1b[41m\x1b[1;1H\x1b[K"))
		for x, cell := range e.Result().Main.Lines[0] {
			if cell.Style.BG != red {
				t.Fatalf("cell %d background = %+v, want red", x, cell.Style.BG)
			}
		}
	})
}
