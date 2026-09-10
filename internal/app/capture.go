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

func (s Service) Run(req Request) (string, error) {
	capture, err := s.Source.Capture()
	if err != nil {
		return "", err
	}
	result, err := s.Emu.Emulate(capture)
	if err != nil {
		return "", fmt.Errorf("emulating the capture: %w", err)
	}
	header, err := s.header(capture, req)
	if err != nil {
		return "", err
	}
	theme, err := s.Themes.Theme(themeName(req.Theme))
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("rendering: %w", err)
	}

	name := domain.ResolveName(req.Name, capture.Command, s.Gallery.Exists)
	path, err := s.Gallery.Store(name, img)
	if err != nil {
		return "", err
	}
	s.Report.Stored(path)
	return path, nil
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

// title prefers what the program set with an OSC, because that is what the
// real window would have shown, and falls back to the command itself.
func title(r domain.Result, c domain.Capture) string {
	if r.Title != "" {
		return r.Title
	}
	return c.Command
}

func themeName(name string) string {
	if name == "" {
		return "codeshot-dark"
	}
	return name
}
