package app

import (
	"errors"
	"image"
	"strings"
	"testing"

	"codeshot/internal/domain"
)

type stubSource struct{ c domain.Capture }

func (s stubSource) Capture() (domain.Capture, error) { return s.c, nil }

type stubEmulator struct{ seen []string }

// Emulate wraps each line at c.Cols, the way a real terminal would at the
// right margin. Honouring Cols (rather than ignoring it, as an earlier
// version of this stub did) matters because Service.header re-emulates the
// prompt at the capture's own width: a stub that ignores Cols would agree
// just as happily with a caller that emulated the header at the wrong width,
// silently misaligning the header against the body.
func (e *stubEmulator) Emulate(c domain.Capture) (domain.Result, error) {
	e.seen = append(e.seen, string(c.Bytes))
	g := domain.Grid{Cols: c.Cols}
	for _, line := range strings.Split(strings.TrimSuffix(string(c.Bytes), "\r\n"), "\r\n") {
		g.Lines = append(g.Lines, wrapLine(line, c.Cols)...)
	}
	return domain.Result{Main: g}, nil
}

// wrapLine breaks a line into rows of at most cols runes. cols <= 0 means no
// wrapping.
func wrapLine(line string, cols int) [][]domain.Cell {
	runes := []rune(line)
	if cols <= 0 || len(runes) <= cols {
		return [][]domain.Cell{cellsOf(runes)}
	}
	var rows [][]domain.Cell
	for len(runes) > 0 {
		n := cols
		if n > len(runes) {
			n = len(runes)
		}
		rows = append(rows, cellsOf(runes[:n]))
		runes = runes[n:]
	}
	return rows
}

func cellsOf(runes []rune) []domain.Cell {
	row := make([]domain.Cell, 0, len(runes))
	for _, r := range runes {
		row = append(row, domain.Cell{Rune: r, Width: 1})
	}
	return row
}

type stubPrompt struct{}

func (stubPrompt) Header(c domain.Capture) []byte { return []byte("> " + c.Command + "\r\n") }

type stubThemes struct{ err error }

func (s stubThemes) Theme(name string) (domain.Theme, error) {
	if s.err != nil {
		return domain.Theme{}, s.err
	}
	return domain.Theme{Name: name}, nil
}

type stubRenderer struct{ got domain.Window }

func (r *stubRenderer) Render(w domain.Window) (image.Image, error) {
	r.got = w
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

type stubGallery struct {
	stored string
	taken  map[string]bool
}

func (g *stubGallery) Exists(name string) bool { return g.taken[name] }
func (g *stubGallery) Store(name string, img image.Image) (string, error) {
	g.stored = name
	return "/gallery/" + name, nil
}

type stubReporter struct{ path string }

func (r *stubReporter) Stored(path string) { r.path = path }
func (r *stubReporter) Warn(string)        {}

func newService() (Service, *stubRenderer, *stubGallery, *stubReporter) {
	rend := &stubRenderer{}
	gal := &stubGallery{taken: map[string]bool{}}
	rep := &stubReporter{}
	return Service{
		Source:  stubSource{domain.Capture{Command: "ls -la", Cwd: "/tmp", Cols: 10, Rows: 4, Bytes: []byte("a.go\r\nb.go\r\n")}},
		Emu:     &stubEmulator{},
		Prompt:  stubPrompt{},
		Themes:  stubThemes{},
		Render:  rend,
		Gallery: gal,
		Report:  rep,
	}, rend, gal, rep
}

func TestRunComposesPromptAboveOutput(t *testing.T) {
	s, rend, _, _ := newService()
	if _, err := s.Run(Request{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := rend.got.Frame.Grid.Text(); got != "> ls -la\na.go\nb.go\n" {
		t.Errorf("frame = %q", got)
	}
}

func TestRunHonoursNoPrompt(t *testing.T) {
	s, rend, _, _ := newService()
	if _, err := s.Run(Request{NoPrompt: true}); err != nil {
		t.Fatal(err)
	}
	if got := rend.got.Frame.Grid.Text(); got != "a.go\nb.go\n" {
		t.Errorf("frame = %q, want no prompt line", got)
	}
}

func TestRunTitlesTheWindowWithTheCommand(t *testing.T) {
	s, rend, _, _ := newService()
	s.Run(Request{})
	if rend.got.Chrome.Title != "ls -la" {
		t.Errorf("Title = %q, want the command", rend.got.Chrome.Title)
	}
}

func TestRunPrefersAnExplicitTitle(t *testing.T) {
	s, rend, _, _ := newService()
	c := domain.DefaultChrome()
	c.Title = "chosen"
	s.Run(Request{Chrome: c})
	if rend.got.Chrome.Title != "chosen" {
		t.Errorf("Title = %q", rend.got.Chrome.Title)
	}
}

func TestRunNamesAndStoresTheShot(t *testing.T) {
	s, _, gal, rep := newService()
	path, err := s.Run(Request{})
	if err != nil {
		t.Fatal(err)
	}
	if gal.stored != "ls-la.png" {
		t.Errorf("stored %q, want ls-la.png", gal.stored)
	}
	if path != "/gallery/ls-la.png" || rep.path != path {
		t.Errorf("path %q, reported %q", path, rep.path)
	}
}

func TestRunFailsWhenTheThemeIsUnknown(t *testing.T) {
	s, _, _, _ := newService()
	s.Themes = stubThemes{err: errors.New("no such theme")}
	if _, err := s.Run(Request{Theme: "nope"}); err == nil {
		t.Error("Run ignored an unresolvable theme")
	}
}

func TestRunPassesFrameOptionsThrough(t *testing.T) {
	s, rend, _, _ := newService()
	s.Run(Request{Frame: domain.FrameOptions{Rows: 1}})
	if rend.got.Frame.Grid.Rows() != 1 {
		t.Errorf("rows = %d, want the crop applied", rend.got.Frame.Grid.Rows())
	}
}

func TestRunEmulatesTheHeaderAtTheCaptureWidth(t *testing.T) {
	// The header is emulated at the capture's own Cols/Rows so it wraps the
	// same way the real output would. stubPrompt renders "> ls -la\r\n" - 8
	// runes - which at Cols=5 wraps into "> ls " and "-la". If Service.header
	// emulated it at some other width (999, say), it would not wrap and the
	// composed text would read "> ls -la" on one line instead of two, so this
	// is a direct check that the capture's width, not some fixed one, made
	// the trip.
	rend := &stubRenderer{}
	s := Service{
		Source:  stubSource{domain.Capture{Command: "ls -la", Cwd: "/tmp", Cols: 5, Rows: 4, Bytes: []byte("ab\r\ncd\r\n")}},
		Emu:     &stubEmulator{},
		Prompt:  stubPrompt{},
		Themes:  stubThemes{},
		Render:  rend,
		Gallery: &stubGallery{taken: map[string]bool{}},
		Report:  &stubReporter{},
	}
	if _, err := s.Run(Request{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "> ls\n-la\nab\ncd\n"
	if got := rend.got.Frame.Grid.Text(); got != want {
		t.Errorf("frame = %q, want %q (header wrapped at the capture's width)", got, want)
	}
}
