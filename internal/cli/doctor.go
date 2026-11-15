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
	"codeshot/internal/home"
)

// findings is everything doctor looks at, gathered once and then written as
// text for a person or, with --json, for a script or an agent. The JSON names
// are the shape a caller can rely on; the text may change.
type findings struct {
	Version string `json:"version"`
	Config  struct {
		Codeshot struct {
			Path  string   `json:"path"`
			Keys  []string `json:"keys"`
			Error string   `json:"error,omitempty"`
			// Suggested is where a config would be read from, when there is none.
			Suggested string `json:"suggested,omitempty"`
		} `json:"codeshot"`
		Ghostty *ghosttyFindings `json:"ghostty"`
	} `json:"config"`
	Themes struct {
		Embedded    []string `json:"embedded"`
		Directories []string `json:"directories"`
		Resolvable  int      `json:"resolvable"`
	} `json:"themes"`
	Fonts struct {
		DrawingWith string   `json:"drawing_with"`
		Index       string   `json:"index"`
		Families    int      `json:"families"`
		Scanned     []string `json:"scanned"`
		Fallbacks   []string `json:"fallbacks"`
		Symbols     []string `json:"symbols"`
	} `json:"fonts"`
	Output struct {
		Gallery          string `json:"gallery"`
		GalleryWritable  bool   `json:"gallery_writable"`
		GalleryError     string `json:"gallery_error,omitempty"`
		Clipboard        bool   `json:"clipboard"`
		ClipboardProblem string `json:"clipboard_error,omitempty"`
	} `json:"output"`
	Terminal struct {
		Cols     int  `json:"cols"`
		Rows     int  `json:"rows"`
		Terminal bool `json:"stderr_is_terminal"`
	} `json:"terminal"`
}

type ghosttyFindings struct {
	Path       string  `json:"path"`
	Theme      string  `json:"theme"`
	FontFamily string  `json:"font_family"`
	FontSize   float64 `json:"font_size"`
	PaddingX   int     `json:"padding_x"`
	PaddingY   int     `json:"padding_y"`
}

// doctor reports what codeshot found on this machine. Everything it prints is
// something that changes what a shot looks like or where it goes, so that
// "why is this not my theme" and "why is that glyph a box" can be answered by
// reading rather than guessing.
func doctor(args []string, stdout, stderr io.Writer) int {
	configPath, asJSON := "", false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			i++
			configPath = args[i]
		case strings.HasPrefix(args[i], "--config="):
			configPath = strings.TrimPrefix(args[i], "--config=")
		case args[i] == "--json" || args[i] == "-json":
			asJSON = true
		case args[i] == "-h" || args[i] == "--help" || args[i] == "help":
			fmt.Fprint(stdout, usage)
			return 0
		default:
			fmt.Fprintf(stderr, "codeshot: doctor takes --config <path> and --json; %s is not one of them\n", args[i])
			return 2
		}
	}
	f := look(configPath)
	if asJSON {
		return writeJSON(stdout, stderr, f)
	}
	f.write(stdout)
	return 0
}

func look(configPath string) findings {
	var f findings
	f.Version = version()

	own, configErr := config.Load(configPath)
	switch {
	case configErr != nil:
		f.Config.Codeshot.Error = configErr.Error()
	case own.Path == "":
		f.Config.Codeshot.Suggested = firstPath(config.Paths())
	default:
		f.Config.Codeshot.Path = own.Path
		f.Config.Codeshot.Keys = own.Order
	}
	if gh, found := loadGhostty(); found {
		f.Config.Ghostty = &ghosttyFindings{
			Path: gh.Path, Theme: gh.Theme, FontFamily: gh.FontFamily,
			FontSize: gh.FontSize, PaddingX: gh.PaddingX, PaddingY: gh.PaddingY,
		}
	}

	src := themeSource()
	// Asked for, not spelled out: a third embedded theme would make a
	// hardcoded pair here lie, which is the fault `codeshot themes` had.
	f.Themes.Embedded = theme.Source{}.Installed()
	f.Themes.Directories = src.Dirs
	f.Themes.Resolvable = len(src.Installed())

	index := loadFontIndex()
	f.Fonts.DrawingWith = drawingWith(own, index)
	f.Fonts.Index = fonts.CachePath()
	f.Fonts.Families = len(index.Families)
	f.Fonts.Scanned = fonts.SystemFontDirs()
	f.Fonts.Fallbacks = installedFallbacks(index)
	f.Fonts.Symbols = index.SymbolFamilies()

	f.Output.Gallery = defaultGallery()
	if err := writable(f.Output.Gallery); err != nil {
		f.Output.GalleryError = err.Error()
	} else {
		f.Output.GalleryWritable = true
	}
	if err := clipboardAvailable(); err != nil {
		f.Output.ClipboardProblem = err.Error()
	} else {
		f.Output.Clipboard = true
	}

	f.Terminal.Cols, f.Terminal.Rows = tty.Size(os.Stderr)
	f.Terminal.Terminal = tty.IsTerminal(os.Stderr)

	// An empty list is [] in the JSON, never null, so a caller can range over
	// any of them without checking first.
	for _, list := range []*[]string{&f.Config.Codeshot.Keys, &f.Themes.Embedded, &f.Themes.Directories,
		&f.Fonts.Scanned, &f.Fonts.Fallbacks, &f.Fonts.Symbols} {
		if *list == nil {
			*list = []string{}
		}
	}
	return f
}

func (f findings) write(w io.Writer) {
	out := &section{w: w}
	fmt.Fprintf(w, "codeshot %s\n", f.Version)

	out.head("Config")
	own := f.Config.Codeshot
	switch {
	case own.Error != "":
		out.line("codeshot", "%s", own.Error)
	case own.Path == "":
		out.line("codeshot", "none; put one at %s", own.Suggested)
	default:
		out.line("codeshot", "%s (%s)", home.Tildify(own.Path), strings.Join(own.Keys, ", "))
	}
	if gh := f.Config.Ghostty; gh != nil {
		out.line("ghostty", "%s", home.Tildify(gh.Path))
		out.line("  theme", "%s", orNone(gh.Theme))
		out.line("  font-family", "%s", orNone(gh.FontFamily))
		out.line("  font-size", "%s", orZero(gh.FontSize))
		out.line("  padding", "%d/%d", gh.PaddingX, gh.PaddingY)
	} else {
		out.line("ghostty", "none; codeshot uses its own defaults")
	}

	out.head("Themes")
	out.line("embedded", "%s", strings.Join(f.Themes.Embedded, ", "))
	if len(f.Themes.Directories) == 0 {
		out.line("directories", "none found; --theme takes a path to a theme file")
	}
	for _, dir := range f.Themes.Directories {
		out.line("directories", "%s", dir)
	}
	out.line("resolvable", "%d names", f.Themes.Resolvable)

	out.head("Fonts")
	out.line("drawing with", "%s", f.Fonts.DrawingWith)
	out.line("index", "%s (%d families)", home.Tildify(f.Fonts.Index), f.Fonts.Families)
	for _, dir := range f.Fonts.Scanned {
		out.line("scanned", "%s", home.Tildify(dir))
	}
	if len(f.Fonts.Fallbacks) == 0 {
		out.line("fallbacks", "none installed; a rune the font lacks will be a box")
	}
	for _, name := range f.Fonts.Fallbacks {
		out.line("fallback", "%s", name)
	}
	// Its own line because "the icons are crossed-out boxes" is a question
	// doctor should answer outright: a terminal that draws them may be doing
	// it with a copy compiled into the terminal itself, which is a file
	// codeshot has no way to read.
	if len(f.Fonts.Symbols) == 0 {
		out.line("symbols", "%s", "install `brew install --cask font-symbols-only-nerd-font` "+
			"to draw Nerd Font icons; none is installed, so they draw as boxes")
	}
	for _, name := range f.Fonts.Symbols {
		out.line("symbols", "%s", name)
	}

	out.head("Output")
	if f.Output.GalleryWritable {
		out.line("gallery", "%s (writable)", home.Tildify(f.Output.Gallery))
	} else {
		out.line("gallery", "%s (%s)", home.Tildify(f.Output.Gallery), f.Output.GalleryError)
	}
	if f.Output.Clipboard {
		out.line("clipboard", "available for --clip")
	} else {
		out.line("clipboard", "%s", f.Output.ClipboardProblem)
	}

	out.head("Terminal")
	if f.Terminal.Terminal {
		out.line("size", "%dx%d, from stderr", f.Terminal.Cols, f.Terminal.Rows)
	} else {
		out.line("size", "%dx%d; stderr is not a terminal here, so this is the fallback", f.Terminal.Cols, f.Terminal.Rows)
	}
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
	return fmt.Sprintf("JetBrains Mono NL (embedded), because %s from %s is not installed", name, from)
}

func installedFallbacks(index fonts.Index) []string {
	found := []string{}
	for _, name := range fonts.FallbackFamilies() {
		if fam, ok := index.Lookup(name); ok && !fam.Files[0].Empty() {
			found = append(found, fam.Name)
		}
	}
	return found
}

func writable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot be created: %w", err)
	}
	probe := filepath.Join(dir, ".codeshot-write-test")
	f, err := os.Create(probe)
	if err != nil {
		return fmt.Errorf("not writable: %w", err)
	}
	f.Close()
	os.Remove(probe)
	return nil
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
