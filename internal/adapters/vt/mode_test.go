package vt

import (
	"strings"
	"testing"

	"codeshot/internal/domain"
)

func TestAlternateScreenIsSeparateFromTheNormalOne(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("normal\x1b[?1049hTUI"))
	r := e.Result()
	if !r.UsedAlt {
		t.Fatal("UsedAlt is false after entering the alternate screen")
	}
	if got := strings.TrimSpace(r.Alt.Text()); got != "TUI" {
		t.Errorf("alt = %q, want TUI", got)
	}
	if !strings.Contains(r.Main.Text(), "normal") {
		t.Errorf("main lost its content: %q", r.Main.Text())
	}
}

func TestLeavingTheAlternateScreenReturnsToNormal(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("normal\x1b[?1049hTUI\x1b[?1049l"))
	r := e.Result()
	if r.UsedAlt {
		t.Error("UsedAlt is true although the program restored the normal screen")
	}
	if !strings.Contains(r.Main.Text(), "normal") {
		t.Errorf("main = %q, want the pre-TUI content back", r.Main.Text())
	}
}

func TestOSCSetsTheTitle(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("\x1b]0;paradajz danas\x07x"))
	if got := e.Result().Title; got != "paradajz danas" {
		t.Errorf("Title = %q", got)
	}
	if got := strings.TrimSpace(e.Result().Main.Text()); got != "x" {
		t.Errorf("main = %q, want the OSC swallowed", got)
	}
}

func TestOSCTerminatedByStringTerminator(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("\x1b]2;title\x1b\\x"))
	if got := e.Result().Title; got != "title" {
		t.Errorf("Title = %q", got)
	}
}

func TestAutowrapCanBeTurnedOff(t *testing.T) {
	// With DECAWM off the last column is overwritten instead of wrapping.
	if got := mainText(t, 4, 2, "\x1b[?7labcdef"); got != "abcf\n" {
		t.Errorf("got %q, want the tail overwriting the last column", got)
	}
}

func TestUnknownPrivateModesAreReported(t *testing.T) {
	e := New(8, 2)
	var seen []string
	e.Unknown = func(s string) { seen = append(seen, s) }
	e.Write([]byte("\x1b[?2004hx"))
	if len(seen) != 1 {
		t.Errorf("Unknown called %d times, want once for bracketed paste", len(seen))
	}
}

func TestAdapterEmulatesACapture(t *testing.T) {
	var a Adapter
	r, err := a.Emulate(domain.Capture{Cols: 8, Rows: 2, Bytes: []byte("hi")})
	if err != nil {
		t.Fatalf("Emulate: %v", err)
	}
	if got := strings.TrimSpace(r.Main.Text()); got != "hi" {
		t.Errorf("Main = %q", got)
	}
}

func TestAdapterFallsBackToASensibleSizeWhenTheCaptureHasNone(t *testing.T) {
	var a Adapter
	r, err := a.Emulate(domain.Capture{Bytes: []byte("hi")})
	if err != nil {
		t.Fatalf("Emulate: %v", err)
	}
	if r.Main.Cols != 100 {
		t.Errorf("Cols = %d, want the 100-column fallback", r.Main.Cols)
	}
}

func TestAlternateScreenClearsOnReEntry(t *testing.T) {
	// Enter alt, write, leave, then re-enter. The new alt must be blank,
	// not holding the old content. This guards against setAlt failing to
	// reallocate the buffer on entry.
	e := New(8, 3)
	e.Write([]byte("main1\r\n\x1b[?1049h")) // Enter alt on clean buffer
	e.Write([]byte("oldalt"))               // Write to alt
	e.Write([]byte("\x1b[?1049l"))          // Leave alt, return to main
	e.Write([]byte("\r\nmain2"))            // Add more to main (new line)
	e.Write([]byte("\x1b[?1049h"))          // Re-enter alt
	r := e.Result()

	// The new alt must be blank, not holding "oldalt"
	altText := strings.TrimSpace(r.Alt.Text())
	if altText != "" {
		t.Errorf("alt on re-entry = %q, want blank (old content should be cleared)", altText)
	}

	// Main must still have both writes
	mainText := r.Main.Text()
	if !strings.Contains(mainText, "main1") {
		t.Errorf("main lost first write: %q", mainText)
	}
	if !strings.Contains(mainText, "main2") {
		t.Errorf("main lost second write: %q", mainText)
	}
}
