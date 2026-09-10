// Package theme reads colour schemes in Ghostty's theme file format. codeshot
// embeds two of its own and parses anyone else's, which is how a shot can look
// like the terminal it came from without Ghostty being installed.
package theme

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeshot/internal/domain"
)

//go:embed assets/*.conf
var assets embed.FS

// Source resolves a theme by name. Dirs are searched after the embedded
// themes and before the name is tried as a path.
type Source struct {
	Dirs []string
}

// Theme resolves a name to a theme, and takes care to say which of the two
// possible failures happened. "I could not find it" and "I found it and it is
// not a theme" send the user to completely different places, and collapsing
// the second into the first - as an err != nil that just keeps searching
// does - tells someone to go looking for a file that is sitting exactly where
// they put it.
func (s Source) Theme(name string) (domain.Theme, error) {
	if data, err := assets.ReadFile("assets/" + name + ".conf"); err == nil {
		return Parse(strings.NewReader(string(data)), name)
	}
	for _, dir := range s.Dirs {
		th, err := parseFile(filepath.Join(dir, name))
		if err == nil {
			return th, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return domain.Theme{}, err
		}
	}
	th, err := parseFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Theme{}, fmt.Errorf("theme %q: not embedded, not in any theme directory, and not a readable file", name)
	}
	if err != nil {
		return domain.Theme{}, err
	}
	return th, nil
}

func Default() domain.Theme {
	th, err := Source{}.Theme("codeshot-dark")
	if err != nil {
		// The default theme is embedded in the binary; if it will not parse,
		// the binary is broken and no shot it takes could be trusted.
		panic("codeshot: embedded default theme is unreadable: " + err.Error())
	}
	return th
}

func parseFile(path string) (domain.Theme, error) {
	f, err := os.Open(path)
	if err != nil {
		return domain.Theme{}, err
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Parse(f, name)
}

// Parse reads the handful of keys that describe colour. Anything else in the
// file - and Ghostty configs carry a great deal else - is ignored.
//
// Because every unrecognised key is skipped, a file has to prove it is a
// theme rather than merely fail to prove it is not: without the background
// and foreground check at the end, any file at all parsed into the zero
// Theme, whose background is fully transparent. codeshot would then render a
// shadow, three traffic lights and a title floating on nothing, store it and
// exit 0. The likeliest wrong input is a Ghostty *config*, which usually
// names a theme (theme = tokyonight) and carries no literal colours.
func Parse(r io.Reader, name string) (domain.Theme, error) {
	th := domain.Theme{Name: name}
	var haveBackground, haveForeground bool
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		var err error
		switch key {
		case "background":
			th.Background, err = ParseColor(value)
			haveBackground = err == nil
		case "foreground":
			th.Foreground, err = ParseColor(value)
			haveForeground = err == nil
		case "cursor-color":
			th.Cursor, err = ParseColor(value)
		case "palette":
			err = parsePalette(&th, value)
		}
		if err != nil {
			return domain.Theme{}, fmt.Errorf("theme %q: %s: %w", name, key, err)
		}
	}
	if err := sc.Err(); err != nil {
		return domain.Theme{}, err
	}
	switch {
	case !haveBackground && !haveForeground:
		return domain.Theme{}, fmt.Errorf("theme %q: no background or foreground colour; this does not look like a Ghostty theme file", name)
	case !haveBackground:
		return domain.Theme{}, fmt.Errorf("theme %q: no background colour", name)
	case !haveForeground:
		return domain.Theme{}, fmt.Errorf("theme %q: no foreground colour", name)
	}
	return th, nil
}

func parsePalette(th *domain.Theme, value string) error {
	idxText, colorText, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("want index=colour, got %q", value)
	}
	i, err := strconv.Atoi(strings.TrimSpace(idxText))
	if err != nil {
		return fmt.Errorf("index %q: %w", idxText, err)
	}
	if i < 0 || i > 15 {
		return fmt.Errorf("index %d is outside the 16-colour palette", i)
	}
	c, err := ParseColor(strings.TrimSpace(colorText))
	if err != nil {
		return err
	}
	th.Palette[i] = c
	return nil
}

// ParseColor reads #rrggbb, rrggbb or #rgb. It is exported because the CLI
// parses --background with exactly the same rules a theme file uses.
func ParseColor(s string) (domain.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return domain.RGBA{}, fmt.Errorf("%q is not a six-digit hex colour", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return domain.RGBA{}, fmt.Errorf("%q is not hexadecimal", s)
	}
	return domain.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xFF}, nil
}
