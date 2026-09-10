package vt

import "testing"

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

func TestUnknownSequencesAreReportedNotDrawn(t *testing.T) {
	e := New(8, 2)
	var seen []string
	e.Unknown = func(s string) { seen = append(seen, s) }
	e.Write([]byte("a\x1b[5;9Zb"))
	if got := e.Result().Main.TrimTrailingBlank().Text(); got != "ab\n" {
		t.Errorf("got %q, want the sequence swallowed, not printed", got)
	}
	if len(seen) != 1 {
		t.Fatalf("Unknown called %d times, want once: %v", len(seen), seen)
	}
}
