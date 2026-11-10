// Package cli parses arguments and builds the object graph for one run. It is
// the only package that knows flags exist, and the only one that decides what
// an exit code should be.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"

	"codeshot/internal/adapters/capture/file"
	"codeshot/internal/adapters/capture/pipe"
	"codeshot/internal/adapters/capture/plain"
	"codeshot/internal/adapters/capture/pty"
	"codeshot/internal/adapters/clipboard"
	"codeshot/internal/adapters/config"
	"codeshot/internal/adapters/fonts"
	"codeshot/internal/adapters/gallery"
	"codeshot/internal/adapters/ghostty"
	"codeshot/internal/adapters/prompt"
	"codeshot/internal/adapters/render/raster"
	"codeshot/internal/adapters/report"
	"codeshot/internal/adapters/theme"
	"codeshot/internal/adapters/tty"
	"codeshot/internal/adapters/vt"
	"codeshot/internal/app"
	"codeshot/internal/domain"
	"codeshot/internal/home"
)

// Version is stamped at build time by install.sh; it is only ever printed.
var Version = "dev"

// The clipboard adapter, reachable as variables so a test can stand in for
// it. A test that used the real one would walk off with whatever the person
// running it had copied.
var (
	clipboardCopy      = clipboard.Copy
	clipboardAvailable = clipboard.Available
)

// loadFontIndex is the machine's fonts, as a variable so a test can decide
// what appears to be installed. The real one is cached between runs; a test
// that used it would scan several hundred files and then depend on which of
// them the machine happened to have.
var loadFontIndex = func() fonts.Index {
	return fonts.LoadIndex(fonts.CachePath(), fonts.SystemFontDirs())
}

// loadGhostty is Ghostty's config, as a variable so a test can decide what
// the machine it runs on appears to have. Without that, the suite's pictures
// would depend on whether whoever ran it uses Ghostty, and on what theme
// they had set that day.
var loadGhostty = ghostty.Load

const usage = `codeshot - a shot of a command and what it printed

Examples:
  codeshot -- git status                    ~/Codeshots/git-status.png
  codeshot shot.png -- npm test             ~/Codeshots/shot.png
  codeshot ./shot.png -- npm test           a path, used as given
  codeshot --clip -- ls -la                 to the clipboard, no file
  npm test 2>&1 | codeshot --command "npm test"

Usage:
  codeshot [name] [flags] -- <command> [args...]   run a command, take a shot
  <command> | codeshot [name] [flags]              take a shot of what comes in
  codeshot render <file.ansi> [name] [flags]       take a shot of a saved dump
  codeshot themes [--json]                         list the themes that resolve
  codeshot doctor [--json]                         show what codeshot found
  codeshot version

With no name, the shot is named after the command (ls -la -> ls-la.png) and
stored in the gallery. A name with a / in it is a path. A file already there
goes to the Trash first. Everything after -- belongs to the command, and
codeshot exits with the command's own status.

A pipe is not a terminal, so a command on the left of one has already
dropped its colour before codeshot sees a byte, and codeshot cannot know what
that command was. The wrapper has neither problem and is the better half;
shim/codeshot.zsh and shim/codeshot.bash recover the command line for a pipe.

Flags:
  --out <name>         where the shot goes; the same as [name]
  --stdout             write the PNG to standard output, output to stderr
  --clip               copy the shot to the clipboard, writing no file
  --command <text>     the command line after the prompt (default: the one
                       that ran; render has none unless given)
  --cwd <path>         the directory in the prompt (default: the current one)
  --cols <n>           terminal width (default: your terminal's, or 100)
  --max-cols <n>       cap the width (default 132; 0 disables)
  --min-cols <n>       floor for content-fit width (default 80; 0 disables)
  --rows <n>           crop to this many lines (default 0, keeps them all)
  --tail               crop from the bottom instead of the top
  --no-prompt          leave out the prompt and command lines
  --theme <name|path>  a name codeshot themes lists, or a Ghostty theme file
                       (default: Ghostty's theme, else codeshot-dark)
  --title <text>       window title (default: the command)
  --no-title           draw no title
  --controls <style>   macos, linux or none (default macos)
  --scale <n>          pixel scale, at least 1 (default 2)
  --padding <n>        pixels around the grid (default: Ghostty's, else 14)
  --margin <n>         pixels around the window (default 64, 0 with --no-shadow)
  --no-shadow          drop the drop shadow
  --background <hex>   fill the margin (default: transparent)
  --radius <n>         corner radius in pixels (default 10)
  --font <family>      a family codeshot doctor lists (default: Ghostty's
                       font-family, else the embedded JetBrains Mono NL)
  --font-size <n>      points (default: Ghostty's, else 13)
  --line-height <f>    multiple of the font's own height (default 1.0)
  --prompt <text>      the prompt; {cwd} is the directory (default: {cwd} in
                       cyan, then a green ❯ on the next line)
  --config <path>      the defaults file (default ~/.config/codeshot/config)
  --gallery <dir>      where bare names are stored (default ~/Codeshots)
  --debug              report every escape sequence the emulator ignored

Flags win over ~/.config/codeshot/config, which wins over Ghostty's config,
which wins over the defaults above. Every flag is also a key in that file.
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
		fmt.Fprintf(stdout, "codeshot %s\n", version())
		return 0
	case "themes":
		return themes(args[1:], stdout, stderr)
	case "doctor":
		return doctor(args[1:], stdout, stderr)
	case "render":
		return render(args[1:], stdout, stderr)
	}
	// Anything that is not a subcommand captures: a command after --, or a
	// pipeline's output on standard input. The subcommands are matched first
	// so that `codeshot render x.ansi -- name.png` keeps the meaning it has
	// always had.
	return capture(args, stdin, stdout, stderr)
}

// capture is both modes that take a live command: one codeshot runs itself
// after --, and one whose output arrives on standard input. They differ in
// where the bytes come from and in whether there is an exit code worth
// having; everything after the source is the same pipeline.
func capture(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	before, argv, found := cutAtDoubleDash(args)
	if wantsHelp(before) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if found && len(argv) == 0 {
		fmt.Fprint(stderr, "codeshot: put the command to run after --\n")
		return 2
	}
	// Pipe mode is exactly the case where standard input is not a terminal.
	// At a prompt it is one, and `codeshot shot.png` on its own is somebody
	// expecting a command.
	piped := stdin != nil && !tty.IsTerminal(stdin)
	if !found && !piped {
		fmt.Fprint(stderr, "codeshot: put a command after --, as in "+
			"`codeshot [name] -- ls -la`, or pipe a command's output in\n")
		return 2
	}
	opts, positional, err := parse("codeshot", before, stderr)
	if err != nil {
		return usageExit(stderr, err)
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "codeshot: one name at most, got %q\n", positional)
		return 2
	}
	name := ""
	if len(positional) == 1 {
		name = positional[0]
	}
	reporter := report.Writer{Err: stderr}
	dest, name, err := opts.output(name, stdout, reporter)
	if err != nil {
		return usageExit(stderr, err)
	}
	// --stdout owns standard output, so what the command printed moves over
	// to stderr rather than being interleaved into a PNG.
	passthrough := stdout
	if opts.stdout {
		passthrough = stderr
	}
	// The child's colour and the pty's \r\n are there because codeshot gave
	// it a terminal. A passthrough that is not one gets what the command
	// would have written without codeshot in front of it. A pipe's input is
	// already that, so pipe mode passes it on untouched.
	var flat *plain.Writer
	if f := fileOf(passthrough); found && (f == nil || !tty.IsTerminal(f)) {
		flat = &plain.Writer{W: passthrough}
		passthrough = flat
	}

	var source app.CaptureSource
	if found {
		source = pty.Source{
			Argv:    argv,
			Command: opts.command,
			Cwd:     opts.cwd,
			Cols:    opts.cols,
			MaxCols: opts.maxCols,
			Stdin:   stdin,
			Stdout:  passthrough,
			Size:    fileOf(stderr),
		}
	} else {
		source = pipe.Source{
			In:      stdin,
			Command: opts.command,
			Cwd:     opts.cwd,
			Cols:    opts.cols,
			MaxCols: opts.maxCols,
			Stdout:  passthrough,
			Size:    fileOf(stderr),
		}
	}

	// Everything that can fail before the command runs fails here, while
	// failing still costs nothing: no command has been run for nothing.
	service, err := opts.build(source, reporter, dest)
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	out, err := service.Run(opts.request(name))
	if flat != nil {
		flat.Flush()
	}
	if errors.Is(err, pipe.ErrNothing) {
		fmt.Fprint(stderr, "codeshot: put a command after --, as in `codeshot [name] -- ls -la`, "+
			"or pipe one in with --command; standard input was empty, so there is nothing to picture\n")
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		// The command's failure outranks codeshot's. Design §10 says both
		// "exit with the child's code" and "a codeshot failure exits 1"; when
		// both happen, a script gating on `codeshot -- make test` must see
		// the tests fail, not a theme typo. Only a clean command with a lost
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
	dest, name, err := opts.output(name, stdout, reporter)
	if err != nil {
		return usageExit(stderr, err)
	}
	cols := opts.cols
	if opts.maxCols > 0 && cols > opts.maxCols {
		cols = opts.maxCols
	}
	service, err := opts.build(file.Source{
		Path:    positional[0],
		Command: opts.command,
		Cwd:     opts.cwd,
		Cols:    cols,
		Warn:    reporter.Warn,
	}, reporter, dest)
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

// themes lists the theme names that resolve here. Asking the source rather
// than naming the two embedded themes: with a Ghostty installation's
// directories in the list this prints the several hundred that actually
// resolve, and it cannot go stale the way a hardcoded pair does.
func themes(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "-h", "--help", "help":
			fmt.Fprint(stdout, usage)
			return 0
		case "--json", "-json":
			asJSON = true
		default:
			fmt.Fprintf(stderr, "codeshot: themes takes only --json; %s is not one of its flags\n", a)
			return 2
		}
	}
	names := themeSource().Installed()
	if asJSON {
		return writeJSON(stdout, stderr, names)
	}
	for _, name := range names {
		fmt.Fprintln(stdout, name)
	}
	return 0
}

// writeJSON is the --json output of themes and doctor: indented, so that a
// person can read it too, and ending in a newline.
func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "codeshot: %v\n", err)
		return 1
	}
	return 0
}

// version is what install.sh stamped in, or else what the Go toolchain
// recorded: the module version for `go install ...@v1.2.0`, the commit for a
// plain `go build` in a checkout.
func version() string {
	if Version != "dev" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	// A pseudo-version, v0.0.0-20260924162053-11e18db50c5e+dirty, is what Go
	// stamps on a plain build in a checkout; the commit says the same in less.
	if v := info.Main.Version; v != "" && v != "(devel)" && !pseudoVersion.MatchString(v) {
		return v
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return Version
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return "dev-" + rev
}

var pseudoVersion = regexp.MustCompile(`-(0\.)?\d{14}-[0-9a-f]{12}`)

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
		if a == "-h" || a == "-help" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

func usageExit(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "codeshot: %v\n", err)
	return 2
}

// flagError says what the flag package found wrong in codeshot's own words.
// Left to itself it prints "flag provided but not defined: -nope" and then a
// dump of every flag with a single dash and an empty description, none of
// which says what to type instead.
var invalidValue = regexp.MustCompile(`^invalid (?:boolean )?value "(.*)" for flag -([^:]+): `)

func flagError(fs *flag.FlagSet, err error) error {
	msg := err.Error()
	if name, ok := strings.CutPrefix(msg, "flag provided but not defined: -"); ok {
		return fmt.Errorf("--%s is not a flag; `codeshot --help` lists them", strings.TrimPrefix(name, "-"))
	}
	if name, ok := strings.CutPrefix(msg, "flag needs an argument: -"); ok {
		return fmt.Errorf("put a value after --%s; `codeshot --help` says what it takes", strings.TrimPrefix(name, "-"))
	}
	if m := invalidValue.FindStringSubmatch(msg); m != nil {
		want := "a value it takes"
		if f := fs.Lookup(m[2]); f != nil {
			switch f.Value.(flag.Getter).Get().(type) {
			case int:
				want = "a whole number"
			case float64:
				want = "a number"
			case bool:
				want = "true or false"
			}
		}
		return fmt.Errorf("give --%s %s; %q is not one", m[2], want, m[1])
	}
	return err
}

// options is every flag, parsed and validated. It exists so that the two
// modes that take the same flags - a saved dump and a command codeshot runs
// itself - share one definition of what those flags mean and one set of
// checks, and differ only in the CaptureSource they build.
type options struct {
	command    string
	cwd        string
	cols       int
	maxCols    int
	rows       int
	tail       bool
	noPrompt   bool
	theme      string
	fontSize   float64
	lineHeight float64
	gallery    string
	// unknownConfigKeys are keys in codeshot's own config that are not flag
	// names. That file is codeshot's, so a key it does not know is a typo
	// worth mentioning rather than a setting that is none of its business.
	unknownConfigKeys []string
	configPath        string
	// font is --font: the family the caller named, which must exist.
	font string
	// prompt is --prompt: the template drawn above the output.
	prompt string
	// fontFamily is what Ghostty's config asked for. Nothing can honour it
	// until the system font index lands; doctor reports it meanwhile.
	fontFamily string
	out        string
	stdout     bool
	clip       bool
	debug      bool
	chrome     domain.Chrome
}

// parse turns argv into options and the positional arguments left over. Every
// error it returns is a mistake in what the caller typed, and exits 2.
func parse(mode string, args []string, stderr io.Writer) (options, []string, error) {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		command = fs.String("command", "", "")
		cwd     = fs.String("cwd", "", "")
		// --cols has no default of its own on purpose. vt.Adapter already
		// falls back to 100 for a capture that never learned its width, and
		// repeating that number here would leave wrapper mode - which sizes
		// the pty from stderr - unable to tell "not set" from "set to 100".
		cols       = fs.Int("cols", 0, "")
		maxCols    = fs.Int("max-cols", 132, "")
		minCols    = fs.Int("min-cols", domain.DefaultChrome().MinCols, "")
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
		fontName   = fs.String("font", "", "")
		promptText = fs.String("prompt", "", "")
		radius     = fs.Int("radius", domain.DefaultChrome().Radius, "")
		configPath = fs.String("config", "", "")
		out        = fs.String("out", "", "")
		toStdout   = fs.Bool("stdout", false, "")
		clip       = fs.Bool("clip", false, "")
		debug      = fs.Bool("debug", false, "")
	)
	// Flags may follow the positional arguments, which flag.FlagSet does not
	// do on its own, so the positionals are lifted out first.
	positional, flags := split(args)
	if err := fs.Parse(flags); err != nil {
		return options{}, nil, flagError(fs, err)
	}
	named := namedFlags(fs)

	chrome := domain.DefaultChrome()
	chrome.Scale = *scale
	chrome.PaddingX, chrome.PaddingY = *padding, *padding
	chrome.Radius = *radius
	chrome.Margin = *margin
	chrome.MinCols = *minCols
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

	// codeshot's own config comes next in design §4's precedence, under the
	// flags and over Ghostty's. Every key is a flag name, so it is applied
	// through the flag set itself: one definition of what a setting means,
	// and a value that will not parse is rejected here exactly as it would
	// be on the command line.
	own, err := config.Load(*configPath)
	if err != nil {
		return options{}, nil, err
	}
	decided := map[string]bool{}
	for name := range named {
		decided[name] = true
	}
	known := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { known[f.Name] = true })
	for _, key := range own.Order {
		if key == "config" || decided[key] || !known[key] {
			continue
		}
		value := own.Values[key]
		if isBoolFlag(fs.Lookup(key)) {
			b, ok := own.Bool(key)
			if !ok {
				return options{}, nil, fmt.Errorf("%s: %s = %q is not a yes or a no", own.Path, key, value)
			}
			value = strconv.FormatBool(b)
		}
		if err := fs.Set(key, value); err != nil {
			return options{}, nil, fmt.Errorf("%s: %s: %w", own.Path, key, err)
		}
		decided[key] = true
	}

	// The chrome was assembled from the flag values before the config had
	// its say, so the settings the config may have changed are read again.
	chrome.Scale = *scale
	chrome.PaddingX, chrome.PaddingY = *padding, *padding
	chrome.Radius = *radius
	chrome.Margin = *margin
	chrome.MinCols = *minCols
	if *noShadow && !decided["margin"] {
		chrome.Margin = 0
	}
	chrome.Shadow = !*noShadow
	chrome.ShowTitle = !*noTitle
	chrome.Title = *title
	if *scale < 1 {
		return options{}, nil, fmt.Errorf("--scale must be at least 1, got %d", *scale)
	}

	// Ghostty's config fills in what the caller did not name, which is what
	// makes a shot look like the terminal it came from. Design §4's
	// precedence, from strongest: flags, codeshot's own config, Ghostty's,
	// built-in defaults. Anything Ghostty says that codeshot cannot make
	// sense of was already dropped by the parser.
	gh, _ := loadGhostty()
	if !decided["theme"] && gh.Theme != "" {
		*themeName = gh.Theme
	}
	if !decided["font-size"] && gh.FontSize > 0 {
		*fontSize = gh.FontSize
	}
	if !decided["padding"] {
		if gh.PaddingX > 0 {
			chrome.PaddingX = gh.PaddingX
		}
		if gh.PaddingY > 0 {
			chrome.PaddingY = gh.PaddingY
		}
	}

	unknown := own.Unknown(known)

	return options{
		unknownConfigKeys: unknown,
		configPath:        own.Path,
		command:           *command,
		cwd:               *cwd,
		cols:              *cols,
		maxCols:           *maxCols,
		rows:              *rows,
		tail:              *tail,
		noPrompt:          *noPrompt,
		theme:             *themeName,
		fontSize:          *fontSize,
		lineHeight:        *lineHeight,
		gallery:           *galleryDir,
		font:              *fontName,
		prompt:            *promptText,
		fontFamily:        gh.FontFamily,
		out:               *out,
		stdout:            *toStdout,
		clip:              *clip,
		debug:             *debug,
		chrome:            chrome,
	}, positional, nil
}

// build wires the adapters around a source. It is the composition root: the
// one place that knows which concrete implementation stands behind each port.
// An error here is codeshot failing at its own job, not the caller mistyping,
// and exits 1.
func (o options) build(src app.CaptureSource, reporter report.Writer, dest app.Gallery) (app.Service, error) {
	for _, key := range o.unknownConfigKeys {
		reporter.Warn(fmt.Sprintf("%s: no setting called %q", o.configPath, key))
	}
	set, err := o.fontSet(reporter)
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
		Prompt:  prompt.Template{Text: o.prompt},
		Themes:  themeSource(),
		Render:  raster.New(set, raster.Options{FontSize: o.fontSize, LineHeight: o.lineHeight}),
		Gallery: dest,
		Report:  reporter,
	}, nil
}

// fontSet picks the family to draw with: --font, then whatever Ghostty's
// config asked for, then the embedded one. The index goes in either way, so
// that a rune the family has no glyph for can still be found somewhere on
// the machine.
//
// The two ways of naming a font fail differently. --font is a request, and a
// request for a font that is not installed is a mistake worth stopping for.
// Ghostty's font-family is a setting from someone else's file that codeshot
// is reading opportunistically, so a family it cannot find is worth a word
// on stderr and nothing more.
func (o options) fontSet(reporter report.Writer) (*fonts.Set, error) {
	index := loadFontIndex()
	name := o.font
	if name == "" {
		name = o.fontFamily
	}
	if name == "" {
		set, err := fonts.Embedded()
		if err != nil {
			return nil, err
		}
		return set.WithIndex(index), nil
	}
	family, ok := index.Lookup(name)
	if !ok {
		if o.font != "" {
			return nil, fmt.Errorf("pick a font family `codeshot doctor` lists; %q is not installed", name)
		}
		reporter.Warn(fmt.Sprintf("drawing with the embedded font: Ghostty's font-family %q is not installed here", name))
		set, err := fonts.Embedded()
		if err != nil {
			return nil, err
		}
		return set.WithIndex(index), nil
	}
	set, err := fonts.Load(family)
	if err != nil {
		if o.font != "" {
			return nil, err
		}
		reporter.Warn(fmt.Sprintf("drawing with the embedded font: %v", err))
		set, err := fonts.Embedded()
		if err != nil {
			return nil, err
		}
		return set.WithIndex(index), nil
	}
	return set.WithIndex(index), nil
}

// themeSource resolves theme names against the embedded themes first and a
// local Ghostty installation's several hundred second, which is what makes
// --theme tokyonight work without codeshot shipping copies of other
// projects' palettes under their names.
func themeSource() theme.Source {
	return theme.Source{Dirs: theme.GhosttyDirs()}
}

// output decides where the picture goes and settles the name it is asked to
// go by. --out and a positional name are two spellings of one thing, and
// --clip and --stdout each replace the file entirely, so a name alongside
// either has nothing to name: all three combinations are mistakes worth
// stopping for rather than precedence rules to remember.
func (o options) output(name string, stdout io.Writer, reporter report.Writer) (app.Gallery, string, error) {
	if o.out != "" {
		if name != "" {
			return nil, "", fmt.Errorf("--out %s and the name %s say the same thing; use one", o.out, name)
		}
		name = o.out
	}
	if o.clip && o.stdout {
		return nil, "", errors.New("--clip and --stdout each replace the file; use one")
	}
	if elsewhere := o.clip || o.stdout; elsewhere && name != "" {
		where := "--clip"
		if o.stdout {
			where = "--stdout"
		}
		return nil, "", fmt.Errorf("%s writes no file, so there is nothing for %s to name", where, name)
	}
	switch {
	case o.stdout:
		return gallery.Writer{W: stdout}, name, nil
	case o.clip:
		if err := clipboardAvailable(); err != nil {
			return nil, "", err
		}
		return gallery.Clip{Copy: clipboardCopy}, name, nil
	}
	return gallery.FS{Dir: o.gallery, Replaced: func(path, trashed string) {
		reporter.Warn(fmt.Sprintf("moved the old %s to %s", home.Tildify(path), home.Tildify(trashed)))
	}}, name, nil
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
// isBoolFlag reports whether a flag is one that takes no value, which is how
// a config key gets `yes` and `on` translated into what flag.Set accepts.
func isBoolFlag(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

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
		"--no-shadow", "-no-shadow", "--debug", "-debug", "--stdout", "-stdout",
		"--clip", "-clip":
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
