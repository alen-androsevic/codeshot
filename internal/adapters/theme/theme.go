// Package theme reads colour schemes in Ghostty's theme file format. codeshot
// embeds two of its own and parses anyone else's, which is how a shot can look
// like the terminal it came from without Ghostty being installed.
package theme

import (
	"bufio"
	"embed"
	"fmt"
	"io"
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

func (s Source) Theme(name string) (domain.Theme, error) {
	if data, err := assets.ReadFile("assets/" + name + ".conf"); err == nil {
		return Parse(strings.NewReader(string(data)), name)
	}
	for _, dir := range s.Dirs {
		if th, err := parseFile(filepath.Join(dir, name)); err == nil {
			return th, nil
		}
	}
	th, err := parseFile(name)
	if err != nil {
		return domain.Theme{}, fmt.Errorf("theme %q: not embedded, not in any theme directory, and not a readable file", name)
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
func Parse(r io.Reader, name string) (domain.Theme, error) {
	th := domain.Theme{Name: name}
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
		case "foreground":
			th.Foreground, err = ParseColor(value)
		case "cursor-color":
			th.Cursor, err = ParseColor(value)
		case "palette":
			err = parsePalette(&th, value)
		}
		if err != nil {
			return domain.Theme{}, fmt.Errorf("theme %q: %s: %w", name, key, err)
		}
	}
	return th, sc.Err()
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
