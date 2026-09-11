// Package cli parses arguments and builds the object graph for one run. It is
// the only package that knows flags exist, and the only one that decides what
// an exit code should be.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"codeshot/internal/adapters/capture/file"
	"codeshot/internal/adapters/capture/pty"
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
  codeshot [name] [flags] -- <command> [args...]   run a command and picture it
  codeshot render <file.ansi> [name] [flags]       render a saved ANSI dump
  codeshot themes                                  list the embedded themes
  codeshot version

With no name, the shot is named after the command (ls -la -> ls-la.png) and
stored in the gallery. A name with a / in it is a path. Everything after --
belongs to the command. codeshot exits with the command's own status.

Flags:
  --command <text>     the command line shown after the prompt (default: the
                       command that ran; render has none unless given)
  --cwd <path>         the directory to show in the prompt
  --cols <n>           terminal width (default: your terminal's, or 100)
  --rows <n>           crop to this many lines (0 keeps them all)
  --tail               crop from the bottom instead of the top
  --no-prompt          leave out the prompt and command lines
  --theme <name|path>  codeshot-dark, codeshot-light, or a Ghostty theme file
  --title <text>       window title (default: the command)
  --no-title           draw no title
  --controls <style>   macos, linux or none (default macos)
  --scale <n>          pixel scale, at least 1 (default 2)
  --padding <n>        pixels around the grid (default 14)
  --margin <n>         pixels around the window (default 64, 0 with --no-shadow)
  --no-shadow          drop the drop shadow
  --background <hex>   fill the margin instead of leaving it transparent
  --font-size <n>      points (default 13)
  --line-height <f>    multiple of the font's own height (default 1.0)
  --gallery <dir>      where bare names are stored (default ~/Codeshots)
  --debug              report every escape sequence the emulator ignored
`

// Run is codeshot from argv to exit code. stdin is an *os.File rather than an
// io.Reader because wrapper mode puts it into raw mode when it is a terminal,
// which is something done to a file descriptor; nil forwards nothing.
func Run(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, "codeshot: nothing to do; try `codeshot -- <command>` or `codeshot render <file.ansi>`\n")
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
	}
	// Anything that is not a subcommand is wrapper mode: a name, flags, or
	// the -- itself. The subcommands are matched first so that `codeshot
	// render x.ansi -- name.png` keeps the meaning it has always had.
	return wrap(args, stdin, stdout, stderr)
}

// wrap runs a command under a pty and pictures it. Its exit code is the
// child's: codeshot in front of a command must not change what a script
// gating on that command sees.
func wrap(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	before, argv, found := cutAtDoubleDash(args)
	if wantsHelp(before) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if !found || len(argv) == 0 {
		fmt.Fprint(stderr, "codeshot: nothing to run; put the command after --, as in "+
			"`codeshot [name] -- ls -la` (reading output from a pipe is not supported yet)\n")
		return 2
	}
	opts, positional, err := parse("codeshot", before, stderr)
	if err != nil {
		return usageExit(stderr, err)
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "codeshot: one name at most before --, got %q\n", positional)
		return 2
	}
	name := ""
	if len(positional) == 1 {
		name = positional[0]
	}
	reporter := report.Writer{Err: stderr}
	// Everything that can fail before the child runs fails here, while
	// failing still costs nothing: no command has been run for nothing.
	service, err := opts.build(pty.Source{
		Argv:    argv,
		Command: opts.command,
		Cwd:     opts.cwd,
		Cols:    opts.cols,
		Stdin:   stdin,
		Stdout:  stdout,
		Size:    fileOf(stderr),
	}, reporter)
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	out, err := service.Run(opts.request(name))
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		// The child's failure outranks codeshot's. Design §10 says both
		// "exit with the child's code" and "a codeshot failure exits 1"; when
		// both happen, a script gating on `codeshot -- make test` must see
		// the tests fail, not a theme typo. Only a clean child with a lost
		// picture exits 1.
		if out.ExitCode != 0 {
			return out.ExitCode
		}
		return 1
	}
	return out.ExitCode
}

// cutAtDoubleDash splits argv at the first --: codeshot's arguments before
// it, the command's after. found reports whether there was a -- at all.
func cutAtDoubleDash(args []string) (before, after []string, found bool) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:], true
		}
	}
	return args, nil, false
}

// fileOf is the *os.File behind w, when there is one. The pty is sized from
// stderr, and a stderr that is a buffer - in a test - has no size to give.
func fileOf(w io.Writer) *os.File {
	f, _ := w.(*os.File)
	return f
}

func render(args []string, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	opts, positional, err := parse("render", args, stderr)
	if err != nil {
		return usageExit(stderr, err)
	}
	if len(positional) == 0 {
		fmt.Fprint(stderr, "codeshot: render needs a file to read\n")
		return 2
	}
	name := ""
	if len(positional) > 1 {
		name = positional[1]
	}
	reporter := report.Writer{Err: stderr}
	service, err := opts.build(file.Source{
		Path:    positional[0],
		Command: opts.command,
		Cwd:     opts.cwd,
		Cols:    opts.cols,
		Warn:    reporter.Warn,
	}, reporter)
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	if _, err := service.Run(opts.request(name)); err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	return 0
}

// wantsHelp reports whether the caller asked for the usage block, stopping at
// a --. Help is intercepted before flag.FlagSet ever sees it: left to the
// FlagSet, ContinueOnError returns ErrHelp only *after* printing its own dump
// of single-dash flags with empty descriptions, so the usage block above -
// which spells the flags the way the README does - was unreachable. The -- is
// where codeshot's arguments end and a child command's begin, and that
// command's own --help is not codeshot's to answer.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

// errReported stands for a failure the flag package has already written to
// stderr. Printing "codeshot: <it>" afterwards would say the same thing
// twice.
var errReported = errors.New("reported")

func usageExit(stderr io.Writer, err error) int {
	if !errors.Is(err, errReported) {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
	}
	return 2
}

// options is every flag, parsed and validated. It exists so that the two
// modes that take the same flags - a saved dump and a command codeshot runs
// itself - share one definition of what those flags mean and one set of
// checks, and differ only in the CaptureSource they build.
type options struct {
	command    string
	cwd        string
	cols       int
	rows       int
	tail       bool
	noPrompt   bool
	theme      string
	fontSize   float64
	lineHeight float64
	gallery    string
	debug      bool
	chrome     domain.Chrome
}

// parse turns argv into options and the positional arguments left over. Every
// error it returns is a mistake in what the caller typed, and exits 2.
func parse(mode string, args []string, stderr io.Writer) (options, []string, error) {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		command = fs.String("command", "", "")
		cwd     = fs.String("cwd", "", "")
		// --cols has no default of its own on purpose. vt.Adapter already
		// falls back to 100 for a capture that never learned its width, and
		// repeating that number here would leave wrapper mode - which sizes
		// the pty from stderr - unable to tell "not set" from "set to 100".
		cols       = fs.Int("cols", 0, "")
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
		return options{}, nil, errReported
	}
	named := namedFlags(fs)

	chrome := domain.DefaultChrome()
	chrome.Scale = *scale
	chrome.Padding = *padding
	chrome.Margin = *margin
	if *noShadow && !named["margin"] {
		// The margin exists to hold the blur, which spreads about forty
		// pixels and sits eighteen lower. With no shadow to hold there is
		// nothing in it, and 64px of dead transparent border on every side is
		// not what --no-shadow asks for. An explicit --margin 64 still means
		// 64, which is why this asks whether the flag was named rather than
		// comparing its value against the default.
		chrome.Margin = 0
	}
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
		return options{}, nil, fmt.Errorf("--controls %q is not macos, linux or none", *controls)
	}
	if *scale < 1 {
		return options{}, nil, fmt.Errorf("--scale must be at least 1, got %d", *scale)
	}
	if *background != "" {
		c, err := theme.ParseColor(*background)
		if err != nil {
			return options{}, nil, fmt.Errorf("--background: %w", err)
		}
		chrome.Background = &c
	}

	return options{
		command:    *command,
		cwd:        *cwd,
		cols:       *cols,
		rows:       *rows,
		tail:       *tail,
		noPrompt:   *noPrompt,
		theme:      *themeName,
		fontSize:   *fontSize,
		lineHeight: *lineHeight,
		gallery:    *galleryDir,
		debug:      *debug,
		chrome:     chrome,
	}, positional, nil
}

// build wires the adapters around a source. It is the composition root: the
// one place that knows which concrete implementation stands behind each port.
// An error here is codeshot failing at its own job, not the caller mistyping,
// and exits 1.
func (o options) build(src app.CaptureSource, reporter report.Writer) (app.Service, error) {
	set, err := fonts.Embedded()
	if err != nil {
		return app.Service{}, err
	}
	emulator := vt.Adapter{}
	if o.debug {
		emulator.Unknown = func(seq string) { reporter.Warn("ignored " + seq) }
	}
	return app.Service{
		Source:  src,
		Emu:     emulator,
		Prompt:  prompt.Template{},
		Themes:  theme.Source{},
		Render:  raster.New(set, raster.Options{FontSize: o.fontSize, LineHeight: o.lineHeight}),
		Gallery: gallery.FS{Dir: o.gallery},
		Report:  reporter,
	}, nil
}

func (o options) request(name string) app.Request {
	return app.Request{
		Name:     name,
		Theme:    o.theme,
		NoPrompt: o.noPrompt,
		Frame:    domain.FrameOptions{Rows: o.rows, Tail: o.tail},
		Chrome:   o.chrome,
	}
}

// namedFlags reports which flags the caller actually wrote out, which
// flag.FlagSet offers no direct query for. It is what lets one flag's default
// depend on another without trampling a value the user set by hand: a
// hardcoded default cannot tell "not set" from "set to the same number".
func namedFlags(fs *flag.FlagSet) map[string]bool {
	named := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { named[f.Name] = true })
	return named
}

// split separates positional arguments from flags, so that both orders work:
// `render file.ansi --rows 24` and `render --rows 24 file.ansi`.
func split(args []string) (positional, flags []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			// A bare -- ends the flags, the way it does everywhere else.
			// Without this it looked like a flag, and takesValue defaults to
			// true, so it ate the argument after it: `render hi.ansi --
			// name.png` exited 0 and wrote codeshot.png. Phase 2's primary
			// syntax is `codeshot shot.png -- npm test`, so this separator
			// has to carry its usual meaning.
			positional = append(positional, args[i+1:]...)
			return positional, flags
		}
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
