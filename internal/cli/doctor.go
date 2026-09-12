package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"codeshot/internal/adapters/config"
	"codeshot/internal/adapters/fonts"
	"codeshot/internal/adapters/theme"
	"codeshot/internal/adapters/tty"
)

// doctor reports what codeshot found on this machine. Everything it prints is
// something that changes what a picture looks like or where it goes, so that
// "why is this not my theme" and "why is that glyph a box" can be answered by
// reading rather than guessing.
func doctor(args []string, stdout, stderr io.Writer) int {
	configPath := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			i++
			configPath = args[i]
		case strings.HasPrefix(args[i], "--config="):
			configPath = strings.TrimPrefix(args[i], "--config=")
		case args[i] == "-h" || args[i] == "--help":
			fmt.Fprint(stdout, usage)
			return 0
		}
	}

	out := &section{w: stdout}
	fmt.Fprintf(stdout, "codeshot %s\n", Version)

	own, configErr := config.Load(configPath)
	out.head("Config")
	switch {
	case configErr != nil:
		out.line("codeshot", "%v", configErr)
	case own.Path == "":
		out.line("codeshot", "none; put one at %s", firstPath(config.Paths()))
	default:
		out.line("codeshot", "%s (%s)", tildify(own.Path), strings.Join(own.Order, ", "))
	}
	if gh, found := loadGhostty(); found {
		out.line("ghostty", "%s", tildify(gh.Path))
		out.line("", "theme %s, font-family %s, font-size %s, padding %d/%d",
			orNone(gh.Theme), orNone(gh.FontFamily), orZero(gh.FontSize), gh.PaddingX, gh.PaddingY)
	} else {
		out.line("ghostty", "none; codeshot uses its own defaults")
	}

	src := themeSource()
	out.head("Themes")
	// Asked for, not spelled out: a third embedded theme would make a
	// hardcoded pair here lie, which is the fault `codeshot themes` had.
	out.line("embedded", "%s", strings.Join(theme.Source{}.Installed(), ", "))
	if dirs := src.Dirs; len(dirs) == 0 {
		out.line("directories", "none found; --theme takes a path to a theme file")
	} else {
		for _, dir := range dirs {
			out.line("directories", "%s", dir)
		}
	}
	out.line("resolvable", "%d names", len(src.Installed()))

	index := loadFontIndex()
	out.head("Fonts")
	out.line("drawing with", "%s", drawingWith(own, index))
	out.line("index", "%s (%d families)", tildify(fonts.CachePath()), len(index.Families))
	for _, dir := range fonts.SystemFontDirs() {
		out.line("scanned", "%s", tildify(dir))
	}
	out.line("fallbacks", "%s", strings.Join(installedFallbacks(index), ", "))

	out.head("Output")
	gallery := defaultGallery()
	out.line("gallery", "%s (%s)", tildify(gallery), writability(gallery))
	if err := clipboardAvailable(); err != nil {
		out.line("clipboard", "%v", err)
	} else {
		out.line("clipboard", "available for --clip")
	}

	out.head("Terminal")
	cols, rows := tty.Size(os.Stderr)
	if tty.IsTerminal(os.Stderr) {
		out.line("size", "%dx%d, from stderr", cols, rows)
	} else {
		out.line("size", "%dx%d; stderr is not a terminal here, so this is the fallback", cols, rows)
	}
	return 0
}

// section keeps doctor's output in two aligned columns.
type section struct{ w io.Writer }

func (s *section) head(name string) { fmt.Fprintf(s.w, "\n%s\n", name) }

func (s *section) line(label, format string, args ...any) {
	fmt.Fprintf(s.w, "  %-14s %s\n", label, fmt.Sprintf(format, args...))
}

// drawingWith answers the question doctor exists for: which font a shot taken
// right now would actually use.
func drawingWith(own config.Config, index fonts.Index) string {
	name, from := "", ""
	if v, ok := own.String("font"); ok {
		name, from = v, "codeshot config"
	} else if gh, found := loadGhostty(); found && gh.FontFamily != "" {
		name, from = gh.FontFamily, "ghostty config"
	}
	if name == "" {
		return "JetBrains Mono NL (embedded)"
	}
	if fam, ok := index.Lookup(name); ok {
		return fmt.Sprintf("%s (%s)", fam.Name, from)
	}
	return fmt.Sprintf("%s from %s is not installed; using JetBrains Mono NL (embedded)", name, from)
}

func installedFallbacks(index fonts.Index) []string {
	var found []string
	for _, name := range fonts.FallbackFamilies() {
		if fam, ok := index.Lookup(name); ok && !fam.Files[0].Empty() {
			found = append(found, fam.Name)
		}
	}
	if len(found) == 0 {
		return []string{"none installed; a rune the font lacks will be a box"}
	}
	return found
}

func writability(dir string) string {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("cannot be created: %v", err)
	}
	probe := filepath.Join(dir, ".codeshot-write-test")
	f, err := os.Create(probe)
	if err != nil {
		return fmt.Sprintf("not writable: %v", err)
	}
	f.Close()
	os.Remove(probe)
	return "writable"
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return "~/.config/codeshot/config"
	}
	return paths[0]
}

func orNone(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

func orZero(f float64) string {
	if f == 0 {
		return "(unset)"
	}
	return fmt.Sprintf("%g", f)
}

// tildify contracts the home directory, the way the stored line does.
func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
