package vt

import (
	"fmt"

	"codeshot/internal/domain"
)

// sgr applies Select Graphic Rendition parameters to the current style. The
// loop consumes extra parameters itself for 38 and 48, which is the only place
// SGR is not one-parameter-one-effect.
func (e *Emulator) sgr() {
	params := e.params
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			e.style = domain.Style{}
		case p == 1:
			e.style = e.style.Set(domain.AttrBold)
		case p == 2:
			e.style = e.style.Set(domain.AttrDim)
		case p == 3:
			e.style = e.style.Set(domain.AttrItalic)
		case p == 4:
			e.style = e.style.Set(domain.AttrUnderline)
		case p == 7:
			e.style = e.style.Set(domain.AttrInverse)
		case p == 8:
			e.style = e.style.Set(domain.AttrHidden)
		case p == 9:
			e.style = e.style.Set(domain.AttrStrike)
		case p == 22:
			e.style = e.style.Clear(domain.AttrBold | domain.AttrDim)
		case p == 23:
			e.style = e.style.Clear(domain.AttrItalic)
		case p == 24:
			e.style = e.style.Clear(domain.AttrUnderline)
		case p == 27:
			e.style = e.style.Clear(domain.AttrInverse)
		case p == 28:
			e.style = e.style.Clear(domain.AttrHidden)
		case p == 29:
			e.style = e.style.Clear(domain.AttrStrike)
		case p >= 30 && p <= 37:
			e.style.FG = domain.IndexedColor(uint8(p - 30))
		case p == 38:
			if c, next, ok := extendedColor(params, i); ok {
				e.style.FG, i = c, next
			} else {
				return
			}
		case p == 39:
			e.style.FG = domain.DefaultColor()
		case p >= 40 && p <= 47:
			e.style.BG = domain.IndexedColor(uint8(p - 40))
		case p == 48:
			if c, next, ok := extendedColor(params, i); ok {
				e.style.BG, i = c, next
			} else {
				return
			}
		case p == 49:
			e.style.BG = domain.DefaultColor()
		case p >= 90 && p <= 97:
			e.style.FG = domain.IndexedColor(uint8(p - 90 + 8))
		case p >= 100 && p <= 107:
			e.style.BG = domain.IndexedColor(uint8(p - 100 + 8))
		default:
			e.report(fmt.Sprintf("SGR %d", p))
		}
	}
}

// extendedColor reads the 5;n or 2;r;g;b form that follows a 38 or 48. It
// returns the index of the last parameter it consumed, so the caller's loop
// carries on with whatever came after the colour.
func extendedColor(params []int, i int) (domain.Color, int, bool) {
	if i+1 >= len(params) {
		return domain.Color{}, i, false
	}
	switch params[i+1] {
	case 5:
		if i+2 >= len(params) {
			return domain.Color{}, i, false
		}
		return domain.IndexedColor(uint8(params[i+2])), i + 2, true
	case 2:
		if i+4 >= len(params) {
			return domain.Color{}, i, false
		}
		return domain.RGBColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4])), i + 4, true
	}
	return domain.Color{}, i, false
}
