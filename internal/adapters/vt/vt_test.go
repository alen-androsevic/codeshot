package vt

import (
	"strings"
	"testing"
	"unicode/utf8"
)

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
	// mark is written as the \u030C escape, not typed, because a raw
	// combining sequence in source is visually indistinguishable from the
	// precomposed rune and would be silently collapsed by any tool that
	// normalises to NFC on save - leaving a test that covers nothing.
	const mark = "\u030C"
	const decomposed = "c" + mark + "ao"
	if got := mainText(t, 6, 2, decomposed); got != decomposed+"\n" {
		t.Errorf("got %q, want the mark kept on the c", got)
	}
	e := New(6, 2)
	e.Write([]byte(decomposed))
	line := e.Result().Main.Lines[0]
	if line[0].Rune != 'c' || line[0].Combining != mark {
		t.Errorf("cell 0 = %+v, want Rune 'c' with Combining %q", line[0], mark)
	}
	for i, c := range line {
		if c.Rune == []rune(mark)[0] {
			t.Errorf("cell %d holds the combining mark as its own rune; it must hang off the preceding cell instead of consuming one of its own", i)
		}
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

func TestUTF8RuneSplitAcrossWrites(t *testing.T) {
	// A pty delivers bytes in whatever chunks the kernel felt like handing
	// back, so a multi-byte rune can straddle two Write calls. "日" is three
	// bytes; splitting after the first and after the second exercises the
	// accumulator in both of its partial states, not just one.
	const r = "日"
	b := []byte(r)
	if len(b) != 3 {
		t.Fatalf("test setup: want a 3-byte rune, got %d bytes", len(b))
	}
	for _, split := range []int{1, 2} {
		e := New(10, 3)
		if _, err := e.Write(b[:split]); err != nil {
			t.Fatalf("split %d: first Write: %v", split, err)
		}
		if _, err := e.Write(b[split:]); err != nil {
			t.Fatalf("split %d: second Write: %v", split, err)
		}
		got := e.Result().Main.TrimTrailingBlank().Text()
		if got != r+"\n" {
			t.Errorf("split %d: got %q, want %q", split, got, r+"\n")
		}
		if strings.ContainsRune(got, utf8.RuneError) {
			t.Errorf("split %d: got a replacement character in %q, rune was not reassembled", split, got)
		}
	}
}

// TestALoneCombiningMarkSurvivesTrimming is the end-to-end half of the
// domain's TestCellCarryingACombiningMarkIsNotBlank: it proves the emulator
// really does produce a {Rune: ' ', Combining: …} cell, rather than that
// shape being a fixture invented to fit the fix. A combining mark with no
// base character before it - a decomposed string starting with whitespace,
// or a lone accent - hangs off the space the terminal is sitting on.
func TestALoneCombiningMarkSurvivesTrimming(t *testing.T) {
	e := New(10, 3)
	e.Write([]byte("x\r\n ́"))
	g := e.Result().Main
	cell := g.Lines[1][0]
	if cell.Rune != ' ' || cell.Combining != "́" {
		t.Fatalf("cell = %+v, want a space carrying the combining acute", cell)
	}
	if got := g.TrimTrailingBlank().Rows(); got != 2 {
		t.Errorf("TrimTrailingBlank left %d rows, want 2: the accent was thrown away", got)
	}
}
