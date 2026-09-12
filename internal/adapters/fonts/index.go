package fonts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/image/font/sfnt"
)

// FontFile is one cut of one family: a file, and which font inside it, since
// a .ttc holds several. Menlo.ttc carries all four of Menlo's cuts.
type FontFile struct {
	Path  string `json:"path"`
	Index int    `json:"index"`
}

func (f FontFile) Empty() bool { return f.Path == "" }

// Family is the four cuts codeshot draws with, indexed by the same constants
// the embedded set uses. A family missing a cut leaves that slot empty, which
// is what the fallback to synthetic bold and sheared italic is for.
type Family struct {
	Name  string      `json:"name"`
	Files [4]FontFile `json:"files"`
}

// Index is every family found on this machine. It is a cache of names and
// paths - never of terminal output - which is the only file codeshot writes
// besides the picture itself (design §2, constraint 5).
type Index struct {
	// Mtimes is every directory walked, and when it last changed. A font
	// installed or removed changes its directory's mtime, which is how the
	// cache knows it has gone stale without reopening several hundred files.
	Mtimes   map[string]int64  `json:"mtimes"`
	Families map[string]Family `json:"families"`
}

// SystemFontDirs is the OS's font directories that exist here.
func SystemFontDirs() []string {
	var found []string
	for _, dir := range systemFontDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			found = append(found, dir)
		}
	}
	return found
}

// FallbackFamilies is the platform's fallback chain, in order: the families
// consulted for a rune the chosen font has no glyph for. Exported so that
// `codeshot doctor` can say which of them are actually installed.
func FallbackFamilies() []string { return fallbackFamilies() }

// CachePath is where the index is kept between runs.
func CachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "codeshot", "fonts.json")
}

// Lookup finds a family by name, ignoring case and surrounding space, so
// that --font "jetbrains mono nl" and a Ghostty config's `font-family =
// JetBrains Mono NL` reach the same place.
func (i Index) Lookup(name string) (Family, bool) {
	f, ok := i.Families[key(name)]
	return f, ok
}

// Names is every family found, sorted, for `codeshot doctor` to count and
// for an unknown --font to be answered with something better than "no".
func (i Index) Names() []string {
	names := make([]string, 0, len(i.Families))
	for _, f := range i.Families {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// LoadIndex returns the index for dirs, reusing the cache at path when it is
// still good and rescanning when it is not. A cache that cannot be read or
// written is not an error: it costs a scan, and a scan is what the cache was
// avoiding, not something it made possible.
func LoadIndex(path string, dirs []string) Index {
	if cached, ok := readCache(path); ok && !cached.stale(dirs) {
		return cached
	}
	idx := ScanDirs(dirs)
	idx.write(path)
	return idx
}

// stale reports whether anything the index was built from has changed. A
// directory that has appeared, gone, or been written to since the scan means
// the fonts on this machine are not the fonts in here.
func (i Index) stale(dirs []string) bool {
	seen := map[string]bool{}
	for _, dir := range dirs {
		if _, ok := i.Mtimes[dir]; !ok {
			return true
		}
		seen[dir] = true
	}
	for dir, when := range i.Mtimes {
		info, err := os.Stat(dir)
		if err != nil || info.ModTime().UnixNano() != when {
			return true
		}
		// A directory the index walked into is fine; one it walked from that
		// is no longer asked for means the caller changed its mind.
		if !seen[dir] && !within(dir, dirs) {
			return true
		}
	}
	return false
}

func within(dir string, roots []string) bool {
	for _, root := range roots {
		if strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ScanDirs walks dirs and reads the family name out of every font it can
// open. Nothing here fails: a font that will not parse, a directory that
// cannot be read and a file that is not a font at all are all simply not in
// the index, because a broken font somewhere on the machine is no reason to
// refuse to take a picture.
func ScanDirs(dirs []string) Index {
	idx := Index{Mtimes: map[string]int64{}, Families: map[string]Family{}}
	// definitive records whether a slot was filled by a cut that said plainly
	// what it was, so that a "Regular" replaces a "Light" but not the other
	// way round.
	definitive := map[string][4]bool{}

	for _, root := range dirs {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if info, err := d.Info(); err == nil {
					idx.Mtimes[path] = info.ModTime().UnixNano()
				}
				return nil
			}
			if !isFontFile(path) {
				return nil
			}
			for _, face := range readFaces(path) {
				idx.add(face, definitive)
			}
			return nil
		})
	}
	return idx
}

// face is one cut as the scanner found it, before it is filed under a family.
type face struct {
	family     string
	file       FontFile
	variant    int
	definitive bool
}

func (i Index) add(f face, definitive map[string][4]bool) {
	k := key(f.family)
	fam, ok := i.Families[k]
	if !ok {
		fam = Family{Name: f.family}
	}
	def := definitive[k]
	// First one in wins, unless this one is a cut that named itself and the
	// one already there was only a guess.
	if fam.Files[f.variant].Empty() || (f.definitive && !def[f.variant]) {
		fam.Files[f.variant] = f.file
		def[f.variant] = f.definitive
	}
	definitive[k] = def
	i.Families[k] = fam
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// readFaces names every font in a file. It reads through an *os.File rather
// than loading the bytes: Apple Color Emoji alone is 192MB, and an index
// that read every font whole would spend a second and a gigabyte to learn a
// few hundred strings.
func readFaces(path string) []face {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	collection, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		return nil
	}
	var faces []face
	var buf sfnt.Buffer
	for i := 0; i < collection.NumFonts(); i++ {
		font, err := collection.Font(i)
		if err != nil {
			continue
		}
		family, err := font.Name(&buf, sfnt.NameIDFamily)
		if err != nil || family == "" {
			continue
		}
		sub, err := font.Name(&buf, sfnt.NameIDSubfamily)
		if err != nil {
			sub = ""
		}
		variant, definitive := classify(sub)
		faces = append(faces, face{
			family:     family,
			file:       FontFile{Path: path, Index: i},
			variant:    variant,
			definitive: definitive,
		})
	}
	return faces
}

// classify reads a cut's subfamily - "Regular", "Bold Italic", "Light",
// "Condensed Black Oblique" - and says which of the four slots it belongs in.
// definitive distinguishes a cut that named itself from one merely assumed:
// .SF NS Mono ships only a "Light", which is the best regular it has, but a
// real "Regular" alongside it should win.
func classify(subfamily string) (variant int, definitive bool) {
	s := strings.ToLower(strings.TrimSpace(subfamily))
	italic := strings.Contains(s, "italic") || strings.Contains(s, "oblique")
	bold := strings.Contains(s, "bold")
	switch {
	case bold && italic:
		return variantBoldItalic, true
	case bold:
		return variantBold, true
	case italic:
		return variantItalic, true
	}
	switch s {
	case "regular", "normal", "book", "roman", "":
		return variantRegular, true
	}
	return variantRegular, false
}

func readCache(path string) (Index, bool) {
	if path == "" {
		return Index{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Index{}, false
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil || idx.Families == nil || idx.Mtimes == nil {
		return Index{}, false
	}
	return idx, true
}

func (i Index) write(path string) {
	if path == "" {
		return
	}
	data, err := json.Marshal(i)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	os.WriteFile(path, data, 0o644)
}
