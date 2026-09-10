package vt

import (
	"testing"

	"codeshot/internal/domain"
)

// styleAt runs the input and returns the style of the cell at row 0, column 0.
func styleAt(t *testing.T, in string) domain.Style {
	t.Helper()
	e := New(20, 2)
	e.Write([]byte(in))
	return e.Result().Main.Lines[0][0].Style
}

func TestSGRAttributes(t *testing.T) {
	s := styleAt(t, "\x1b[1;3;4;7mx")
	for _, a := range []domain.Attr{domain.AttrBold, domain.AttrItalic, domain.AttrUnderline, domain.AttrInverse} {
		if !s.Has(a) {
			t.Errorf("attribute %b missing from %b", a, s.Attrs)
		}
	}
}

func TestSGRResetClearsEverything(t *testing.T) {
	if s := styleAt(t, "\x1b[1;31m\x1b[0mx"); s != (domain.Style{}) {
		t.Errorf("SGR 0 left %+v, want the zero style", s)
	}
}

func TestSGREmptyParameterIsReset(t *testing.T) {
	if s := styleAt(t, "\x1b[1;31m\x1b[mx"); s != (domain.Style{}) {
		t.Errorf("bare CSI m left %+v, want the zero style", s)
	}
}

func TestSGRBasicAndBrightColours(t *testing.T) {
	if s := styleAt(t, "\x1b[31;44mx"); s.FG != domain.IndexedColor(1) || s.BG != domain.IndexedColor(4) {
		t.Errorf("got %+v, want fg 1 bg 4", s)
	}
	if s := styleAt(t, "\x1b[92;101mx"); s.FG != domain.IndexedColor(10) || s.BG != domain.IndexedColor(9) {
		t.Errorf("got %+v, want bright fg 10 bg 9", s)
	}
	if s := styleAt(t, "\x1b[31;39mx"); s.FG != domain.DefaultColor() {
		t.Errorf("SGR 39 left %+v, want the default foreground", s)
	}
}

func TestSGR256AndTrueColour(t *testing.T) {
	if s := styleAt(t, "\x1b[38;5;208mx"); s.FG != domain.IndexedColor(208) {
		t.Errorf("got %+v, want indexed 208", s)
	}
	if s := styleAt(t, "\x1b[48;2;10;20;30mx"); s.BG != domain.RGBColor(10, 20, 30) {
		t.Errorf("got %+v, want rgb 10,20,30", s)
	}
}

func TestSGRTrueColourDoesNotLeakIntoLaterParameters(t *testing.T) {
	// The 1 after the triple is bold, not a colour index.
	s := styleAt(t, "\x1b[38;2;10;20;30;1mx")
	if s.FG != domain.RGBColor(10, 20, 30) || !s.Has(domain.AttrBold) {
		t.Errorf("got %+v, want rgb plus bold", s)
	}
}

func TestSGRIndividualResets(t *testing.T) {
	if s := styleAt(t, "\x1b[1;4m\x1b[22;24mx"); s.Has(domain.AttrBold) || s.Has(domain.AttrUnderline) {
		t.Errorf("got %b, want bold and underline cleared", s.Attrs)
	}
}

func TestSGRMalformedExtendedColourIsIgnored(t *testing.T) {
	// A truncated 38;5 must not panic and must not eat the next command.
	if s := styleAt(t, "\x1b[38;5m\x1b[1mx"); !s.Has(domain.AttrBold) {
		t.Errorf("got %+v, want the following bold to still apply", s)
	}
}
