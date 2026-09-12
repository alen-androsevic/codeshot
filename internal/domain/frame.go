package domain

// Capture is everything one command produced. It is the only thing any
// CaptureSource ever yields, whatever the source: a pty, a pipe or a file.
type Capture struct {
	Command  string
	Cwd      string
	ExitCode int
	Cols     int
	Rows     int
	Bytes    []byte
}

// Result is what an Emulator makes of a Capture. Main carries the normal
// buffer including everything that scrolled off; Alt is exactly Cols x Rows
// and is only meaningful when UsedAlt is set.
type Result struct {
	Main    Grid
	Alt     Grid
	UsedAlt bool
	Title   string
}

// FrameOptions crops the composed grid. Rows of 0 means no crop at all.
type FrameOptions struct {
	Rows int
	Tail bool
}

// Frame is the grid that will be drawn. The title is not here: the window is
// titled from Chrome.Title, which the app layer fills in from the command or
// an OSC title, and a second copy on the Frame was written by Compose and
// read by nobody.
type Frame struct {
	Grid Grid
}

// Compose applies the framing rule. A program that took the alternate screen
// is shown as its final screen and nothing else: it painted over the whole
// terminal, so a prompt line above it would be a fiction. Everything else is
// the prompt, the command, and every line of output including what scrolled
// away.
func Compose(header Grid, r Result, opts FrameOptions) Frame {
	g := r.Alt
	if !r.UsedAlt {
		g = Join(header, r.Main).TrimTrailingBlank()
	}
	if opts.Tail {
		g = g.Tail(opts.Rows)
	} else {
		g = g.Head(opts.Rows)
	}
	return Frame{Grid: g}
}
