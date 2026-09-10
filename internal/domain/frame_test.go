package domain

import "testing"

func TestComposeStacksHeaderAboveOutput(t *testing.T) {
	header := gridOf(8, "~", "> ls")
	res := Result{Main: gridOf(8, "a.go", "b.go", "")}
	f := Compose(header, res, FrameOptions{})
	if got, want := f.Grid.Text(), "~\n> ls\na.go\nb.go\n"; got != want {
		t.Errorf("Compose = %q, want %q", got, want)
	}
}

func TestComposeDropsTheHeaderForAlternateScreenPrograms(t *testing.T) {
	header := gridOf(8, "~", "> htop")
	res := Result{
		Main:    gridOf(8, "leftovers"),
		Alt:     gridOf(8, "TUI", ""),
		UsedAlt: true,
	}
	f := Compose(header, res, FrameOptions{})
	if got, want := f.Grid.Text(), "TUI\n\n"; got != want {
		t.Errorf("alt frame = %q, want the alt screen verbatim %q", got, want)
	}
}

func TestComposeCropsToRows(t *testing.T) {
	res := Result{Main: gridOf(4, "a", "b", "c", "d")}
	if got := Compose(Grid{}, res, FrameOptions{Rows: 2}).Grid.Text(); got != "a\nb\n" {
		t.Errorf("Rows crop = %q, want the first two", got)
	}
	if got := Compose(Grid{}, res, FrameOptions{Rows: 2, Tail: true}).Grid.Text(); got != "c\nd\n" {
		t.Errorf("Tail crop = %q, want the last two", got)
	}
}

func TestComposeCarriesTheTitle(t *testing.T) {
	f := Compose(Grid{}, Result{Main: gridOf(2, "x"), Title: "ls"}, FrameOptions{})
	if f.Title != "ls" {
		t.Errorf("Title = %q, want ls", f.Title)
	}
}

func TestDefaultChromeMatchesTheSpec(t *testing.T) {
	c := DefaultChrome()
	if c.Controls != ControlsMacOS || !c.ShowTitle || !c.Shadow {
		t.Errorf("defaults = %+v, want macOS controls, title and shadow on", c)
	}
	if c.TitlebarHeight != 28 || c.Radius != 10 || c.Margin != 64 || c.Scale != 2 {
		t.Errorf("geometry = %+v, want 28/10/64/2 per the spec", c)
	}
}
