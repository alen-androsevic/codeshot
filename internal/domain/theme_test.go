package domain

import "testing"

func testTheme() Theme {
	t := Theme{
		Name:       "test",
		Background: RGBA{0x10, 0x10, 0x10, 0xFF},
		Foreground: RGBA{0xE0, 0xE0, 0xE0, 0xFF},
	}
	t.Palette[1] = RGBA{0xFF, 0x00, 0x00, 0xFF}
	t.Palette[4] = RGBA{0x00, 0x00, 0xFF, 0xFF}
	return t
}

func TestResolveUsesThemeForDefaults(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{})
	if fg != th.Foreground || bg != th.Background {
		t.Errorf("Resolve(zero) = %v/%v, want theme fg/bg", fg, bg)
	}
}

func TestResolveUsesPaletteForLowIndices(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), BG: IndexedColor(4)})
	if fg != th.Palette[1] || bg != th.Palette[4] {
		t.Errorf("Resolve(indexed) = %v/%v, want palette 1/4", fg, bg)
	}
}

func TestResolveIgnoresThemeForTrueColour(t *testing.T) {
	th := testTheme()
	fg, _ := th.Resolve(Style{FG: RGBColor(1, 2, 3)})
	if (fg != RGBA{1, 2, 3, 0xFF}) {
		t.Errorf("Resolve(rgb) = %v, want 1,2,3 opaque", fg)
	}
}

func TestInverseSwapsAfterResolution(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), Attrs: AttrInverse})
	if fg != th.Background || bg != th.Palette[1] {
		t.Errorf("inverse = %v/%v, want bg/palette1", fg, bg)
	}
}

func TestDimBlendsForegroundTowardBackground(t *testing.T) {
	th := Theme{Background: RGBA{0, 0, 0, 0xFF}, Foreground: RGBA{0xFF, 0xFF, 0xFF, 0xFF}}
	fg, _ := th.Resolve(Style{Attrs: AttrDim})
	if fg.R < 0x7D || fg.R > 0x81 {
		t.Errorf("dim white on black = %v, want about half", fg)
	}
}

func TestHiddenCollapsesForegroundIntoBackground(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), Attrs: AttrHidden})
	if fg != bg {
		t.Errorf("hidden fg %v != bg %v", fg, bg)
	}
}

func TestXTerm256Cube(t *testing.T) {
	cases := map[uint8]RGBA{
		16:  {0x00, 0x00, 0x00, 0xFF},
		21:  {0x00, 0x00, 0xFF, 0xFF},
		231: {0xFF, 0xFF, 0xFF, 0xFF},
		232: {0x08, 0x08, 0x08, 0xFF},
		255: {0xEE, 0xEE, 0xEE, 0xFF},
	}
	for i, want := range cases {
		if got := XTerm256(i); got != want {
			t.Errorf("XTerm256(%d) = %v, want %v", i, got, want)
		}
	}
}

func TestResolveFallsBackToXTermForHighIndices(t *testing.T) {
	th := testTheme()
	fg, _ := th.Resolve(Style{FG: IndexedColor(21)})
	if (fg != RGBA{0x00, 0x00, 0xFF, 0xFF}) {
		t.Errorf("index 21 = %v, want pure blue from the cube", fg)
	}
}
