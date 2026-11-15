// Package sbix reads the colour bitmap glyphs Apple's emoji font is made of.
//
// golang.org/x/image draws outlines, and Apple Color Emoji has none: its
// glyphs are PNG images in an `sbix` table, one set per pixel size. x/image
// parses the font happily and then draws nothing at all for 🎉, which is why
// emoji have been a documented tofu limitation since phase 1 (design §7).
//
// This package is deliberately small and knows nothing about codeshot: give
// it a font file and a glyph id, and it hands back an image.
package sbix

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"sort"
)

// ErrNoTable is returned for a font with no sbix table, which is almost every
// font. It is an ordinary answer, not a failure.
var ErrNoTable = errors.New("font has no sbix table")

// Font is one font's colour bitmaps. It keeps the file open and reads glyph
// data on demand: Apple Color Emoji is 192MB, and a picture usually wants a
// handful of glyphs from it.
type Font struct {
	f       io.ReaderAt
	closer  io.Closer
	strikes []strike
}

// strike is one pixel size's worth of glyphs.
type strike struct {
	ppem, ppi uint16
	// offsets are absolute file offsets, one per glyph plus a final end
	// marker; a glyph whose two offsets are equal has no bitmap at this size.
	offsets []uint32
}

// Open reads the sbix table of font number index in the file at path. A .ttc
// holds several fonts; Apple Color Emoji is one.
func Open(path string, index int) (*Font, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	font, err := New(f, index)
	if err != nil {
		f.Close()
		return nil, err
	}
	font.closer = f
	return font, nil
}

// New is Open for a reader that is already open.
func New(r io.ReaderAt, index int) (*Font, error) {
	tableOffset, tableLength, err := findTable(r, index, "sbix")
	if err != nil {
		return nil, err
	}
	strikes, err := readStrikes(r, tableOffset, tableLength)
	if err != nil {
		return nil, err
	}
	if len(strikes) == 0 {
		return nil, ErrNoTable
	}
	return &Font{f: r, strikes: strikes}, nil
}

func (f *Font) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

// Sizes are the pixel sizes this font has bitmaps for, smallest first.
func (f *Font) Sizes() []int {
	sizes := make([]int, len(f.strikes))
	for i, s := range f.strikes {
		sizes[i] = int(s.ppem)
	}
	return sizes
}

// Glyph returns the bitmap for a glyph at the smallest size that is at least
// ppem, or the largest there is when every strike is smaller. Scaling down a
// larger bitmap looks far better than scaling one up, which is why the search
// goes upwards first.
//
// A glyph with no bitmap - most of them, in a font that also has outlines -
// comes back nil with no error.
func (f *Font) Glyph(gid, ppem int) (image.Image, error) {
	if gid < 0 {
		return nil, nil
	}
	for _, s := range f.pick(ppem) {
		img, err := f.glyphFrom(s, gid, 0)
		if err != nil {
			return nil, err
		}
		if img != nil {
			return img, nil
		}
	}
	return nil, nil
}

// pick orders the strikes to try: the best fit first, then the rest, so that
// a glyph missing from one size is still found in another.
func (f *Font) pick(ppem int) []strike {
	ordered := make([]strike, len(f.strikes))
	copy(ordered, f.strikes)
	sort.SliceStable(ordered, func(i, j int) bool {
		bigEnoughI, bigEnoughJ := int(ordered[i].ppem) >= ppem, int(ordered[j].ppem) >= ppem
		if bigEnoughI != bigEnoughJ {
			return bigEnoughI
		}
		if bigEnoughI {
			return ordered[i].ppem < ordered[j].ppem
		}
		return ordered[i].ppem > ordered[j].ppem
	})
	return ordered
}

// maxDupeHops stops a `dupe` glyph that points at itself, or at another dupe
// in a ring, from looping.
const maxDupeHops = 4

func (f *Font) glyphFrom(s strike, gid, hops int) (image.Image, error) {
	if hops > maxDupeHops || gid < 0 || gid+1 >= len(s.offsets) {
		return nil, nil
	}
	start, end := s.offsets[gid], s.offsets[gid+1]
	if end <= start {
		return nil, nil
	}
	// Every glyph record begins with two origin offsets and a four-byte tag
	// naming the image format.
	const header = 8
	if end-start < header {
		return nil, nil
	}
	buf := make([]byte, end-start)
	if _, err := f.f.ReadAt(buf, int64(start)); err != nil {
		return nil, fmt.Errorf("reading glyph %d: %w", gid, err)
	}
	switch tag := string(buf[4:8]); tag {
	case "png ":
		img, err := png.Decode(newReader(buf[header:]))
		if err != nil {
			return nil, fmt.Errorf("glyph %d: %w", gid, err)
		}
		return img, nil
	case "dupe":
		// The glyph is another glyph's bitmap, named by id.
		if len(buf) < header+2 {
			return nil, nil
		}
		return f.glyphFrom(s, int(binary.BigEndian.Uint16(buf[header:])), hops+1)
	default:
		// jpg, tiff and the rest: real, rare, and not worth carrying a
		// decoder for until something uses them.
		return nil, nil
	}
}

func readStrikes(r io.ReaderAt, offset, length uint32) ([]strike, error) {
	head := make([]byte, 8)
	if _, err := r.ReadAt(head, int64(offset)); err != nil {
		return nil, err
	}
	count := binary.BigEndian.Uint32(head[4:])
	if count == 0 || count > 1<<12 {
		return nil, ErrNoTable
	}
	offsets := make([]byte, 4*count)
	if _, err := r.ReadAt(offsets, int64(offset)+8); err != nil {
		return nil, err
	}

	var strikes []strike
	for i := uint32(0); i < count; i++ {
		strikeOffset := offset + binary.BigEndian.Uint32(offsets[4*i:])
		if strikeOffset < offset || strikeOffset >= offset+length {
			continue
		}
		s, err := readStrike(r, strikeOffset, offset+length)
		if err != nil {
			continue
		}
		strikes = append(strikes, s)
	}
	sort.Slice(strikes, func(i, j int) bool { return strikes[i].ppem < strikes[j].ppem })
	return strikes, nil
}

func readStrike(r io.ReaderAt, offset, tableEnd uint32) (strike, error) {
	head := make([]byte, 4)
	if _, err := r.ReadAt(head, int64(offset)); err != nil {
		return strike{}, err
	}
	s := strike{
		ppem: binary.BigEndian.Uint16(head[0:]),
		ppi:  binary.BigEndian.Uint16(head[2:]),
	}
	if s.ppem == 0 {
		return strike{}, errors.New("strike has no size")
	}
	// The glyph offsets run to the end of the strike, and the strike runs to
	// wherever the next one starts; reading to the end of the table and
	// stopping at the first offset that cannot be right is simpler than
	// tracking neighbours, and a malformed table is not worth trusting
	// anyway.
	raw := make([]byte, tableEnd-offset-4)
	n, err := r.ReadAt(raw, int64(offset)+4)
	if err != nil && n == 0 {
		return strike{}, err
	}
	raw = raw[:n-n%4]
	offsets := make([]uint32, 0, len(raw)/4)
	for i := 0; i+4 <= len(raw); i += 4 {
		v := binary.BigEndian.Uint32(raw[i:])
		absolute := offset + v
		if absolute > tableEnd {
			break
		}
		offsets = append(offsets, absolute)
		// The offsets are non-decreasing; the first one that is not ends the
		// list, which is where the glyph data itself begins.
		if len(offsets) > 1 && offsets[len(offsets)-1] < offsets[len(offsets)-2] {
			offsets = offsets[:len(offsets)-1]
			break
		}
	}
	if len(offsets) < 2 {
		return strike{}, errors.New("strike has no glyphs")
	}
	s.offsets = offsets
	return s, nil
}

// findTable walks the font's table directory for one tag. sfnt parses fonts
// but exposes no way to reach a table it does not itself understand, and
// sbix is one of those.
func findTable(r io.ReaderAt, index int, want string) (offset, length uint32, err error) {
	base, err := fontOffset(r, index)
	if err != nil {
		return 0, 0, err
	}
	head := make([]byte, 12)
	if _, err := r.ReadAt(head, int64(base)); err != nil {
		return 0, 0, err
	}
	numTables := int(binary.BigEndian.Uint16(head[4:]))
	if numTables == 0 || numTables > 1<<12 {
		return 0, 0, ErrNoTable
	}
	entries := make([]byte, 16*numTables)
	if _, err := r.ReadAt(entries, int64(base)+12); err != nil {
		return 0, 0, err
	}
	for i := 0; i < numTables; i++ {
		e := entries[16*i:]
		if string(e[0:4]) != want {
			continue
		}
		return binary.BigEndian.Uint32(e[8:]), binary.BigEndian.Uint32(e[12:]), nil
	}
	return 0, 0, ErrNoTable
}

// fontOffset is where font number index's table directory begins: 0 for a
// plain font, and out of the collection header for a .ttc.
func fontOffset(r io.ReaderAt, index int) (uint32, error) {
	tag := make([]byte, 12)
	if _, err := r.ReadAt(tag, 0); err != nil {
		return 0, err
	}
	if string(tag[0:4]) != "ttcf" {
		if index != 0 {
			return 0, fmt.Errorf("font %d asked for in a file holding one", index)
		}
		return 0, nil
	}
	count := int(binary.BigEndian.Uint32(tag[8:]))
	if index < 0 || index >= count {
		return 0, fmt.Errorf("font %d is not in a collection of %d", index, count)
	}
	entry := make([]byte, 4)
	if _, err := r.ReadAt(entry, int64(12+4*index)); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(entry), nil
}

// newReader is bytes.NewReader, without importing bytes for one call.
func newReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
