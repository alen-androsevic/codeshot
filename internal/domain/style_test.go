package domain

import "testing"

func TestColorConstructorsCarryTheirKind(t *testing.T) {
	if got := DefaultColor(); got.Kind != ColorDefault {
		t.Errorf("DefaultColor kind = %v, want ColorDefault", got.Kind)
	}
	if got := IndexedColor(9); got.Kind != ColorIndexed || got.Index != 9 {
		t.Errorf("IndexedColor(9) = %+v, want indexed 9", got)
	}
	got := RGBColor(0x1D, 0x1F, 0x21)
	if got.Kind != ColorRGB || got.R != 0x1D || got.G != 0x1F || got.B != 0x21 {
		t.Errorf("RGBColor = %+v, want rgb 1d1f21", got)
	}
}

func TestStyleAttributesSetAndClearIndependently(t *testing.T) {
	s := Style{}.Set(AttrBold).Set(AttrUnderline)
	if !s.Has(AttrBold) || !s.Has(AttrUnderline) {
		t.Fatalf("Set lost an attribute: %b", s.Attrs)
	}
	if s.Has(AttrItalic) {
		t.Errorf("italic set without being asked for")
	}
	s = s.Clear(AttrBold)
	if s.Has(AttrBold) || !s.Has(AttrUnderline) {
		t.Errorf("Clear(AttrBold) = %b, want underline only", s.Attrs)
	}
}

func TestStyleValueSemantics(t *testing.T) {
	base := Style{}
	base.Set(AttrBold)
	if base.Has(AttrBold) {
		t.Error("Set mutated the receiver; Style must be a value type")
	}
}
