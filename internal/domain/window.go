package domain

// Controls selects which window buttons are drawn. It is deliberately not tied
// to the host operating system: a Linux box can render macOS traffic lights,
// because what is being drawn is a picture of a window, not this window.
type Controls uint8

const (
	ControlsMacOS Controls = iota
	ControlsLinux
	ControlsNone
)

// Chrome is everything around the grid. Lengths are pixels at Scale 1 and are
// multiplied by Scale when rasterised.
type Chrome struct {
	Controls  Controls
	Title     string
	ShowTitle bool
	// PaddingX and PaddingY are the gaps between the grid and the window's
	// edges. They are separate because Ghostty's are: window-padding-x and
	// window-padding-y are set independently, and a shot that means to look
	// like that window has to be able to follow both.
	PaddingX       int
	PaddingY       int
	Radius         int
	TitlebarHeight int
	Shadow         bool
	Margin         int
	Scale          int
	// Background paints behind the margin. Nil leaves it transparent, which is
	// what makes a shot drop cleanly onto any README.
	Background *RGBA
}

func DefaultChrome() Chrome {
	return Chrome{
		Controls:       ControlsMacOS,
		ShowTitle:      true,
		PaddingX:       14,
		PaddingY:       14,
		Radius:         10,
		TitlebarHeight: 28,
		Shadow:         true,
		// 64 rather than a rounder 48: the shadow spreads about forty pixels
		// and sits eighteen lower, and a clipped shadow ends in a hard
		// straight edge that reads as a rendering bug.
		Margin: 64,
		Scale:  2,
	}
}

// Window is the whole picture: what to draw, how to frame it, in what colours.
type Window struct {
	Frame  Frame
	Chrome Chrome
	Theme  Theme
}
