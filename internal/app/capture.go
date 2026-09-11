package app

import (
	"fmt"

	"codeshot/internal/domain"
)

// Request is one shot's worth of choices, already parsed. A zero Chrome means
// the defaults; a zero Theme name means the default theme.
type Request struct {
	Name     string
	Theme    string
	NoPrompt bool
	Frame    domain.FrameOptions
	Chrome   domain.Chrome
}

// Outcome is what one run produced: where the image landed, and what the
// command codeshot wrapped exited with. The exit code is the child's, not
// codeshot's - a source that never ran a child reports 0 - and the CLI turns
// it into codeshot's own status so that the wrapper is drop-in in front of
// anything.
type Outcome struct {
	Path     string
	ExitCode int
}

// Service runs the one pipeline codeshot has: source, emulate, frame, render,
// store, report.
type Service struct {
	Source  CaptureSource
	Emu     Emulator
	Prompt  PromptSource
	Themes  ThemeSource
	Render  Renderer
	Gallery Gallery
	Report  Reporter
}

func (s Service) Run(req Request) (Outcome, error) {
	capture, err := s.Source.Capture()
	if err != nil {
		// A source can fail and still know the exit code - a wrapped command
		// that was never found is 127, the way a shell reports it - so the
		// code comes out even though there is nothing to render.
		return Outcome{ExitCode: capture.ExitCode}, err
	}
	// From here on the child has already run and its output has already
	// passed through, so every failure below carries the exit code out with
	// it: losing the image must not look like losing the command.
	out := Outcome{ExitCode: capture.ExitCode}
	result, err := s.Emu.Emulate(capture)
	if err != nil {
		return out, fmt.Errorf("emulating the capture: %w", err)
	}
	header, err := s.header(capture, req)
	if err != nil {
		return out, err
	}
	theme, err := s.Themes.Theme(themeName(req.Theme))
	if err != nil {
		return out, err
	}

	chrome := req.Chrome
	if chrome.Scale == 0 {
		chrome = domain.DefaultChrome()
	}
	if chrome.Title == "" {
		chrome.Title = title(result, capture)
	}

	img, err := s.Render.Render(domain.Window{
		Frame:  domain.Compose(header, result, req.Frame),
		Chrome: chrome,
		Theme:  theme,
	})
	if err != nil {
		return out, fmt.Errorf("rendering: %w", err)
	}

	name := domain.ResolveName(req.Name, capture.Command, s.Gallery.Exists)
	path, err := s.Gallery.Store(name, img)
	if err != nil {
		return out, err
	}
	s.Report.Stored(path)
	out.Path = path
	return out, nil
}

// header runs the prompt through the emulator too, so a prompt with colour in
// it is styled by exactly the same code as the output below it.
func (s Service) header(c domain.Capture, req Request) (domain.Grid, error) {
	if req.NoPrompt {
		return domain.Grid{}, nil
	}
	bytes := s.Prompt.Header(c)
	res, err := s.Emu.Emulate(domain.Capture{Bytes: bytes, Cols: c.Cols, Rows: c.Rows})
	if err != nil {
		return domain.Grid{}, fmt.Errorf("emulating the prompt: %w", err)
	}
	return res.Main.TrimTrailingBlank(), nil
}

// title prefers the command, and falls back to an OSC title only when no
// command is known.
//
// The other order is tempting - an OSC title is what the real window would
// have shown - but nearly every interactive shell sets one, to something like
// "alen@host: ~/code". A user asking for a picture of `make build` would then
// get their shell's status line as the caption. The command is what they
// named, so the command is what the window is called; the OSC title covers
// pipe mode without the shim, where the command is genuinely unknown.
func title(r domain.Result, c domain.Capture) string {
	if c.Command != "" {
		return c.Command
	}
	return r.Title
}

func themeName(name string) string {
	if name == "" {
		return "codeshot-dark"
	}
	return name
}
