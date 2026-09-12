package raster

import (
	"image"
	"testing"

	"codeshot/internal/domain"
)

// titled is a window wide enough for a titlebar to hold something.
func titled(title string) domain.Window {
	w := oneGlyph(' ', domain.Style{})
	line := make([]domain.Cell, 24)
	for i := range line {
		line[i] = domain.Cell{Rune: ' ', Width: 1}
	}
	w.Frame.Grid = domain.Grid{Cols: 24, Lines: [][]domain.Cell{line}}
	w.Chrome.ShowTitle = true
	w.Chrome.Title = title
	return w
}

// titlebarColours counts what is drawn in the titlebar strip alone, above the
// grid and below the top of the window.
func titlebarColours(t *testing.T, img image.Image, height int) int {
	t.Helper()
	b := img.Bounds()
	if height <= 0 || height > b.Dy() {
		t.Fatalf("titlebar height %d does not fit in %v", height, b)
	}
	seen := map[uint64]bool{}
	for y := b.Min.Y; y < b.Min.Y+height; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[uint64(r)<<48|uint64(g)<<32|uint64(bl)<<16|uint64(a)] = true
		}
	}
	return len(seen)
}

// TestTitleDrawsAnEmojiInColour closes the gap the first emoji shot showed:
// the output had its colours and the titlebar above it, which defaults to the
// command, still had tofu.
func TestTitleDrawsAnEmojiInColour(t *testing.T) {
	r := emojiRenderer(t)

	withEmoji := titled("🎉 build")
	img, err := r.Render(withEmoji)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	plain, err := r.Render(titled("oo build"))
	if err != nil {
		t.Fatal(err)
	}

	height := withEmoji.Chrome.TitlebarHeight * withEmoji.Chrome.Scale
	emojiColours := titlebarColours(t, img, height)
	plainColours := titlebarColours(t, plain, height)
	if emojiColours <= plainColours {
		t.Errorf("titlebar with an emoji has %d colours and one without has %d; the bitmap path did not run",
			emojiColours, plainColours)
	}
}

// TestTitleIsCentredWhateverDrawsIt: the title is measured with the same
// per-rune widths it is drawn with, so an emoji or a CJK rune must not push
// it off centre. The check is that the ink is balanced about the middle.
func TestTitleIsCentredWhateverDrawsIt(t *testing.T) {
	r := emojiRenderer(t)
	img, err := r.Render(titled("🎉 build 🎉"))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	height := domain.DefaultChrome().TitlebarHeight * domain.DefaultChrome().Scale
	left, right := -1, -1
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Min.Y+height; y++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 && !isWindowBackground(img, x, y) {
				if left == -1 {
					left = x
				}
				right = x
				break
			}
		}
	}
	if left == -1 {
		t.Fatal("nothing was drawn in the titlebar")
	}
	centre := (b.Min.X + b.Max.X) / 2
	leftGap, rightGap := centre-left, right-centre
	if diff := leftGap - rightGap; diff > 8 || diff < -8 {
		t.Errorf("title ink runs from %d to %d about a centre of %d; it is not centred", left, right, centre)
	}
}

// isWindowBackground reports whether a pixel is the window's own fill rather
// than something drawn on top of it.
func isWindowBackground(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r == 0 && g == 0 && b == 0
}

func TestFitTitleShortensWithAnEllipsis(t *testing.T) {
	// One unit per rune, so the arithmetic in the test is the arithmetic in
	// the assertion.
	measure := func(s string) int { return len([]rune(s)) }

	if got, ok := fitTitle(measure, "npm test", 20); !ok || got != "npm test" {
		t.Errorf("fitTitle = %q,%v; a title that fits is left alone", got, ok)
	}
	got, ok := fitTitle(measure, "npm run build --workspaces", 10)
	if !ok {
		t.Fatal("fitTitle gave up on a title that could be shortened")
	}
	if len([]rune(got)) > 10 {
		t.Errorf("fitTitle = %q, longer than the 10 it was given", got)
	}
	if got[len(got)-len("…"):] != "…" {
		t.Errorf("fitTitle = %q, want it to end in an ellipsis", got)
	}
	if _, ok := fitTitle(measure, "anything", 0); ok {
		t.Error("fitTitle fitted a title into no space at all")
	}
}
