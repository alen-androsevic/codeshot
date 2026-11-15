// Package ghostty reads the settings codeshot can honour from a Ghostty
// config file. It is what makes a shot look like the terminal it came from -
// your theme, your size, your padding - while keeping Ghostty entirely
// optional: the file's absence is never an error, and nothing here is
// required for codeshot to run.
//
// Only four keys are read. Ghostty configs carry a great deal else, and
// everything unrecognised is skipped, the way the theme parser skips it.
package ghostty

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is what codeshot found worth honouring. A zero field means the file
// said nothing about it, which is why FontSize is a float64 and not a
// sentinel: 0 is not a size anyone asked for.
type Config struct {
	// Theme is a single theme name, already reduced from Ghostty's
	// light/dark pair if it was one.
	Theme      string
	FontFamily string
	FontSize   float64
	PaddingX   int
	PaddingY   int
	// Path is the file this came from, for `codeshot doctor` to name.
	Path string
}

// Paths are where a Ghostty config might be, in the order Ghostty itself
// looks. The XDG location comes first on every platform; macOS also has an
// Application Support directory, which is where the app writes one if you
// never made your own.
func Paths() []string {
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "ghostty", "config"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return paths
	}
	if os.Getenv("XDG_CONFIG_HOME") == "" {
		paths = append(paths, filepath.Join(home, ".config", "ghostty", "config"))
	}
	return append(paths, filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config"))
}

// Load reads the first config file that exists. It reports whether it found
// one; not finding one is the ordinary case on a machine without Ghostty and
// is not a failure.
func Load() (Config, bool) {
	for _, path := range Paths() {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		cfg := Parse(f)
		f.Close()
		cfg.Path = path
		return cfg, true
	}
	return Config{}, false
}

// Parse reads the four keys codeshot understands. A value it cannot make
// sense of is left alone rather than reported: this file is not codeshot's,
// and refusing to run because of a key codeshot happens to read would be a
// poor trade for a setting nobody asked it to honour.
func Parse(r io.Reader) Config {
	var cfg Config
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
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "theme":
			cfg.Theme = themeName(value)
		case "font-family":
			cfg.FontFamily = unquote(value)
		case "font-size":
			if size, err := strconv.ParseFloat(value, 64); err == nil && size > 0 {
				cfg.FontSize = size
			}
		case "window-padding-x":
			cfg.PaddingX = padding(value)
		case "window-padding-y":
			cfg.PaddingY = padding(value)
		}
	}
	return cfg
}

// themeName reduces Ghostty's theme setting to one name. It has two forms: a
// bare name, and a pair that follows the system appearance,
// `dark:Catppuccin Mocha,light:Catppuccin Latte`. codeshot takes the dark
// one. Its own default is dark, a shot is usually pasted somewhere dark, and
// - the deciding reason - picking by the machine's current appearance would
// make the same command produce different pictures at different times of
// day, which is not a property a documentation tool should have.
func themeName(value string) string {
	if !strings.Contains(value, ":") {
		return unquote(value)
	}
	var light string
	for _, part := range strings.Split(value, ",") {
		mode, name, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "dark":
			return unquote(strings.TrimSpace(name))
		case "light":
			light = unquote(strings.TrimSpace(name))
		}
	}
	// A pair with only a light side in it still says more than nothing.
	return light
}

// padding reads Ghostty's window-padding-x, which is either one number or a
// left,right pair. codeshot draws one padding on each side, so a pair is
// reduced to its larger half: padding is there to keep text off the edge,
// and the smaller number is the one that fails at that.
func padding(value string) int {
	best := 0
	for _, part := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			continue
		}
		if n > best {
			best = n
		}
	}
	return best
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
