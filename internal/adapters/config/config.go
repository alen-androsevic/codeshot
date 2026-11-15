// Package config reads codeshot's own defaults file. It is the middle of
// design §4's precedence: flags beat it, it beats Ghostty's config, and
// Ghostty's beats the built-in defaults.
//
// The format is the same flat `key = value` Ghostty uses for its config and
// its themes, rather than the TOML design §4 first named. codeshot already
// parses that shape twice, every setting here is a flag name with its dashes
// intact, and there are no sections to put anything in - so TOML would have
// bought a dependency and a second format to learn, for nothing.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is what the file said, by key. Values are kept as text and
// converted on demand, so that this package needs to know nothing about what
// any particular setting means.
type Config struct {
	Values map[string]string
	// Order is the keys as they appeared, so a report about them reads the
	// way the file does.
	Order []string
	Path  string
}

// Paths are where codeshot looks for its config, in order.
func Paths() []string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return []string{filepath.Join(xdg, "codeshot", "config")}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".config", "codeshot", "config")}
}

// Load reads the config. An explicit path - from --config - that is not there
// is an error, because it was asked for by name; the default one's absence is
// the ordinary case and is not.
func Load(explicit string) (Config, error) {
	if explicit != "" {
		return read(explicit)
	}
	for _, path := range Paths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		return read(path)
	}
	return Config{Values: map[string]string{}}, nil
}

func read(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{Values: map[string]string{}}, err
	}
	defer f.Close()
	cfg := Parse(f)
	cfg.Path = path
	return cfg, nil
}

// Parse reads `key = value` lines, ignoring blanks and # comments. Keys are
// flag names without their leading dashes: `theme = tokyonight`,
// `font-size = 15`, `no-shadow = true`.
func Parse(r io.Reader) Config {
	cfg := Config{Values: map[string]string{}}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// A line with no = is a bare key, which means true the way a bare
		// flag on the command line does: `no-shadow` and `no-shadow = true`
		// say the same thing.
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimPrefix(strings.TrimSpace(key), "--")
		if key == "" {
			continue
		}
		if _, seen := cfg.Values[key]; !seen {
			cfg.Order = append(cfg.Order, key)
		}
		cfg.Values[key] = unquote(strings.TrimSpace(value))
	}
	return cfg
}

func (c Config) String(key string) (string, bool) {
	v, ok := c.Values[key]
	return v, ok && v != ""
}

func (c Config) Float(key string) (float64, bool) {
	v, ok := c.String(key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

func (c Config) Int(key string) (int, bool) {
	v, ok := c.String(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// Bool accepts what a person would write: true, yes and on, and their
// opposites. A bare key with no value means true, the way a flag does.
func (c Config) Bool(key string) (bool, bool) {
	v, ok := c.Values[key]
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on", "1", "":
		return true, true
	case "false", "no", "off", "0":
		return false, true
	}
	return false, false
}

// Unknown lists keys that are not in known, so the CLI can say so rather than
// silently ignoring a typo in a file the user wrote for codeshot itself.
// Ghostty's config gets no such treatment: that file is not codeshot's, and
// most of what is in it is none of its business.
func (c Config) Unknown(known map[string]bool) []string {
	var unknown []string
	for _, key := range c.Order {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	return unknown
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// Example is what `codeshot doctor` points at when there is no config yet.
func Example() string {
	return fmt.Sprintf(`# %s
# Every key is a flag name without its dashes.

# theme = Catppuccin Mocha
# font = JetBrains Mono
# font-size = 13
# padding = 14
# scale = 2
# no-shadow = true
# controls = macos
# gallery = ~/Codeshots
`, firstOr(Paths(), "~/.config/codeshot/config"))
}

func firstOr(paths []string, fallback string) string {
	if len(paths) == 0 {
		return fallback
	}
	return paths[0]
}
