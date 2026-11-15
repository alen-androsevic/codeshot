package plain

import (
	"bytes"
	"testing"
)

func strip(t *testing.T, chunks ...string) string {
	t.Helper()
	var out bytes.Buffer
	w := &Writer{W: &out}
	for _, c := range chunks {
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestStripsWhatThePtyAdded(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"sgr", "\x1b[34mcodeshot\x1b[39;49m\x1b[0m\r\n", "codeshot\n"},
		{"osc title with BEL", "\x1b]0;make\x07ok\r\n", "ok\n"},
		{"osc with ST", "\x1b]8;;https://x\x1b\\link\x1b]8;;\x1b\\\r\n", "link\n"},
		{"charset", "\x1b(Bline\r\n", "line\n"},
		{"keypad mode", "\x1b=a\x1b>b", "ab"},
		{"dcs", "\x1bPq#0\x1b\\after", "after"},
		{"progress bar keeps its \\r", "10%\r20%\r\n", "10%\r20%\n"},
		{"trailing \\r", "done\r", "done\r"},
		{"utf-8", "\x1b[1m中文 ❯\x1b[0m\r\n", "中文 ❯\n"},
	} {
		if got := strip(t, tc.in); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestKeepsStateAcrossWrites: a pty read can end anywhere, including in the
// middle of a sequence or between the \r and the \n.
func TestKeepsStateAcrossWrites(t *testing.T) {
	if got := strip(t, "a\x1b[3", "1mb\r", "\nc"); got != "ab\nc" {
		t.Errorf("%q, want %q", got, "ab\nc")
	}
}
