// Package cli parses arguments and builds the object graph for one run. It is
// the only package that knows flags exist, and the only one that decides what
// an exit code should be.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"codeshot/internal/adapters/capture/file"
	"codeshot/internal/adapters/fonts"
	"codeshot/internal/adapters/gallery"
	"codeshot/internal/adapters/prompt"
	"codeshot/internal/adapters/render/raster"
	"codeshot/internal/adapters/report"
	"codeshot/internal/adapters/theme"
	"codeshot/internal/adapters/vt"
	"codeshot/internal/app"
	"codeshot/internal/domain"
)

// Version is stamped at build time by install.sh; it is only ever printed.
var Version = "dev"

const usage = `codeshot - a picture of a command and what it printed

Usage:
  codeshot render <file.ansi> [name] [flags]   render a saved ANSI dump
  codeshot themes                              list the embedded themes
  codeshot version

Flags for render:
  --command <text>     the command line to show above the output
  --cwd <path>         the directory to show in the prompt
  --cols <n>           terminal width the dump was produced at (default 100)
  --rows <n>           crop to this many lines (0 keeps them all)
  --tail               crop from the bottom instead of the top
  --no-prompt          leave out the prompt and command lines
  --theme <name|path>  codeshot-dark, codeshot-light, or a Ghostty theme file
  --title <text>       window title (default: the command)
  --no-title           draw no title
  --controls <style>   macos, linux or none (default macos)
  --scale <n>          pixel scale, at least 1 (default 2)
  --padding <n>        pixels around the grid (default 14)
  --margin <n>         pixels around the window (default 64)
  --no-shadow          drop the drop shadow
  --background <hex>   fill the margin instead of leaving it transparent
  --font-size <n>      points (default 13)
  --line-height <f>    multiple of the font's own height (default 1.0)
  --gallery <dir>      where bare names are stored (default ~/Codeshots)
  --debug              report every escape sequence the emulator ignored
`

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, "codeshot: nothing to do; try `codeshot render <file.ansi>`\n")
		return 2
	}
	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "codeshot %s\n", Version)
		return 0
	case "themes":
		fmt.Fprint(stdout, "codeshot-dark\ncodeshot-light\n")
		return 0
	case "render":
		return render(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "codeshot: unknown command %q; try `codeshot --help`\n", args[0])
		return 2
	}
}

func render(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		command    = fs.String("command", "", "")
		cwd        = fs.String("cwd", "", "")
		cols       = fs.Int("cols", 100, "")
		rows       = fs.Int("rows", 0, "")
		tail       = fs.Bool("tail", false, "")
		noPrompt   = fs.Bool("no-prompt", false, "")
		themeName  = fs.String("theme", "", "")
		title      = fs.String("title", "", "")
		noTitle    = fs.Bool("no-title", false, "")
		controls   = fs.String("controls", "macos", "")
		scale      = fs.Int("scale", 2, "")
		padding    = fs.Int("padding", 14, "")
		margin     = fs.Int("margin", 64, "")
		noShadow   = fs.Bool("no-shadow", false, "")
		background = fs.String("background", "", "")
		fontSize   = fs.Float64("font-size", 13, "")
		lineHeight = fs.Float64("line-height", 1, "")
		galleryDir = fs.String("gallery", defaultGallery(), "")
		debug      = fs.Bool("debug", false, "")
	)
	// Flags may follow the positional arguments, which flag.FlagSet does not
	// do on its own, so the positionals are lifted out first.
	positional, flags := split(args)
	if err := fs.Parse(flags); err != nil {
		return 2
	}
	if len(positional) == 0 {
		fmt.Fprint(stderr, "codeshot: render needs a file to read\n")
		return 2
	}

	chrome := domain.DefaultChrome()
	chrome.Scale = *scale
	chrome.Padding = *padding
	chrome.Margin = *margin
	chrome.Shadow = !*noShadow
	chrome.ShowTitle = !*noTitle
	chrome.Title = *title
	switch *controls {
	case "macos":
		chrome.Controls = domain.ControlsMacOS
	case "linux":
		chrome.Controls = domain.ControlsLinux
	case "none":
		chrome.Controls = domain.ControlsNone
	default:
		fmt.Fprintf(stderr, "codeshot: --controls %q is not macos, linux or none\n", *controls)
		return 2
	}
	if *scale < 1 {
		fmt.Fprintf(stderr, "codeshot: --scale must be at least 1, got %d\n", *scale)
		return 2
	}
	if *background != "" {
		c, err := theme.ParseColor(*background)
		if err != nil {
			fmt.Fprintf(stderr, "codeshot: --background: %v\n", err)
			return 2
		}
		chrome.Background = &c
	}

	set, err := fonts.Embedded()
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	reporter := report.Writer{Err: stderr}
	emulator := vt.Adapter{}
	if *debug {
		emulator.Unknown = func(seq string) { reporter.Warn("ignored " + seq) }
	}

	name := ""
	if len(positional) > 1 {
		name = positional[1]
	}
	service := app.Service{
		Source: file.Source{
			Path:    positional[0],
			Command: *command,
			Cwd:     *cwd,
			Cols:    *cols,
		},
		Emu:     emulator,
		Prompt:  prompt.Template{},
		Themes:  theme.Source{},
		Render:  raster.New(set, raster.Options{FontSize: *fontSize, LineHeight: *lineHeight}),
		Gallery: gallery.FS{Dir: *galleryDir},
		Report:  reporter,
	}
	if _, err := service.Run(app.Request{
		Name:     name,
		Theme:    *themeName,
		NoPrompt: *noPrompt,
		Frame:    domain.FrameOptions{Rows: *rows, Tail: *tail},
		Chrome:   chrome,
	}); err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	return 0
}

// split separates positional arguments from flags, so that both orders work:
// `render file.ansi --rows 24` and `render --rows 24 file.ansi`.
func split(args []string) (positional, flags []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			// A flag that takes a value swallows the next argument unless the
			// value is already attached with =.
			if i+1 < len(args) && !hasValue(a) && takesValue(a) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return positional, flags
}

func hasValue(flag string) bool {
	for i := 0; i < len(flag); i++ {
		if flag[i] == '=' {
			return true
		}
	}
	return false
}

// takesValue lists the flags that are followed by a value. The booleans are
// the exception, and naming them here is cheaper than reflecting over the set.
func takesValue(flag string) bool {
	switch flag {
	case "--tail", "-tail", "--no-prompt", "-no-prompt", "--no-title", "-no-title",
		"--no-shadow", "-no-shadow", "--debug", "-debug":
		return false
	}
	return true
}

func defaultGallery() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Codeshots"
	}
	return filepath.Join(home, "Codeshots")
}
