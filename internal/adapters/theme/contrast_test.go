package theme

import (
	"math"
	"testing"

	"codeshot/internal/domain"
)

// TestEmbeddedThemesAreReadable holds codeshot's own themes to the 4.5:1 a
// line of text needs against its background. Palette 0 of a dark theme is
// the one exception: it is black, there to paint backgrounds with.
func TestEmbeddedThemesAreReadable(t *testing.T) {
	for _, name := range []string{"codeshot-dark", "codeshot-light"} {
		th, err := Source{}.Theme(name)
		if err != nil {
			t.Fatal(err)
		}
		dark := luminance(th.Background) < 0.5
		if c := contrast(th.Foreground, th.Background); c < 4.5 {
			t.Errorf("%s: foreground is %.2f:1", name, c)
		}
		for i, p := range th.Palette {
			if i == 0 && dark {
				continue
			}
			if c := contrast(p, th.Background); c < 4.5 {
				t.Errorf("%s: palette %d is %.2f:1", name, i, c)
			}
		}
	}
}

func luminance(c domain.RGBA) float64 {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func contrast(a, b domain.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
