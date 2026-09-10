package vt

import "testing"

// mainText is what the normal buffer looks like once the empty tail is gone.
func mainText(t *testing.T, cols, rows int, in string) string {
	t.Helper()
	e := New(cols, rows)
	if _, err := e.Write([]byte(in)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return e.Result().Main.TrimTrailingBlank().Text()
}

func TestPrintsPlainText(t *testing.T) {
	if got := mainText(t, 10, 3, "hello"); got != "hello\n" {
		t.Errorf("got %q, want %q", got, "hello\n")
	}
}

func TestLineFeedKeepsTheColumn(t *testing.T) {
	// A pty has already applied ONLCR, so a lone LF really does mean
	// "down one, same column".
	if got := mainText(t, 10, 3, "ab\ncd"); got != "ab\n  cd\n" {
		t.Errorf("got %q, want %q", got, "ab\n  cd\n")
	}
}

func TestCarriageReturnOverwrites(t *testing.T) {
	if got := mainText(t, 10, 3, "100%\rdone"); got != "done\n" {
		t.Errorf("got %q, want %q", got, "done\n")
	}
}

func TestBackspaceAndTab(t *testing.T) {
	if got := mainText(t, 20, 3, "abc\b\bX"); got != "aXc\n" {
		t.Errorf("backspace: got %q, want %q", got, "aXc\n")
	}
	if got := mainText(t, 20, 3, "a\tb"); got != "a       b\n" {
		t.Errorf("tab: got %q, want a then column 8", got)
	}
}

func TestAutowrapAtTheRightEdge(t *testing.T) {
	if got := mainText(t, 4, 3, "abcdef"); got != "abcd\nef\n" {
		t.Errorf("got %q, want %q", got, "abcd\nef\n")
	}
}

func TestWideRunesTakeTwoCellsAndNeverStraddleTheEdge(t *testing.T) {
	// Five columns cannot hold three double-width runes; the third wraps
	// rather than being split across the edge.
	got := mainText(t, 5, 3, "日本語")
	if got != "日本\n語\n" {
		t.Errorf("got %q, want %q", got, "日本\n語\n")
	}
}

func TestCombiningMarksAttachToThePrecedingCell(t *testing.T) {
	// Decomposed "c" + U+030C, which is how macOS hands over filenames. The
	// literal is escaped rather than typed, because the precomposed rune looks
	// identical in a source file and would test nothing.
	const decomposed = "čao"
	if got := mainText(t, 6, 2, decomposed); got != decomposed+"\n" {
		t.Errorf("got %q, want the mark kept on the c", got)
	}
	e := New(6, 2)
	e.Write([]byte(decomposed))
	if n := len(e.Result().Main.Lines[0]); n != 6 {
		t.Errorf("line length %d, want 6 cells: the mark must not consume one", n)
	}
}

func TestOutputTallerThanTheScreenKeepsScrollback(t *testing.T) {
	// Twenty columns, so nothing wraps and the staircase below is only the
	// line feeds keeping their column.
	got := mainText(t, 20, 2, "one\ntwo\nthree\nfour")
	want := "one\n   two\n      three\n           four\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
