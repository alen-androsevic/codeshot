package raster

import (
	"bytes"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the golden PNGs")

func TestShadowDarkensOutsideTheWindow(t *testing.T) {
	g := domain.Grid{Cols: 20, Lines: [][]domain.Cell{cells("shadow", domain.Style{})}}
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Margin = 40
	c.Title = "x"
	withShadow := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})
	c.Shadow = false
	without := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})

	// A point just below the window, inside the margin.
	x, y := withShadow.Bounds().Dx()/2, withShadow.Bounds().Dy()-20
	_, _, _, a1 := withShadow.At(x, y).RGBA()
	_, _, _, a2 := without.At(x, y).RGBA()
	if a1 == 0 {
		t.Error("no shadow below the window")
	}
	if a2 != 0 {
		t.Error("something is painted in the margin with the shadow off")
	}
	if a1 == 0xFFFF {
		t.Error("the shadow is fully opaque; it must fade")
	}
}

func TestShadowLeavesTheImageEdgeClear(t *testing.T) {
	g := domain.Grid{Cols: 20, Lines: [][]domain.Cell{cells("shadow", domain.Style{})}}
	c := domain.DefaultChrome()
	c.Scale = 1
	img := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("image corner alpha = %d, want the 64px margin to contain the blur", a)
	}
}

func TestGoldenShots(t *testing.T) {
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]domain.Window{
		"plain":  goldenWindow(false),
		"styled": goldenWindow(true),
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			img, err := New(set, DefaultOptions()).Render(w)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", name+".png")
			if *update {
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run: go test ./internal/adapters/render/raster/ -update)", err)
			}
			if !bytes.Equal(want, buf.Bytes()) {
				got := filepath.Join(t.TempDir(), name+".got.png")
				os.WriteFile(got, buf.Bytes(), 0o644)
				t.Errorf("rendering changed; compare %s with %s", path, got)
			}
		})
	}
}

func goldenWindow(styled bool) domain.Window {
	th := testTheme()
	th.Background = domain.RGBA{0x15, 0x18, 0x1D, 0xFF}
	th.Foreground = domain.RGBA{0xC3, 0xC8, 0xD1, 0xFF}
	line := cells("$ codeshot render session.ansi", domain.Style{})
	if styled {
		st := domain.Style{FG: domain.IndexedColor(4)}.Set(domain.AttrBold)
		line = cells("$ codeshot render session.ansi", st)
	}
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Title = "codeshot"
	return domain.Window{
		Frame:  domain.Frame{Grid: domain.Grid{Cols: 34, Lines: [][]domain.Cell{line, cells("done.", domain.Style{})}}},
		Theme:  th,
		Chrome: c,
	}
}
