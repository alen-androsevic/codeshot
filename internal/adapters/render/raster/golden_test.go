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

// shadowGeometryFixture mirrors the fixture TestShadowDarkensOutsideTheWindow
// builds inline. It is a separate copy, not a shared extraction, because the
// two tests below need the layout's window rectangle as well as the
// rendered image, and reach for both through a freshly constructed Renderer
// rather than the render(t, w) test helper.
func shadowGeometryFixture() domain.Window {
	g := domain.Grid{Cols: 20, Lines: [][]domain.Cell{cells("shadow", domain.Style{})}}
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Margin = 40
	c.Title = "x"
	return domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c}
}

// TestShadowOffsetPushesTheShadowDownward exercises the actual render.go call
// site (via Render, not by calling drawShadow directly with hand-picked
// arguments), so a regression in the offsetY literal there is caught, not
// just a bug in drawShadow's own math.
//
// TestShadowDarkensOutsideTheWindow's single checkpoint (image bottom minus
// 20px) sits far enough into the blur's tail that it stayed non-zero and
// non-opaque even with offsetY zeroed - the box blur's 3-pass support is
// wide enough to still leak a little shadow there regardless of the offset,
// so "not zero, not fully opaque" never actually pinned the offset. What the
// offset alone produces, that a symmetric (unoffset) blur cannot, is
// asymmetry: the shadow must be substantially darker a fixed distance below
// the window's bottom edge than the same distance above its top edge. That
// asymmetry is the checkpoint here, picked from the geometry (the two edges)
// rather than one arbitrary pixel.
func TestShadowOffsetPushesTheShadowDownward(t *testing.T) {
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	r := New(set, DefaultOptions())
	w := shadowGeometryFixture()
	l, err := r.layout(w)
	if err != nil {
		t.Fatal(err)
	}
	img, err := r.Render(w)
	if err != nil {
		t.Fatal(err)
	}
	x := (l.window.Min.X + l.window.Max.X) / 2
	const d = 5
	_, _, _, above := img.At(x, l.window.Min.Y-d).RGBA()
	_, _, _, below := img.At(x, l.window.Max.Y+d).RGBA()
	if below <= 3*above {
		t.Errorf("%dpx below the window alpha = %d, %dpx above = %d; a downward offset must weight the shadow toward the bottom", d, below, d, above)
	}
}

// TestShadowAlphaFadesNearTheWindow checks the darkness of the shadow right
// at the window's own bottom edge - the strongest, most reliable signal the
// shadow produces, because that point sits deep enough inside the offset
// silhouette that the mask is close to fully opaque there before the blur
// softens it. TestShadowDarkensOutsideTheWindow's checkpoint, by contrast,
// sits far out in the blur's tail, where a correct alpha of 0.35 and a
// regressed alpha of 1.0 both land on small values neither "not zero" nor
// "not fully opaque" can tell apart. At the window's edge the gap is wide: at
// alpha 0.35 the point lands around 20000/65535; at alpha 1.0 it roughly
// triples past 58000.
func TestShadowAlphaFadesNearTheWindow(t *testing.T) {
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	r := New(set, DefaultOptions())
	w := shadowGeometryFixture()
	l, err := r.layout(w)
	if err != nil {
		t.Fatal(err)
	}
	img, err := r.Render(w)
	if err != nil {
		t.Fatal(err)
	}
	x := (l.window.Min.X + l.window.Max.X) / 2
	_, _, _, below := img.At(x, l.window.Max.Y).RGBA()
	if below == 0 {
		t.Fatal("no shadow at the window's own bottom edge")
	}
	if below > 0x9000 {
		t.Errorf("shadow alpha at the window's bottom edge = %d, want it to stay a fraction of full strength instead of saturating", below)
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

// segment is a run of text in one style: the unit the golden fixtures are
// built from. wide says every rune in the run is double-width, stated rather
// than guessed so the fixture does not depend on a width table.
type segment struct {
	text  string
	style domain.Style
	wide  bool
}

// goldenLine lays segments out as cells, giving a double-width rune the two
// cells it occupies - a leading cell holding the rune and a width-0
// continuation behind it, which is the shape vt/buffer.go produces and the
// shape the renderer's wide-rune handling expects.
func goldenLine(cols int, segs ...segment) []domain.Cell {
	line := make([]domain.Cell, 0, cols)
	for _, seg := range segs {
		for _, r := range seg.text {
			if seg.wide {
				line = append(line,
					domain.Cell{Rune: r, Style: seg.style, Width: 2},
					domain.Cell{Style: seg.style, Width: 0})
				continue
			}
			line = append(line, domain.Cell{Rune: r, Style: seg.style, Width: 1})
		}
	}
	for len(line) < cols {
		line = append(line, domain.Cell{Rune: ' ', Width: 1})
	}
	return line
}

// goldenWindow builds the two fixtures the golden PNGs are made from. The
// styled one is deliberately busy: before it was enriched, neither fixture
// contained a single coloured background, so the run-merging pass, inverse,
// dim and the two-cell layout of a wide rune were all rendered by code no
// golden had ever looked at.
//
// The wide rune is CJK, which JetBrains Mono NL does not cover, so it comes
// out as the font's .notdef box. That is not an oversight in the fixture -
// it is what codeshot genuinely draws today, and the design doc says so
// (no fallback chain until a later phase). Pinning it means the day a
// fallback font arrives, this golden changes and someone has to look at it.
func goldenWindow(styled bool) domain.Window {
	th := testTheme()
	th.Background = domain.RGBA{R: 0x15, G: 0x18, B: 0x1D, A: 0xFF}
	th.Foreground = domain.RGBA{R: 0xC3, G: 0xC8, B: 0xD1, A: 0xFF}
	// A palette wide enough for the fixture to use more than one hue.
	// Palette[0] is set explicitly: an unset entry is the zero RGBA, which is
	// transparent, and text drawn in it vanishes - which is exactly what the
	// first cut of this fixture did to the text on the green run.
	th.Palette[0] = domain.RGBA{R: 0x15, G: 0x18, B: 0x1D, A: 0xFF}
	th.Palette[1] = domain.RGBA{R: 0xE5, G: 0x53, B: 0x5F, A: 0xFF}
	th.Palette[2] = domain.RGBA{R: 0x6E, G: 0xC1, B: 0x77, A: 0xFF}
	th.Palette[3] = domain.RGBA{R: 0xE0, G: 0xAF, B: 0x68, A: 0xFF}
	th.Palette[4] = domain.RGBA{R: 0x61, G: 0x9A, B: 0xE8, A: 0xFF}
	th.Palette[7] = domain.RGBA{R: 0xE8, G: 0xEC, B: 0xF2, A: 0xFF}

	const cols = 34
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Title = "codeshot"

	var lines [][]domain.Cell
	if !styled {
		lines = [][]domain.Cell{
			cells("$ codeshot render session.ansi", domain.Style{}),
			cells("done.", domain.Style{}),
		}
	} else {
		bold := domain.Style{FG: domain.IndexedColor(4)}.Set(domain.AttrBold)
		// A background run three cells wide, with a second run of a different
		// colour hard against it, so the boundary between two fills is in the
		// picture and not only in a unit test.
		onRed := domain.Style{FG: domain.IndexedColor(7), BG: domain.IndexedColor(1)}
		onGreen := domain.Style{FG: domain.IndexedColor(0), BG: domain.IndexedColor(2)}
		lines = [][]domain.Cell{
			goldenLine(cols, segment{text: "$ codeshot render session.ansi", style: bold}),
			goldenLine(cols,
				segment{text: " FAIL ", style: onRed},
				segment{text: " PASS ", style: onGreen},
				segment{text: " 2 of 3", style: domain.Style{FG: domain.IndexedColor(3)}}),
			goldenLine(cols,
				segment{text: "inverse", style: domain.Style{}.Set(domain.AttrInverse)},
				segment{text: " "},
				segment{text: "dim", style: domain.Style{}.Set(domain.AttrDim)},
				segment{text: " "},
				segment{text: "under", style: domain.Style{}.Set(domain.AttrUnderline)},
				segment{text: " "},
				segment{text: "struck", style: domain.Style{}.Set(domain.AttrStrike)}),
			goldenLine(cols,
				segment{text: "wide ", style: domain.Style{}},
				segment{text: "\u65e5\u672c", style: domain.Style{FG: domain.IndexedColor(2)}, wide: true},
				segment{text: " done.", style: domain.Style{}}),
		}
	}
	return domain.Window{
		Frame:  domain.Frame{Grid: domain.Grid{Cols: cols, Lines: lines}},
		Theme:  th,
		Chrome: c,
	}
}
