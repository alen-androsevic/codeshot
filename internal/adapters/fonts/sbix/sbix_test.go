package sbix

import (
	"errors"
	"image"
	"os"
	"testing"

	"golang.org/x/image/font/sfnt"
)

const applePath = "/System/Library/Fonts/Apple Color Emoji.ttc"

// appleEmoji opens the system emoji font, and the sfnt parse of it that turns
// a rune into the glyph id this package works in.
func appleEmoji(t *testing.T) (*Font, func(r rune) int) {
	t.Helper()
	if _, err := os.Stat(applePath); err != nil {
		t.Skip("Apple Color Emoji is not installed here")
	}
	font, err := Open(applePath, 0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { font.Close() })

	f, err := os.Open(applePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	collection, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := collection.Font(0)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	return font, func(r rune) int {
		gid, err := parsed.GlyphIndex(&buf, r)
		if err != nil {
			t.Fatalf("GlyphIndex(%q): %v", r, err)
		}
		return int(gid)
	}
}

func TestSizesAreTheStrikesTheFontCarries(t *testing.T) {
	font, _ := appleEmoji(t)
	sizes := font.Sizes()
	if len(sizes) == 0 {
		t.Fatal("no strikes found in a font that is nothing but strikes")
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] <= sizes[i-1] {
			t.Errorf("sizes %v are not increasing", sizes)
			break
		}
	}
}

// TestGlyphDecodesAnEmoji is the whole point: x/image draws nothing at all
// for this rune, and here is its picture.
func TestGlyphDecodesAnEmoji(t *testing.T) {
	font, gidOf := appleEmoji(t)
	img, err := font.Glyph(gidOf('🎉'), 64)
	if err != nil {
		t.Fatalf("Glyph: %v", err)
	}
	if img == nil {
		t.Fatal("no bitmap for 🎉")
	}
	b := img.Bounds()
	if b.Dx() < 16 || b.Dy() < 16 {
		t.Errorf("bounds = %v, want a real bitmap", b)
	}
	if b.Dx() != b.Dy() {
		t.Errorf("bounds = %v, want a square emoji bitmap", b)
	}
	if !hasColour(img) {
		t.Error("the bitmap is entirely one colour; a party popper is not")
	}
}

// hasColour reports whether more than one colour appears, which is what
// separates a decoded emoji from an empty or broken one.
func hasColour(img image.Image) bool {
	b := img.Bounds()
	first := img.At(b.Min.X, b.Min.Y)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != first {
				return true
			}
		}
	}
	return false
}

// TestGlyphPrefersTheSmallestStrikeThatIsBigEnough: scaling a bitmap down
// looks far better than scaling one up, so the search goes upwards first.
func TestGlyphPrefersTheSmallestStrikeThatIsBigEnough(t *testing.T) {
	font, gidOf := appleEmoji(t)
	gid := gidOf('🎉')
	sizes := font.Sizes()

	small, err := font.Glyph(gid, sizes[0])
	if err != nil || small == nil {
		t.Fatalf("Glyph at the smallest strike: %v", err)
	}
	large, err := font.Glyph(gid, sizes[len(sizes)-1])
	if err != nil || large == nil {
		t.Fatalf("Glyph at the largest strike: %v", err)
	}
	if large.Bounds().Dx() <= small.Bounds().Dx() {
		t.Errorf("asking for %d gave %v and asking for %d gave %v; the larger request must not come back smaller",
			sizes[0], small.Bounds(), sizes[len(sizes)-1], large.Bounds())
	}

	// Beyond the largest strike there is nothing bigger to find, and the
	// biggest bitmap is the best answer rather than an error.
	huge, err := font.Glyph(gid, 4096)
	if err != nil {
		t.Fatalf("Glyph far above every strike: %v", err)
	}
	if huge == nil {
		t.Error("no bitmap at all above the largest strike")
	}
}

// TestGlyphOfNothingIsNotAFailure: most glyph ids in most fonts have no
// bitmap, and a cell that needs drawing is not an error condition.
func TestGlyphOfNothingIsNotAFailure(t *testing.T) {
	font, _ := appleEmoji(t)
	for _, gid := range []int{-1, 1 << 20} {
		img, err := font.Glyph(gid, 64)
		if err != nil {
			t.Errorf("Glyph(%d) = %v, want a quiet nil", gid, err)
		}
		if img != nil {
			t.Errorf("Glyph(%d) invented a bitmap", gid)
		}
	}
}

// TestAFontWithoutStrikesSaysSo covers the ordinary case: almost every font
// has no sbix table, and that is an answer rather than a failure.
func TestAFontWithoutStrikesSaysSo(t *testing.T) {
	const outlines = "../assets/JetBrainsMonoNL-Regular.ttf"
	if _, err := os.Stat(outlines); err != nil {
		t.Skipf("embedded asset not readable from here: %v", err)
	}
	_, err := Open(outlines, 0)
	if !errors.Is(err, ErrNoTable) {
		t.Errorf("Open = %v, want ErrNoTable", err)
	}
}

func TestOpenRefusesAFontThatIsNotThere(t *testing.T) {
	if _, err := Open("/no/such/font.ttc", 0); err == nil {
		t.Error("Open invented a font")
	}
}

func TestOpenChecksTheCollectionIndex(t *testing.T) {
	if _, err := Open(applePath, 99); err == nil {
		t.Error("Open accepted font 99 of a collection that has two")
	}
}
