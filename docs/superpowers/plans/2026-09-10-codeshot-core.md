# codeshot Phase 1 (Core) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the pure `bytes → PNG` core of codeshot: a VT emulator, a native Go rasteriser that draws a Ghostty-looking window, and a working `codeshot render session.ansi shot.png` command.

**Architecture:** Hexagonal. `internal/domain` is pure — a grid of styled cells, a theme, framing rules, window chrome — and imports nothing but the standard library. `internal/app` declares the ports and holds the one use case. Every adapter (VT emulator, rasteriser, theme parser, font set, gallery) lives under `internal/adapters` and is wired together only in `cmd/codeshot`. Nothing in this phase opens a pty or branches on the operating system.

**Tech Stack:** Go 1.27, `module codeshot`. Dependencies: `github.com/mattn/go-runewidth`, `golang.org/x/image`. Hand-rolled argument parsing, no CLI framework. Vendored JetBrains Mono (OFL).

**Spec:** `docs/superpowers/specs/2026-09-10-codeshot-design.md`

## Global Constraints

- Module path is bare `codeshot`, not a GitHub URL. Internal imports read `codeshot/internal/domain`.
- Only three third-party modules may appear in `go.mod` across the whole project: `github.com/creack/pty` (phase 2 only), `github.com/mattn/go-runewidth`, `golang.org/x/image`. Adding anything else needs a decision recorded in `docs/adr/`.
- `CGO_ENABLED=0` must build on darwin and linux. No cgo, ever.
- `internal/domain` imports only the standard library. If you find yourself importing `golang.org/x/image` there, the type belongs in an adapter.
- Colours in `Style` are palette-relative. A `domain.Style` never holds a resolved RGB value except via `ColorRGB`, which the terminal itself specified.
- TDD: every step-pair is a failing test then the minimal code to pass it. Run `go test ./...` before every commit.
- `gofmt -l .` must print nothing before every commit.
- Comments explain *why*, in prose, the way `~/code/paradajz` does. No comment restates the code.
- Commit after every task, with a subject line in the imperative mood and no attribution trailer.

## File Structure

| File | Responsibility |
|---|---|
| `internal/domain/color.go` | `Color`, `ColorKind` — palette-relative colour references |
| `internal/domain/style.go` | `Style`, `Attr` — per-cell appearance |
| `internal/domain/theme.go` | `RGBA`, `Theme`, `Resolve`, `XTerm256` — the only place colour becomes concrete |
| `internal/domain/grid.go` | `Cell`, `Grid` and its cropping/joining operations |
| `internal/domain/frame.go` | `Capture`, `Result`, `FrameOptions`, `Frame`, `Compose` — the framing rule |
| `internal/domain/window.go` | `Chrome`, `Controls`, `Window`, `DefaultChrome` |
| `internal/domain/name.go` | `Slug`, `ResolveName` — gallery naming rules |
| `internal/adapters/vt/vt.go` | Emulator struct, `Write`, ground state, printing, scrollback |
| `internal/adapters/vt/buffer.go` | Screen buffer: cursor, line feed, erase, scroll |
| `internal/adapters/vt/csi.go` | CSI parsing and cursor/erase/scroll dispatch |
| `internal/adapters/vt/sgr.go` | SGR parameter decoding into `domain.Style` |
| `internal/adapters/vt/mode.go` | Alt screen, DECAWM, OSC title, unknown-sequence reporting |
| `internal/adapters/theme/theme.go` | Ghostty theme-file parser and theme lookup |
| `internal/adapters/theme/assets/*.conf` | `codeshot-dark`, `codeshot-light` |
| `internal/adapters/fonts/fonts.go` | Embedded face set, metrics, per-rune coverage |
| `internal/adapters/fonts/assets/` | Vendored JetBrains Mono TTFs + OFL licence |
| `internal/adapters/render/raster/render.go` | `Renderer`, image sizing, orchestration |
| `internal/adapters/render/raster/text.go` | Cell grid → pixels |
| `internal/adapters/render/raster/chrome.go` | Rounded window, titlebar, traffic lights, title |
| `internal/adapters/render/raster/shadow.go` | Box-blur shadow and margin background |
| `internal/adapters/capture/file/file.go` | `.ansi` dump → `domain.Capture` |
| `internal/adapters/prompt/prompt.go` | Prompt template → header bytes |
| `internal/adapters/gallery/fs.go` | Filesystem gallery |
| `internal/app/ports.go` | Every driven port interface |
| `internal/app/capture.go` | The `Service.Run` use case |
| `internal/cli/cli.go` | Flag parsing and the `render` subcommand |
| `cmd/codeshot/main.go` | Composition root |

---

### Task 1: Repository skeleton, colours and styles

**Files:**
- Create: `go.mod`, `internal/domain/color.go`, `internal/domain/style.go`
- Test: `internal/domain/style_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `domain.ColorKind` (`ColorDefault`, `ColorIndexed`, `ColorRGB`); `domain.Color{Kind ColorKind; Index, R, G, B uint8}`; constructors `DefaultColor() Color`, `IndexedColor(uint8) Color`, `RGBColor(r, g, b uint8) Color`; `domain.Attr` bit flags `AttrBold`, `AttrDim`, `AttrItalic`, `AttrUnderline`, `AttrStrike`, `AttrInverse`, `AttrHidden`; `domain.Style{FG, BG Color; Attrs Attr}` with `Has(Attr) bool`, `Set(Attr) Style`, `Clear(Attr) Style`.

- [ ] **Step 1: Initialise the module**

```bash
cd ~/code/codeshot
go mod init codeshot
go mod edit -go=1.27
```

- [ ] **Step 2: Write the failing test**

Create `internal/domain/style_test.go`:

```go
package domain

import "testing"

func TestColorConstructorsCarryTheirKind(t *testing.T) {
	if got := DefaultColor(); got.Kind != ColorDefault {
		t.Errorf("DefaultColor kind = %v, want ColorDefault", got.Kind)
	}
	if got := IndexedColor(9); got.Kind != ColorIndexed || got.Index != 9 {
		t.Errorf("IndexedColor(9) = %+v, want indexed 9", got)
	}
	got := RGBColor(0x1D, 0x1F, 0x21)
	if got.Kind != ColorRGB || got.R != 0x1D || got.G != 0x1F || got.B != 0x21 {
		t.Errorf("RGBColor = %+v, want rgb 1d1f21", got)
	}
}

func TestStyleAttributesSetAndClearIndependently(t *testing.T) {
	s := Style{}.Set(AttrBold).Set(AttrUnderline)
	if !s.Has(AttrBold) || !s.Has(AttrUnderline) {
		t.Fatalf("Set lost an attribute: %b", s.Attrs)
	}
	if s.Has(AttrItalic) {
		t.Errorf("italic set without being asked for")
	}
	s = s.Clear(AttrBold)
	if s.Has(AttrBold) || !s.Has(AttrUnderline) {
		t.Errorf("Clear(AttrBold) = %b, want underline only", s.Attrs)
	}
}

func TestStyleValueSemantics(t *testing.T) {
	base := Style{}
	base.Set(AttrBold)
	if base.Has(AttrBold) {
		t.Error("Set mutated the receiver; Style must be a value type")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/domain/`
Expected: FAIL — undefined: `DefaultColor`, `ColorDefault`, `Style`, and friends.

- [ ] **Step 4: Write the implementation**

Create `internal/domain/color.go`:

```go
// Package domain holds codeshot's pure core: the grid a terminal produced, the
// theme it is painted with, and the window it is framed in. Nothing here opens
// a file, loads a font or talks to a terminal, which is what makes the whole
// bytes-to-image pipeline testable without one.
package domain

// ColorKind says how a Color must be resolved against a Theme.
type ColorKind uint8

const (
	// ColorDefault is the terminal's own foreground or background, whatever
	// the theme says those are.
	ColorDefault ColorKind = iota
	// ColorIndexed is a palette entry. 0-15 come from the theme; 16-255 are
	// fixed by the xterm cube and ignore it.
	ColorIndexed
	// ColorRGB is a literal 24-bit colour the program asked for by name, so
	// the theme has no say in it at all.
	ColorRGB
)

// Color is a reference to a colour, not a colour. It stays unresolved until a
// Theme is applied, which is what lets one capture be rendered in any theme.
type Color struct {
	Kind    ColorKind
	Index   uint8
	R, G, B uint8
}

func DefaultColor() Color          { return Color{Kind: ColorDefault} }
func IndexedColor(i uint8) Color   { return Color{Kind: ColorIndexed, Index: i} }
func RGBColor(r, g, b uint8) Color { return Color{Kind: ColorRGB, R: r, G: g, B: b} }
```

Create `internal/domain/style.go`:

```go
package domain

// Attr is a set of the appearance bits SGR can turn on for a cell.
type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrStrike
	AttrInverse
	AttrHidden
)

// Style is everything about a cell except which rune is in it.
type Style struct {
	FG, BG Color
	Attrs  Attr
}

func (s Style) Has(a Attr) bool { return s.Attrs&a != 0 }

// Set and Clear return a copy: a Style is a value, so the emulator can keep a
// current style and hand copies of it to thousands of cells without aliasing.
func (s Style) Set(a Attr) Style   { s.Attrs |= a; return s }
func (s Style) Clear(a Attr) Style { s.Attrs &^= a; return s }
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/domain/`
Expected: PASS, 3 tests.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go test ./...
git add go.mod internal/domain/
git commit -m "Add palette-relative colours and cell styles"
```

---

### Task 2: Theme and colour resolution

**Files:**
- Create: `internal/domain/theme.go`
- Test: `internal/domain/theme_test.go`

**Interfaces:**
- Consumes: `Color`, `ColorKind`, `Style`, `Attr` from Task 1.
- Produces: `domain.RGBA{R, G, B, A uint8}`; `domain.Theme{Name string; Background, Foreground, Cursor RGBA; Palette [16]RGBA}`; `(Theme).Resolve(Style) (fg, bg RGBA)`; `domain.XTerm256(uint8) RGBA`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/theme_test.go`:

```go
package domain

import "testing"

func testTheme() Theme {
	t := Theme{
		Name:       "test",
		Background: RGBA{0x10, 0x10, 0x10, 0xFF},
		Foreground: RGBA{0xE0, 0xE0, 0xE0, 0xFF},
	}
	t.Palette[1] = RGBA{0xFF, 0x00, 0x00, 0xFF}
	t.Palette[4] = RGBA{0x00, 0x00, 0xFF, 0xFF}
	return t
}

func TestResolveUsesThemeForDefaults(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{})
	if fg != th.Foreground || bg != th.Background {
		t.Errorf("Resolve(zero) = %v/%v, want theme fg/bg", fg, bg)
	}
}

func TestResolveUsesPaletteForLowIndices(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), BG: IndexedColor(4)})
	if fg != th.Palette[1] || bg != th.Palette[4] {
		t.Errorf("Resolve(indexed) = %v/%v, want palette 1/4", fg, bg)
	}
}

func TestResolveIgnoresThemeForTrueColour(t *testing.T) {
	th := testTheme()
	fg, _ := th.Resolve(Style{FG: RGBColor(1, 2, 3)})
	if (fg != RGBA{1, 2, 3, 0xFF}) {
		t.Errorf("Resolve(rgb) = %v, want 1,2,3 opaque", fg)
	}
}

func TestInverseSwapsAfterResolution(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), Attrs: AttrInverse})
	if fg != th.Background || bg != th.Palette[1] {
		t.Errorf("inverse = %v/%v, want bg/palette1", fg, bg)
	}
}

func TestDimBlendsForegroundTowardBackground(t *testing.T) {
	th := Theme{Background: RGBA{0, 0, 0, 0xFF}, Foreground: RGBA{0xFF, 0xFF, 0xFF, 0xFF}}
	fg, _ := th.Resolve(Style{Attrs: AttrDim})
	if fg.R < 0x7D || fg.R > 0x81 {
		t.Errorf("dim white on black = %v, want about half", fg)
	}
}

func TestHiddenCollapsesForegroundIntoBackground(t *testing.T) {
	th := testTheme()
	fg, bg := th.Resolve(Style{FG: IndexedColor(1), Attrs: AttrHidden})
	if fg != bg {
		t.Errorf("hidden fg %v != bg %v", fg, bg)
	}
}

func TestXTerm256Cube(t *testing.T) {
	cases := map[uint8]RGBA{
		16:  {0x00, 0x00, 0x00, 0xFF},
		21:  {0x00, 0x00, 0xFF, 0xFF},
		231: {0xFF, 0xFF, 0xFF, 0xFF},
		232: {0x08, 0x08, 0x08, 0xFF},
		255: {0xEE, 0xEE, 0xEE, 0xFF},
	}
	for i, want := range cases {
		if got := XTerm256(i); got != want {
			t.Errorf("XTerm256(%d) = %v, want %v", i, got, want)
		}
	}
}

func TestResolveFallsBackToXTermForHighIndices(t *testing.T) {
	th := testTheme()
	fg, _ := th.Resolve(Style{FG: IndexedColor(21)})
	if (fg != RGBA{0x00, 0x00, 0xFF, 0xFF}) {
		t.Errorf("index 21 = %v, want pure blue from the cube", fg)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/domain/ -run 'Theme|Resolve|XTerm|Inverse|Dim|Hidden'`
Expected: FAIL — undefined: `Theme`, `RGBA`, `XTerm256`.

- [ ] **Step 3: Write the implementation**

Create `internal/domain/theme.go`:

```go
package domain

// RGBA is a resolved colour, ready for the rasteriser. Nothing upstream of
// Theme.Resolve is allowed to hold one.
type RGBA struct {
	R, G, B, A uint8
}

// Theme is a terminal colour scheme in the shape Ghostty's theme files use.
type Theme struct {
	Name       string
	Background RGBA
	Foreground RGBA
	Cursor     RGBA
	Palette    [16]RGBA
}

// Resolve turns a palette-relative Style into the two concrete colours a cell
// is painted with. The order of the attribute rules is the order terminals
// apply them: dim blends first, then inverse swaps what dim produced, then
// hidden collapses whatever is left.
func (t Theme) Resolve(s Style) (fg, bg RGBA) {
	fg = t.resolveColor(s.FG, t.Foreground)
	bg = t.resolveColor(s.BG, t.Background)
	if s.Has(AttrDim) {
		fg = blend(fg, bg, 0.5)
	}
	if s.Has(AttrInverse) {
		fg, bg = bg, fg
	}
	if s.Has(AttrHidden) {
		fg = bg
	}
	return fg, bg
}

func (t Theme) resolveColor(c Color, def RGBA) RGBA {
	switch c.Kind {
	case ColorIndexed:
		if c.Index < 16 {
			return t.Palette[c.Index]
		}
		return XTerm256(c.Index)
	case ColorRGB:
		return RGBA{c.R, c.G, c.B, 0xFF}
	default:
		return def
	}
}

// xtermBase is the fixed low half of the 256-colour table. A Theme overrides
// these through its Palette; XTerm256 answers for callers who have no theme.
var xtermBase = [16]RGBA{
	{0x00, 0x00, 0x00, 0xFF}, {0x80, 0x00, 0x00, 0xFF},
	{0x00, 0x80, 0x00, 0xFF}, {0x80, 0x80, 0x00, 0xFF},
	{0x00, 0x00, 0x80, 0xFF}, {0x80, 0x00, 0x80, 0xFF},
	{0x00, 0x80, 0x80, 0xFF}, {0xC0, 0xC0, 0xC0, 0xFF},
	{0x80, 0x80, 0x80, 0xFF}, {0xFF, 0x00, 0x00, 0xFF},
	{0x00, 0xFF, 0x00, 0xFF}, {0xFF, 0xFF, 0x00, 0xFF},
	{0x00, 0x00, 0xFF, 0xFF}, {0xFF, 0x00, 0xFF, 0xFF},
	{0x00, 0xFF, 0xFF, 0xFF}, {0xFF, 0xFF, 0xFF, 0xFF},
}

// XTerm256 resolves any 256-colour index: sixteen base colours, then a
// 6x6x6 cube, then a 24-step grey ramp.
func XTerm256(i uint8) RGBA {
	switch {
	case i < 16:
		return xtermBase[i]
	case i < 232:
		n := int(i) - 16
		level := [6]uint8{0x00, 0x5F, 0x87, 0xAF, 0xD7, 0xFF}
		return RGBA{level[n/36], level[(n/6)%6], level[n%6], 0xFF}
	default:
		v := uint8(8 + 10*(int(i)-232))
		return RGBA{v, v, v, 0xFF}
	}
}

func blend(a, b RGBA, f float64) RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-f) + float64(y)*f + 0.5)
	}
	return RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), a.A}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go test ./...
git add internal/domain/
git commit -m "Resolve palette-relative styles against a theme"
```

---

### Task 3: The cell grid

**Files:**
- Create: `internal/domain/grid.go`
- Test: `internal/domain/grid_test.go`

**Interfaces:**
- Consumes: `Style`, `Color`, `ColorDefault`, `AttrInverse` from Tasks 1-2.
- Produces: `domain.Cell{Rune rune; Style Style; Width uint8}` with `IsBlank() bool`; `domain.Grid{Cols int; Lines [][]Cell}` with `Rows() int`, `TrimTrailingBlank() Grid`, `Head(n int) Grid`, `Tail(n int) Grid`, `Text() string`; `domain.Join(...Grid) Grid`.

`Cell.Width` is `1` for a normal rune, `2` for the leading half of a wide rune, and `0` for the trailing half, which carries no glyph of its own. The rasteriser skips width-0 cells.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/grid_test.go`:

```go
package domain

import "testing"

// gridOf builds a Grid from plain strings, one per line, all cells default.
func gridOf(cols int, lines ...string) Grid {
	g := Grid{Cols: cols}
	for _, s := range lines {
		row := make([]Cell, 0, cols)
		for _, r := range s {
			row = append(row, Cell{Rune: r, Width: 1})
		}
		for len(row) < cols {
			row = append(row, Cell{Rune: ' ', Width: 1})
		}
		g.Lines = append(g.Lines, row)
	}
	return g
}

func TestGridText(t *testing.T) {
	g := gridOf(6, "ab", "cd")
	if got, want := g.Text(), "ab\ncd\n"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestTrimTrailingBlankRemovesEmptyTail(t *testing.T) {
	g := gridOf(4, "x", "", "")
	if got := g.TrimTrailingBlank().Rows(); got != 1 {
		t.Errorf("rows after trim = %d, want 1", got)
	}
}

func TestTrimTrailingBlankKeepsInteriorBlanks(t *testing.T) {
	g := gridOf(4, "x", "", "y", "")
	if got := g.TrimTrailingBlank().Rows(); got != 3 {
		t.Errorf("rows after trim = %d, want 3", got)
	}
}

func TestTrimTrailingBlankKeepsLinesColouredByBackground(t *testing.T) {
	g := gridOf(4, "x", "")
	g.Lines[1][0].Style.BG = IndexedColor(4)
	if got := g.TrimTrailingBlank().Rows(); got != 2 {
		t.Errorf("a background-painted blank line was trimmed; rows = %d, want 2", got)
	}
}

func TestHeadAndTailCrop(t *testing.T) {
	g := gridOf(4, "a", "b", "c", "d")
	if got := g.Head(2).Text(); got != "a\nb\n" {
		t.Errorf("Head(2) = %q", got)
	}
	if got := g.Tail(2).Text(); got != "c\nd\n" {
		t.Errorf("Tail(2) = %q", got)
	}
}

func TestHeadAndTailAreNoOpsWhenNotSmaller(t *testing.T) {
	g := gridOf(4, "a", "b")
	if g.Head(0).Rows() != 2 || g.Head(9).Rows() != 2 || g.Tail(0).Rows() != 2 {
		t.Error("crop to 0 or to more rows than exist must return the grid unchanged")
	}
}

func TestJoinConcatenatesAndWidensToTheWidestGrid(t *testing.T) {
	j := Join(gridOf(4, "a"), gridOf(9, "bb"))
	if j.Cols != 9 {
		t.Errorf("Cols = %d, want 9", j.Cols)
	}
	if j.Rows() != 2 || j.Text() != "a\nbb\n" {
		t.Errorf("Join = %q with %d rows", j.Text(), j.Rows())
	}
}

func TestCellIsBlank(t *testing.T) {
	if !(Cell{Rune: ' ', Width: 1}).IsBlank() || !(Cell{}).IsBlank() {
		t.Error("space and zero cell must both count as blank")
	}
	if (Cell{Rune: 'x', Width: 1}).IsBlank() {
		t.Error("a rune is not blank")
	}
	painted := Cell{Rune: ' ', Width: 1}
	painted.Style.BG = IndexedColor(2)
	if painted.IsBlank() {
		t.Error("a space with a background colour is visible, so not blank")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/domain/ -run 'Grid|Trim|Head|Tail|Join|Cell'`
Expected: FAIL — undefined: `Cell`, `Grid`, `Join`.

- [ ] **Step 3: Write the implementation**

Create `internal/domain/grid.go`:

```go
package domain

import "strings"

// Cell is one terminal cell. Width is 1 for an ordinary rune, 2 for the
// leading half of a wide one, and 0 for its trailing half, which holds no
// glyph and is skipped when drawing.
type Cell struct {
	Rune  rune
	Style Style
	Width uint8
}

// IsBlank reports whether the cell would leave no mark. A space with a
// background colour is not blank: terminals paint it, so codeshot must too.
func (c Cell) IsBlank() bool {
	if c.Rune != 0 && c.Rune != ' ' {
		return false
	}
	return c.Style.BG.Kind == ColorDefault && !c.Style.Has(AttrInverse)
}

// Grid is a rectangular block of cells: what an emulator produced, or a piece
// of it. Lines may be shorter than Cols; missing cells read as blank.
type Grid struct {
	Cols  int
	Lines [][]Cell
}

func (g Grid) Rows() int { return len(g.Lines) }

// TrimTrailingBlank drops empty lines from the bottom. A capture almost always
// ends with the newline the program printed last, and that newline would
// otherwise become an empty row at the foot of the image.
func (g Grid) TrimTrailingBlank() Grid {
	end := len(g.Lines)
	for end > 0 && lineIsBlank(g.Lines[end-1]) {
		end--
	}
	g.Lines = g.Lines[:end]
	return g
}

func lineIsBlank(line []Cell) bool {
	for _, c := range line {
		if !c.IsBlank() {
			return false
		}
	}
	return true
}

// Head keeps the first n lines, Tail the last n. Both are no-ops for n <= 0 or
// n >= the number of lines, so callers can pass an unset option straight in.
func (g Grid) Head(n int) Grid {
	if n <= 0 || n >= len(g.Lines) {
		return g
	}
	g.Lines = g.Lines[:n]
	return g
}

func (g Grid) Tail(n int) Grid {
	if n <= 0 || n >= len(g.Lines) {
		return g
	}
	g.Lines = g.Lines[len(g.Lines)-n:]
	return g
}

// Text renders the grid as plain text, trailing blanks stripped per line. It
// exists for tests and for --debug; nothing in the pipeline consumes it.
func (g Grid) Text() string {
	var b strings.Builder
	for _, line := range g.Lines {
		var row strings.Builder
		for _, c := range line {
			if c.Width == 0 {
				continue
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			row.WriteRune(r)
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// Join stacks grids vertically, taking the widest column count. It is how the
// prompt header gets glued on top of a command's output.
func Join(grids ...Grid) Grid {
	out := Grid{}
	for _, g := range grids {
		if g.Cols > out.Cols {
			out.Cols = g.Cols
		}
		out.Lines = append(out.Lines, g.Lines...)
	}
	return out
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go test ./...
git add internal/domain/
git commit -m "Add the cell grid and its cropping rules"
```

---

### Task 4: Captures, results and the framing rule

**Files:**
- Create: `internal/domain/frame.go`, `internal/domain/window.go`
- Test: `internal/domain/frame_test.go`

**Interfaces:**
- Consumes: `Grid`, `Join`, `TrimTrailingBlank`, `Head`, `Tail` from Task 3.
- Produces:
  - `domain.Capture{Command, Cwd string; ExitCode, Cols, Rows int; Bytes []byte}`
  - `domain.Result{Main, Alt Grid; UsedAlt bool; Title string}`
  - `domain.FrameOptions{Rows int; Tail bool}`
  - `domain.Frame{Grid Grid; Title string}`
  - `domain.Compose(header Grid, r Result, opts FrameOptions) Frame`
  - `domain.Controls` (`ControlsMacOS`, `ControlsLinux`, `ControlsNone`)
  - `domain.Chrome{Controls Controls; Title string; ShowTitle bool; Padding, Radius, TitlebarHeight, Margin, Scale int; Shadow bool; Background *RGBA}`
  - `domain.DefaultChrome() Chrome`
  - `domain.Window{Frame Frame; Chrome Chrome; Theme Theme}`

- [ ] **Step 1: Write the failing test**

Create `internal/domain/frame_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/domain/ -run 'Compose|Chrome'`
Expected: FAIL — undefined: `Compose`, `Result`, `DefaultChrome`.

- [ ] **Step 3: Write the implementation**

Create `internal/domain/frame.go`:

```go
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

// Frame is the grid that will be drawn, and the title over it.
type Frame struct {
	Grid  Grid
	Title string
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
	return Frame{Grid: g, Title: r.Title}
}
```

Create `internal/domain/window.go`:

```go
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
	Controls       Controls
	Title          string
	ShowTitle      bool
	Padding        int
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
		Padding:        14,
		Radius:         10,
		TitlebarHeight: 28,
		Shadow:         true,
		// 64 rather than a rounder 48: the shadow spreads about forty pixels
		// and sits eighteen lower, and a clipped shadow ends in a hard
		// straight edge that reads as a rendering bug.
		Margin: 64,
		Scale:          2,
	}
}

// Window is the whole picture: what to draw, how to frame it, in what colours.
type Window struct {
	Frame  Frame
	Chrome Chrome
	Theme  Theme
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go test ./...
git add internal/domain/
git commit -m "Add the framing rule and window chrome"
```

---

### Task 5: VT emulator — printing, control characters, wrapping, scrollback

**Files:**
- Create: `internal/adapters/vt/buffer.go`, `internal/adapters/vt/vt.go`
- Modify: `internal/domain/grid.go` (add `Cell.Combining`)
- Test: `internal/adapters/vt/vt_test.go`

**Interfaces:**
- Consumes: `domain.Cell`, `domain.Grid`, `domain.Style` from Tasks 1-3.
- Produces: `vt.New(cols, rows int) *vt.Emulator`; `(*Emulator).Write([]byte) (int, error)`; `(*Emulator).Result() domain.Result`; unexported `buffer` with `put`, `combine`, `lineFeed`, `carriageReturn`, `backspace`, `tab`, `blankLine`, `blank`, `row`, `grid`. (`moveTo`, `eraseInLine`, `eraseInDisplay`, `scrollUp` and `scrollDown` arrive in Task 6, not here.)
- Also produces: `domain.Cell.Combining string` — combining marks that hang off the cell's rune. It stays a `string` rather than a `[]rune` so that `Cell` remains comparable.

Note on line feeds: codeshot reads the *master* side of a pty, where the kernel has already turned the program's `\n` into `\r\n`. A bare LF therefore moves down without returning to column one, and the tests below pin that.

- [ ] **Step 1: Add the dependency**

```bash
cd ~/code/codeshot
go get github.com/mattn/go-runewidth@latest
```

- [ ] **Step 2: Write the failing test**

Create `internal/adapters/vt/vt_test.go`:

```go
package vt

import "testing"

// mainText is what the normal buffer looks like once the empty tail is gone.
func mainText(t *testing.T, cols, rows int, in string) string {
	t.Helper()
	e := New(cols, rows)
	if _, err := e.Write([]byte(in)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return e.Result().Main.TrimTrailingBlank().Text()
}

func TestPrintsPlainText(t *testing.T) {
	if got := mainText(t, 10, 3, "hello"); got != "hello\n" {
		t.Errorf("got %q, want %q", got, "hello\n")
	}
}

func TestLineFeedKeepsTheColumn(t *testing.T) {
	// A pty has already applied ONLCR, so a lone LF really does mean
	// "down one, same column".
	if got := mainText(t, 10, 3, "ab\ncd"); got != "ab\n  cd\n" {
		t.Errorf("got %q, want %q", got, "ab\n  cd\n")
	}
}

func TestCarriageReturnOverwrites(t *testing.T) {
	if got := mainText(t, 10, 3, "100%\rdone"); got != "done\n" {
		t.Errorf("got %q, want %q", got, "done\n")
	}
}

func TestBackspaceAndTab(t *testing.T) {
	if got := mainText(t, 20, 3, "abc\b\bX"); got != "aXc\n" {
		t.Errorf("backspace: got %q, want %q", got, "aXc\n")
	}
	if got := mainText(t, 20, 3, "a\tb"); got != "a       b\n" {
		t.Errorf("tab: got %q, want a then column 8", got)
	}
}

func TestAutowrapAtTheRightEdge(t *testing.T) {
	if got := mainText(t, 4, 3, "abcdef"); got != "abcd\nef\n" {
		t.Errorf("got %q, want %q", got, "abcd\nef\n")
	}
}

func TestWideRunesTakeTwoCellsAndNeverStraddleTheEdge(t *testing.T) {
	// Five columns cannot hold three double-width runes; the third wraps
	// rather than being split across the edge.
	got := mainText(t, 5, 3, "日本語")
	if got != "日本\n語\n" {
		t.Errorf("got %q, want %q", got, "日本\n語\n")
	}
}

func TestCombiningMarksAttachToThePrecedingCell(t *testing.T) {
	// Decomposed "c" + U+030C, which is how macOS hands over filenames. The
	// literal is escaped rather than typed, because the precomposed rune looks
	// identical in a source file and would test nothing.
	const decomposed = "c\u030Cao"
	if got := mainText(t, 6, 2, decomposed); got != decomposed+"\n" {
		t.Errorf("got %q, want the mark kept on the c", got)
	}
	e := New(6, 2)
	e.Write([]byte(decomposed))
	if n := len(e.Result().Main.Lines[0]); n != 6 {
		t.Errorf("line length %d, want 6 cells: the mark must not consume one", n)
	}
}

func TestOutputTallerThanTheScreenKeepsScrollback(t *testing.T) {
	// Twenty columns, so nothing wraps and the staircase below is only the
	// line feeds keeping their column.
	got := mainText(t, 20, 2, "one\ntwo\nthree\nfour")
	want := "one\n   two\n      three\n           four\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/adapters/vt/`
Expected: FAIL — undefined: `New`.

- [ ] **Step 4: Add `Combining` to the domain cell**

In `internal/domain/grid.go`, change the `Cell` struct and `Text` method:

```go
// Cell is one terminal cell. Width is 1 for an ordinary rune, 2 for the
// leading half of a wide one, and 0 for its trailing half, which holds no
// glyph and is skipped when drawing. Combining holds any marks that hang off
// the rune - a string rather than a slice so that Cell stays comparable.
type Cell struct {
	Rune      rune
	Combining string
	Style     Style
	Width     uint8
}
```

In `Text`, after `row.WriteRune(r)`, add `row.WriteString(c.Combining)`.

- [ ] **Step 5: Write the buffer**

Create `internal/adapters/vt/buffer.go`:

```go
// Package vt is codeshot's terminal emulator: bytes in, a grid of styled cells
// out. It implements only what command-line programs actually emit - the list
// is in the design doc - and silently ignores the rest, because a shot of the
// output is not a terminal anyone will type into.
package vt

import "codeshot/internal/domain"

// buffer is one screen: a window of rows lines over a line store. The normal
// buffer lets that store grow, which is how scrollback survives; the alternate
// buffer keeps it exactly rows long, because a program that took the whole
// screen has no history worth showing.
type buffer struct {
	cols, rows int
	lines      [][]domain.Cell
	origin     int  // index in lines of the top visible row
	x, y       int  // cursor; y is relative to origin
	wrapNext   bool // the cursor is parked past the last column
	scrollback bool
}

func newBuffer(cols, rows int, scrollback bool) *buffer {
	b := &buffer{cols: cols, rows: rows, scrollback: scrollback}
	for i := 0; i < rows; i++ {
		b.lines = append(b.lines, b.blankLine(domain.Style{}))
	}
	return b
}

func (b *buffer) blankLine(st domain.Style) []domain.Cell {
	line := make([]domain.Cell, b.cols)
	for i := range line {
		line[i] = blank(st)
	}
	return line
}

// blank keeps only the background: an erased cell shows the colour that was
// current when it was erased, but not its underline or its bold.
func blank(st domain.Style) domain.Cell {
	return domain.Cell{Rune: ' ', Width: 1, Style: domain.Style{BG: st.BG}}
}

func (b *buffer) row(y int) []domain.Cell { return b.lines[b.origin+y] }

func (b *buffer) put(r rune, w int, st domain.Style) {
	if w == 0 {
		b.combine(r)
		return
	}
	if b.wrapNext {
		b.carriageReturn()
		b.lineFeed()
	}
	if w == 2 && b.x == b.cols-1 {
		// A double-width rune is never split across the edge; the terminal
		// leaves the last column blank and starts it on the next line.
		b.row(b.y)[b.x] = blank(st)
		b.carriageReturn()
		b.lineFeed()
	}
	line := b.row(b.y)
	line[b.x] = domain.Cell{Rune: r, Style: st, Width: uint8(w)}
	if w == 2 {
		line[b.x+1] = domain.Cell{Width: 0, Style: st}
	}
	b.x += w
	if b.x >= b.cols {
		b.x = b.cols - 1
		b.wrapNext = true
	}
}

// combine hangs a zero-width mark off whatever was printed last. Decomposed
// text - which is how macOS hands over filenames - depends on this.
func (b *buffer) combine(r rune) {
	x := b.x
	if !b.wrapNext {
		x--
	}
	if x < 0 {
		return
	}
	line := b.row(b.y)
	for x > 0 && line[x].Width == 0 {
		x--
	}
	line[x].Combining += string(r)
}

func (b *buffer) carriageReturn() {
	b.x = 0
	b.wrapNext = false
}

func (b *buffer) lineFeed() {
	b.wrapNext = false
	if b.y < b.rows-1 {
		b.y++
		return
	}
	if b.scrollback {
		b.origin++
		b.lines = append(b.lines, b.blankLine(domain.Style{}))
		return
	}
	copy(b.lines, b.lines[1:])
	b.lines[b.rows-1] = b.blankLine(domain.Style{})
}

func (b *buffer) backspace() {
	b.wrapNext = false
	if b.x > 0 {
		b.x--
	}
}

func (b *buffer) tab() {
	b.wrapNext = false
	next := (b.x/8 + 1) * 8
	if next > b.cols-1 {
		next = b.cols - 1
	}
	b.x = next
}

func (b *buffer) grid() domain.Grid {
	end := len(b.lines)
	if !b.scrollback {
		end = b.origin + b.rows
	}
	g := domain.Grid{Cols: b.cols}
	g.Lines = append(g.Lines, b.lines[:end]...)
	return g
}
```

- [ ] **Step 6: Write the emulator's ground state**

Create `internal/adapters/vt/vt.go`:

```go
package vt

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"codeshot/internal/domain"
)

type parseState uint8

const (
	stGround parseState = iota
	stEsc
	stCSI
	stOSC
	stOSCEsc
)

// Emulator turns a byte stream into grids. It is a value with no I/O of its
// own: feed it everything a command printed, then ask for the Result.
type Emulator struct {
	cols, rows int
	main, alt  *buffer
	cur        *buffer
	style      domain.Style
	title      string
	autowrap   bool

	state   parseState
	params  []int
	private byte
	osc     []byte
	utf8buf []byte

	// Unknown, if set, is told about every sequence the emulator ignored. The
	// CLI wires it to --debug; nothing else looks at it.
	Unknown func(seq string)
}

func New(cols, rows int) *Emulator {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	e := &Emulator{cols: cols, rows: rows, autowrap: true}
	e.main = newBuffer(cols, rows, true)
	e.alt = newBuffer(cols, rows, false)
	e.cur = e.main
	return e
}

func (e *Emulator) Write(p []byte) (int, error) {
	for _, b := range p {
		e.step(b)
	}
	return len(p), nil
}

// Result is the emulator's whole output. UsedAlt means the stream *ended* on
// the alternate screen: a TUI that exited and restored the normal screen left
// the terminal showing the normal one, and so does codeshot.
func (e *Emulator) Result() domain.Result {
	return domain.Result{
		Main:    e.main.grid(),
		Alt:     e.alt.grid(),
		UsedAlt: e.cur == e.alt,
		Title:   e.title,
	}
}

func (e *Emulator) step(b byte) {
	switch e.state {
	case stGround:
		e.ground(b)
	default:
		e.escape(b)
	}
}

func (e *Emulator) ground(b byte) {
	if len(e.utf8buf) > 0 || b >= 0x80 {
		e.decode(b)
		return
	}
	switch b {
	case 0x1B:
		e.state = stEsc
	case '\r':
		e.cur.carriageReturn()
	case '\n', 0x0B, 0x0C:
		e.cur.lineFeed()
	case '\b':
		e.cur.backspace()
	case '\t':
		e.cur.tab()
	case 0x07, 0x00:
		// A bell rings nothing in a picture, and NUL is padding.
	default:
		if b >= 0x20 {
			e.print(rune(b))
		}
	}
}

// decode gathers a UTF-8 rune, which may arrive split across two Writes.
func (e *Emulator) decode(b byte) {
	e.utf8buf = append(e.utf8buf, b)
	if !utf8.FullRune(e.utf8buf) {
		if len(e.utf8buf) < utf8.UTFMax {
			return
		}
		e.utf8buf = e.utf8buf[:0]
		e.print(utf8.RuneError)
		return
	}
	r, _ := utf8.DecodeRune(e.utf8buf)
	e.utf8buf = e.utf8buf[:0]
	e.print(r)
}

func (e *Emulator) print(r rune) {
	w := runewidth.RuneWidth(r)
	if w > 0 && !e.autowrap && e.cur.wrapNext {
		// With autowrap off the cursor stays put and overwrites in place.
		e.cur.wrapNext = false
	}
	e.cur.put(r, w, e.style)
}
```

- [ ] **Step 7: Stub the escape branch so the package compiles**

Still in `internal/adapters/vt/vt.go`. Tasks 6 to 8 replace this body; it exists now only so Task 5's tests can run.

```go
// escape is filled in by the CSI, SGR and mode work; until then every escape
// sequence is swallowed whole.
func (e *Emulator) escape(b byte) {
	if b >= 0x40 && b <= 0x7E {
		e.state = stGround
	}
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/adapters/vt/ ./internal/domain/`
Expected: PASS, 8 tests in vt.

- [ ] **Step 9: Commit**

```bash
gofmt -l . && go test ./...
git add go.mod go.sum internal/
git commit -m "Emulate printing, control characters, wrapping and scrollback"
```

---

### Task 6: VT emulator — CSI, cursor movement, erase and scroll

**Files:**
- Create: `internal/adapters/vt/csi.go`
- Modify: `internal/adapters/vt/vt.go` (replace the `escape` stub), `internal/adapters/vt/buffer.go` (add movement and erase)
- Test: `internal/adapters/vt/csi_test.go`

**Interfaces:**
- Consumes: `Emulator`, `buffer`, `parseState` from Task 5.
- Produces: `(*buffer).moveTo(x, y int)`, `(*buffer).move(dx, dy int)`, `(*buffer).eraseInLine(mode int, st domain.Style)`, `(*buffer).eraseInDisplay(mode int, st domain.Style)`, `(*buffer).scrollUp(n int)`, `(*buffer).scrollDown(n int)`; `(*Emulator).csi(final byte)` dispatch.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/vt/csi_test.go`:

```go
package vt

import "testing"

func TestCursorPositionIsOneBased(t *testing.T) {
	// CUP row 2, column 3.
	if got := mainText(t, 8, 3, "\x1b[2;3Hx"); got != "\n  x\n" {
		t.Errorf("got %q, want x at row 2 column 3", got)
	}
}

func TestCursorPositionDefaultsToHome(t *testing.T) {
	if got := mainText(t, 8, 3, "abc\x1b[HX"); got != "Xbc\n" {
		t.Errorf("got %q, want the home position overwritten", got)
	}
}

func TestRelativeCursorMoves(t *testing.T) {
	if got := mainText(t, 10, 4, "abcd\x1b[2D\x1b[1AX"); got != "abXd\n" {
		t.Errorf("left-then-up: got %q, want the c overwritten and the d intact", got)
	}
	if got := mainText(t, 10, 4, "a\x1b[2BX"); got != "a\n\n X\n" {
		t.Errorf("down two: got %q", got)
	}
	if got := mainText(t, 10, 4, "\x1b[4CX"); got != "    X\n" {
		t.Errorf("forward four: got %q", got)
	}
}

func TestCursorMovesClampToTheScreen(t *testing.T) {
	if got := mainText(t, 6, 2, "\x1b[99;99Hx"); got != "\n     x\n" {
		t.Errorf("got %q, want the cursor clamped to the last cell", got)
	}
}

func TestColumnAbsolute(t *testing.T) {
	if got := mainText(t, 8, 2, "abcdef\x1b[3GX"); got != "abXdef\n" {
		t.Errorf("got %q, want column 3 overwritten", got)
	}
}

func TestEraseInLine(t *testing.T) {
	if got := mainText(t, 8, 2, "abcdef\x1b[4G\x1b[K"); got != "abc\n" {
		t.Errorf("EL 0: got %q, want the tail erased", got)
	}
	if got := mainText(t, 8, 2, "abcdef\x1b[4G\x1b[1K"); got != "    ef\n" {
		t.Errorf("EL 1: got %q, want the head erased", got)
	}
	if got := mainText(t, 8, 2, "abcdef\x1b[2K"); got != "" {
		t.Errorf("EL 2: got %q, want the line gone", got)
	}
}

func TestEraseInDisplay(t *testing.T) {
	if got := mainText(t, 6, 3, "aaa\nbbb\x1b[1;2H\x1b[J"); got != "a\n" {
		t.Errorf("ED 0: got %q, want everything from the cursor gone", got)
	}
	if got := mainText(t, 6, 3, "aaa\nbbb\x1b[2J"); got != "" {
		t.Errorf("ED 2: got %q, want a blank screen", got)
	}
}

func TestScrollUpMovesTheScreenAndKeepsTheLine(t *testing.T) {
	// SU scrolls the visible area; on the normal buffer the line that leaves
	// the top is kept, because that is what scrollback is. So the proof that
	// the screen moved is where a write to row 1 now lands: on "b", not "a".
	got := mainText(t, 6, 3, "a\r\nb\r\nc\x1b[S\x1b[1;1HX")
	if got != "a\nX\nc\n" {
		t.Errorf("got %q, want a kept and the b overwritten", got)
	}
}

func TestScrollDownPushesTheBottomLineOff(t *testing.T) {
	// SD has nowhere to keep what falls off the bottom, so "c" is gone.
	got := mainText(t, 6, 3, "a\r\nb\r\nc\x1b[T\x1b[1;1HX")
	if got != "X\na\nb\n" {
		t.Errorf("got %q, want a blank line inserted at the top and c dropped", got)
	}
}

func TestUnknownSequencesAreReportedNotDrawn(t *testing.T) {
	e := New(8, 2)
	var seen []string
	e.Unknown = func(s string) { seen = append(seen, s) }
	e.Write([]byte("a\x1b[5;9Zb"))
	if got := e.Result().Main.TrimTrailingBlank().Text(); got != "ab\n" {
		t.Errorf("got %q, want the sequence swallowed, not printed", got)
	}
	if len(seen) != 1 {
		t.Fatalf("Unknown called %d times, want once: %v", len(seen), seen)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/vt/ -run 'Cursor|Erase|Scroll|Column|Unknown'`
Expected: FAIL — cursor sequences are currently swallowed, so text lands in the wrong cells.

- [ ] **Step 3: Add movement and erase to the buffer**

Append to `internal/adapters/vt/buffer.go`:

```go
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (b *buffer) moveTo(x, y int) {
	b.x = clamp(x, 0, b.cols-1)
	b.y = clamp(y, 0, b.rows-1)
	b.wrapNext = false
}

func (b *buffer) move(dx, dy int) { b.moveTo(b.x+dx, b.y+dy) }

func (b *buffer) eraseInLine(mode int, st domain.Style) {
	line := b.row(b.y)
	from, to := 0, b.cols
	switch mode {
	case 0:
		from = b.x
	case 1:
		to = b.x + 1
	}
	for i := from; i < to && i < len(line); i++ {
		line[i] = blank(st)
	}
	b.wrapNext = false
}

func (b *buffer) eraseInDisplay(mode int, st domain.Style) {
	switch mode {
	case 0:
		b.eraseInLine(0, st)
		for y := b.y + 1; y < b.rows; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
	case 1:
		for y := 0; y < b.y; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
		b.eraseInLine(1, st)
	default:
		for y := 0; y < b.rows; y++ {
			b.lines[b.origin+y] = b.blankLine(st)
		}
	}
	b.wrapNext = false
}

// scrollUp pushes lines off the top of the screen. On the normal buffer they
// land in scrollback, exactly as a line feed at the bottom would leave them.
func (b *buffer) scrollUp(n int) {
	for i := 0; i < n; i++ {
		if b.scrollback {
			b.origin++
			b.lines = append(b.lines, b.blankLine(domain.Style{}))
			continue
		}
		copy(b.lines, b.lines[1:])
		b.lines[b.rows-1] = b.blankLine(domain.Style{})
	}
}

func (b *buffer) scrollDown(n int) {
	for i := 0; i < n; i++ {
		top := b.origin
		copy(b.lines[top+1:top+b.rows], b.lines[top:top+b.rows-1])
		b.lines[top] = b.blankLine(domain.Style{})
	}
}
```

- [ ] **Step 4: Parse CSI sequences**

Create `internal/adapters/vt/csi.go`:

```go
package vt

import "fmt"

// escape drives everything after ESC. The states are deliberately few: a CSI
// with its numeric parameters, an OSC string, and a catch-all that swallows
// two-byte escapes whole.
func (e *Emulator) escape(b byte) {
	switch e.state {
	case stEsc:
		switch b {
		case '[':
			e.state = stCSI
			e.params = e.params[:0]
			e.private = 0
		case ']':
			e.state = stOSC
			e.osc = e.osc[:0]
		default:
			e.report(fmt.Sprintf("ESC %c", b))
			e.state = stGround
		}
	case stCSI:
		e.csiByte(b)
	case stOSC, stOSCEsc:
		e.oscByte(b)
	}
}

func (e *Emulator) csiByte(b byte) {
	switch {
	case b >= '0' && b <= '9':
		if len(e.params) == 0 {
			e.params = append(e.params, 0)
		}
		e.params[len(e.params)-1] = e.params[len(e.params)-1]*10 + int(b-'0')
	case b == ';':
		e.params = append(e.params, 0)
	case b >= '<' && b <= '?':
		e.private = b
	case b >= 0x20 && b <= 0x2F:
		// Intermediate bytes; codeshot needs none of the sequences that use
		// them, but they must not end the sequence either.
	case b >= 0x40 && b <= 0x7E:
		e.csi(b)
		e.state = stGround
	default:
		e.state = stGround
	}
}

// param returns parameter i, or def when it is absent or zero - the
// convention every CSI sequence in this file follows.
func (e *Emulator) param(i, def int) int {
	if i >= len(e.params) || e.params[i] == 0 {
		return def
	}
	return e.params[i]
}

func (e *Emulator) csi(final byte) {
	if e.private != 0 {
		e.mode(final)
		return
	}
	switch final {
	case 'A':
		e.cur.move(0, -e.param(0, 1))
	case 'B':
		e.cur.move(0, e.param(0, 1))
	case 'C':
		e.cur.move(e.param(0, 1), 0)
	case 'D':
		e.cur.move(-e.param(0, 1), 0)
	case 'E':
		e.cur.moveTo(0, e.cur.y+e.param(0, 1))
	case 'F':
		e.cur.moveTo(0, e.cur.y-e.param(0, 1))
	case 'G', '`':
		e.cur.moveTo(e.param(0, 1)-1, e.cur.y)
	case 'H', 'f':
		e.cur.moveTo(e.param(1, 1)-1, e.param(0, 1)-1)
	case 'd':
		e.cur.moveTo(e.cur.x, e.param(0, 1)-1)
	case 'J':
		e.cur.eraseInDisplay(e.param(0, 0), e.style)
	case 'K':
		e.cur.eraseInLine(e.param(0, 0), e.style)
	case 'S':
		e.cur.scrollUp(e.param(0, 1))
	case 'T':
		e.cur.scrollDown(e.param(0, 1))
	case 'm':
		e.sgr()
	default:
		e.report(fmt.Sprintf("CSI %v %c", e.params, final))
	}
}

func (e *Emulator) report(seq string) {
	if e.Unknown != nil {
		e.Unknown(seq)
	}
}
```

- [ ] **Step 5: Remove the stub and add the two placeholders CSI now calls**

Delete the `escape` stub from `internal/adapters/vt/vt.go`. Tasks 7 and 8 fill these in; add them to `internal/adapters/vt/vt.go` now so the package builds:

```go
func (e *Emulator) sgr()          {}
func (e *Emulator) mode(final byte) { e.state = stGround }
func (e *Emulator) oscByte(b byte)  { e.state = stGround }
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/adapters/vt/`
Expected: PASS, all Task 5 and Task 6 tests.

- [ ] **Step 7: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/vt/
git commit -m "Parse CSI: cursor movement, erase and scroll"
```

---

### Task 7: VT emulator — SGR

**Files:**
- Create: `internal/adapters/vt/sgr.go`
- Modify: `internal/adapters/vt/vt.go` (remove the `sgr` placeholder)
- Test: `internal/adapters/vt/sgr_test.go`

**Interfaces:**
- Consumes: `Emulator.params`, `Emulator.style`, `domain.Style`, `domain.Color` constructors.
- Produces: `(*Emulator).sgr()`; unexported `extendedColor(params []int, i int) (domain.Color, int, bool)`.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/vt/sgr_test.go`:

```go
package vt

import (
	"testing"

	"codeshot/internal/domain"
)

// styleAt runs the input and returns the style of the cell at row 0, column 0.
func styleAt(t *testing.T, in string) domain.Style {
	t.Helper()
	e := New(20, 2)
	e.Write([]byte(in))
	return e.Result().Main.Lines[0][0].Style
}

func TestSGRAttributes(t *testing.T) {
	s := styleAt(t, "\x1b[1;3;4;7mx")
	for _, a := range []domain.Attr{domain.AttrBold, domain.AttrItalic, domain.AttrUnderline, domain.AttrInverse} {
		if !s.Has(a) {
			t.Errorf("attribute %b missing from %b", a, s.Attrs)
		}
	}
}

func TestSGRResetClearsEverything(t *testing.T) {
	if s := styleAt(t, "\x1b[1;31m\x1b[0mx"); s != (domain.Style{}) {
		t.Errorf("SGR 0 left %+v, want the zero style", s)
	}
}

func TestSGREmptyParameterIsReset(t *testing.T) {
	if s := styleAt(t, "\x1b[1;31m\x1b[mx"); s != (domain.Style{}) {
		t.Errorf("bare CSI m left %+v, want the zero style", s)
	}
}

func TestSGRBasicAndBrightColours(t *testing.T) {
	if s := styleAt(t, "\x1b[31;44mx"); s.FG != domain.IndexedColor(1) || s.BG != domain.IndexedColor(4) {
		t.Errorf("got %+v, want fg 1 bg 4", s)
	}
	if s := styleAt(t, "\x1b[92;101mx"); s.FG != domain.IndexedColor(10) || s.BG != domain.IndexedColor(9) {
		t.Errorf("got %+v, want bright fg 10 bg 9", s)
	}
	if s := styleAt(t, "\x1b[31;39mx"); s.FG != domain.DefaultColor() {
		t.Errorf("SGR 39 left %+v, want the default foreground", s)
	}
}

func TestSGR256AndTrueColour(t *testing.T) {
	if s := styleAt(t, "\x1b[38;5;208mx"); s.FG != domain.IndexedColor(208) {
		t.Errorf("got %+v, want indexed 208", s)
	}
	if s := styleAt(t, "\x1b[48;2;10;20;30mx"); s.BG != domain.RGBColor(10, 20, 30) {
		t.Errorf("got %+v, want rgb 10,20,30", s)
	}
}

func TestSGRTrueColourDoesNotLeakIntoLaterParameters(t *testing.T) {
	// The 1 after the triple is bold, not a colour index.
	s := styleAt(t, "\x1b[38;2;10;20;30;1mx")
	if s.FG != domain.RGBColor(10, 20, 30) || !s.Has(domain.AttrBold) {
		t.Errorf("got %+v, want rgb plus bold", s)
	}
}

func TestSGRIndividualResets(t *testing.T) {
	if s := styleAt(t, "\x1b[1;4m\x1b[22;24mx"); s.Has(domain.AttrBold) || s.Has(domain.AttrUnderline) {
		t.Errorf("got %b, want bold and underline cleared", s.Attrs)
	}
}

func TestSGRMalformedExtendedColourIsIgnored(t *testing.T) {
	// A truncated 38;5 must not panic and must not eat the next command.
	if s := styleAt(t, "\x1b[38;5m\x1b[1mx"); !s.Has(domain.AttrBold) {
		t.Errorf("got %+v, want the following bold to still apply", s)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/vt/ -run SGR`
Expected: FAIL — the placeholder `sgr` does nothing, so every style is zero.

- [ ] **Step 3: Write the implementation**

Delete the `func (e *Emulator) sgr() {}` placeholder from `vt.go` and create `internal/adapters/vt/sgr.go`:

```go
package vt

import (
	"fmt"

	"codeshot/internal/domain"
)

// sgr applies Select Graphic Rendition parameters to the current style. The
// loop consumes extra parameters itself for 38 and 48, which is the only place
// SGR is not one-parameter-one-effect.
func (e *Emulator) sgr() {
	params := e.params
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			e.style = domain.Style{}
		case p == 1:
			e.style = e.style.Set(domain.AttrBold)
		case p == 2:
			e.style = e.style.Set(domain.AttrDim)
		case p == 3:
			e.style = e.style.Set(domain.AttrItalic)
		case p == 4:
			e.style = e.style.Set(domain.AttrUnderline)
		case p == 7:
			e.style = e.style.Set(domain.AttrInverse)
		case p == 8:
			e.style = e.style.Set(domain.AttrHidden)
		case p == 9:
			e.style = e.style.Set(domain.AttrStrike)
		case p == 22:
			e.style = e.style.Clear(domain.AttrBold | domain.AttrDim)
		case p == 23:
			e.style = e.style.Clear(domain.AttrItalic)
		case p == 24:
			e.style = e.style.Clear(domain.AttrUnderline)
		case p == 27:
			e.style = e.style.Clear(domain.AttrInverse)
		case p == 28:
			e.style = e.style.Clear(domain.AttrHidden)
		case p == 29:
			e.style = e.style.Clear(domain.AttrStrike)
		case p >= 30 && p <= 37:
			e.style.FG = domain.IndexedColor(uint8(p - 30))
		case p == 38:
			if c, next, ok := extendedColor(params, i); ok {
				e.style.FG, i = c, next
			} else {
				return
			}
		case p == 39:
			e.style.FG = domain.DefaultColor()
		case p >= 40 && p <= 47:
			e.style.BG = domain.IndexedColor(uint8(p - 40))
		case p == 48:
			if c, next, ok := extendedColor(params, i); ok {
				e.style.BG, i = c, next
			} else {
				return
			}
		case p == 49:
			e.style.BG = domain.DefaultColor()
		case p >= 90 && p <= 97:
			e.style.FG = domain.IndexedColor(uint8(p - 90 + 8))
		case p >= 100 && p <= 107:
			e.style.BG = domain.IndexedColor(uint8(p - 100 + 8))
		default:
			e.report(fmt.Sprintf("SGR %d", p))
		}
	}
}

// extendedColor reads the 5;n or 2;r;g;b form that follows a 38 or 48. It
// returns the index of the last parameter it consumed, so the caller's loop
// carries on with whatever came after the colour.
func extendedColor(params []int, i int) (domain.Color, int, bool) {
	if i+1 >= len(params) {
		return domain.Color{}, i, false
	}
	switch params[i+1] {
	case 5:
		if i+2 >= len(params) {
			return domain.Color{}, i, false
		}
		return domain.IndexedColor(uint8(params[i+2])), i + 2, true
	case 2:
		if i+4 >= len(params) {
			return domain.Color{}, i, false
		}
		return domain.RGBColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4])), i + 4, true
	}
	return domain.Color{}, i, false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/adapters/vt/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/vt/
git commit -m "Decode SGR into palette-relative styles"
```

---

### Task 8: VT emulator — alt screen, modes, OSC title, and the port adapter

**Files:**
- Create: `internal/adapters/vt/mode.go`, `internal/adapters/vt/adapter.go`
- Modify: `internal/adapters/vt/vt.go` (remove the `mode` and `oscByte` placeholders)
- Test: `internal/adapters/vt/mode_test.go`

**Interfaces:**
- Consumes: `Emulator`, `buffer`, `domain.Result`, `domain.Capture`.
- Produces: `(*Emulator).mode(final byte)`, `(*Emulator).oscByte(b byte)`; `vt.Adapter{Unknown func(string)}` with `Emulate(domain.Capture) (domain.Result, error)` — the concrete type that satisfies the `app.Emulator` port in Task 14.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/vt/mode_test.go`:

```go
package vt

import (
	"strings"
	"testing"

	"codeshot/internal/domain"
)

func TestAlternateScreenIsSeparateFromTheNormalOne(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("normal\x1b[?1049hTUI"))
	r := e.Result()
	if !r.UsedAlt {
		t.Fatal("UsedAlt is false after entering the alternate screen")
	}
	if got := strings.TrimSpace(r.Alt.Text()); got != "TUI" {
		t.Errorf("alt = %q, want TUI", got)
	}
	if !strings.Contains(r.Main.Text(), "normal") {
		t.Errorf("main lost its content: %q", r.Main.Text())
	}
}

func TestLeavingTheAlternateScreenReturnsToNormal(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("normal\x1b[?1049hTUI\x1b[?1049l"))
	r := e.Result()
	if r.UsedAlt {
		t.Error("UsedAlt is true although the program restored the normal screen")
	}
	if !strings.Contains(r.Main.Text(), "normal") {
		t.Errorf("main = %q, want the pre-TUI content back", r.Main.Text())
	}
}

func TestOSCSetsTheTitle(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("\x1b]0;paradajz danas\x07x"))
	if got := e.Result().Title; got != "paradajz danas" {
		t.Errorf("Title = %q", got)
	}
	if got := strings.TrimSpace(e.Result().Main.Text()); got != "x" {
		t.Errorf("main = %q, want the OSC swallowed", got)
	}
}

func TestOSCTerminatedByStringTerminator(t *testing.T) {
	e := New(8, 2)
	e.Write([]byte("\x1b]2;title\x1b\\x"))
	if got := e.Result().Title; got != "title" {
		t.Errorf("Title = %q", got)
	}
}

func TestAutowrapCanBeTurnedOff(t *testing.T) {
	// With DECAWM off the last column is overwritten instead of wrapping.
	if got := mainText(t, 4, 2, "\x1b[?7labcdef"); got != "abcf\n" {
		t.Errorf("got %q, want the tail overwriting the last column", got)
	}
}

func TestUnknownPrivateModesAreReported(t *testing.T) {
	e := New(8, 2)
	var seen []string
	e.Unknown = func(s string) { seen = append(seen, s) }
	e.Write([]byte("\x1b[?2004hx"))
	if len(seen) != 1 {
		t.Errorf("Unknown called %d times, want once for bracketed paste", len(seen))
	}
}

func TestAdapterEmulatesACapture(t *testing.T) {
	var a Adapter
	r, err := a.Emulate(domain.Capture{Cols: 8, Rows: 2, Bytes: []byte("hi")})
	if err != nil {
		t.Fatalf("Emulate: %v", err)
	}
	if got := strings.TrimSpace(r.Main.Text()); got != "hi" {
		t.Errorf("Main = %q", got)
	}
}

func TestAdapterFallsBackToASensibleSizeWhenTheCaptureHasNone(t *testing.T) {
	var a Adapter
	r, err := a.Emulate(domain.Capture{Bytes: []byte("hi")})
	if err != nil {
		t.Fatalf("Emulate: %v", err)
	}
	if r.Main.Cols != 100 {
		t.Errorf("Cols = %d, want the 100-column fallback", r.Main.Cols)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/vt/ -run 'Alternate|OSC|Autowrap|Private|Adapter'`
Expected: FAIL — undefined: `Adapter`; the placeholders ignore modes and OSC.

- [ ] **Step 3: Write the implementation**

Delete the `mode` and `oscByte` placeholders from `vt.go` and create `internal/adapters/vt/mode.go`:

```go
package vt

import (
	"fmt"
	"strings"
)

// mode handles the private sequences - CSI ? n h and CSI ? n l. Only three
// matter to a still picture: the alternate screen, autowrap, and the cursor,
// which codeshot never draws anyway.
func (e *Emulator) mode(final byte) {
	set := final == 'h'
	if final != 'h' && final != 'l' {
		e.report(fmt.Sprintf("CSI ? %v %c", e.params, final))
		return
	}
	for _, p := range e.params {
		switch p {
		case 47, 1047, 1049:
			e.setAlt(set)
		case 7:
			e.autowrap = set
		case 25:
			// Cursor visibility: a shot has no cursor to hide.
		default:
			e.report(fmt.Sprintf("CSI ? %d %c", p, final))
		}
	}
}

// setAlt switches buffers. Entering clears the alternate screen, which is what
// every terminal does and what every TUI assumes.
func (e *Emulator) setAlt(on bool) {
	if on {
		if e.cur == e.alt {
			return
		}
		e.alt = newBuffer(e.cols, e.rows, false)
		e.cur = e.alt
		return
	}
	e.cur = e.main
}

// oscByte gathers an operating system command. Only the title strings - OSC 0,
// 1 and 2 - are kept; the rest, including the shell integration marks a prompt
// may emit, are dropped without ceremony.
func (e *Emulator) oscByte(b byte) {
	switch {
	case e.state == stOSCEsc:
		e.state = stGround
		if b == '\\' {
			e.finishOSC()
		}
	case b == 0x07:
		e.state = stGround
		e.finishOSC()
	case b == 0x1B:
		e.state = stOSCEsc
	default:
		e.osc = append(e.osc, b)
	}
}

func (e *Emulator) finishOSC() {
	s := string(e.osc)
	e.osc = e.osc[:0]
	kind, text, ok := strings.Cut(s, ";")
	if !ok {
		return
	}
	switch kind {
	case "0", "1", "2":
		e.title = text
	default:
		e.report("OSC " + kind)
	}
}
```

Create `internal/adapters/vt/adapter.go`:

```go
package vt

import "codeshot/internal/domain"

// Adapter is the emulator behind the app.Emulator port. It is stateless: a
// Capture goes in, a fresh Emulator runs it, a Result comes out.
type Adapter struct {
	// Unknown, if set, receives every sequence the emulator ignored.
	Unknown func(seq string)
}

// Fallback size for a capture that never learned how wide its terminal was -
// a file on disk, or a pipe with no tty behind it.
const (
	fallbackCols = 100
	fallbackRows = 24
)

func (a Adapter) Emulate(c domain.Capture) (domain.Result, error) {
	cols, rows := c.Cols, c.Rows
	if cols <= 0 {
		cols = fallbackCols
	}
	if rows <= 0 {
		rows = fallbackRows
	}
	e := New(cols, rows)
	e.Unknown = a.Unknown
	if _, err := e.Write(c.Bytes); err != nil {
		return domain.Result{}, err
	}
	return e.Result(), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/adapters/vt/`
Expected: PASS, every vt test.

- [ ] **Step 5: Check the emulator against real output**

```bash
printf 'plain \033[1;32mbold green\033[0m \033[38;5;208m256\033[0m \033[48;2;30;40;50mtruecolour\033[0m\n' > /tmp/codeshot-smoke.ansi
ls --color=always > /tmp/codeshot-ls.ansi 2>/dev/null || ls -G > /tmp/codeshot-ls.ansi
```

Write a scratch test that emulates both files and prints `Main.Text()`; confirm the text matches what the terminal shows and that `Unknown` reports nothing surprising. Delete the scratch test before committing.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/vt/
git commit -m "Handle the alternate screen, modes and OSC titles"
```

---

### Task 9: Themes

**Files:**
- Create: `internal/adapters/theme/theme.go`, `internal/adapters/theme/assets/codeshot-dark.conf`, `internal/adapters/theme/assets/codeshot-light.conf`
- Test: `internal/adapters/theme/theme_test.go`

**Interfaces:**
- Consumes: `domain.Theme`, `domain.RGBA`.
- Produces: `theme.Parse(r io.Reader, name string) (domain.Theme, error)`; `theme.Default() domain.Theme`; `theme.Source{Dirs []string}` with `Theme(name string) (domain.Theme, error)` — the concrete type behind the `app.ThemeSource` port.

Lookup order in `Source.Theme`: an embedded theme with that name, then `<dir>/<name>` for each configured dir, then `name` as a literal path. `Dirs` is empty in this phase; phase 3 fills it with a local Ghostty installation's themes directory.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/theme/theme_test.go`:

```go
package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeshot/internal/domain"
)

const sample = `# a comment
background = #101214
foreground = c8ccd4
cursor-color = #4d9fe8
palette = 0=#1b1e24
palette = 9 = #ff6f78
selection-background = #333333
unknown-key = whatever
`

func TestParseReadsGhosttyThemeFiles(t *testing.T) {
	th, err := Parse(strings.NewReader(sample), "sample")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if th.Name != "sample" {
		t.Errorf("Name = %q", th.Name)
	}
	if (th.Background != domain.RGBA{0x10, 0x12, 0x14, 0xFF}) {
		t.Errorf("Background = %v", th.Background)
	}
	if (th.Foreground != domain.RGBA{0xC8, 0xCC, 0xD4, 0xFF}) {
		t.Errorf("Foreground = %v, want the hash to be optional", th.Foreground)
	}
	if (th.Cursor != domain.RGBA{0x4D, 0x9F, 0xE8, 0xFF}) {
		t.Errorf("Cursor = %v", th.Cursor)
	}
	if (th.Palette[0] != domain.RGBA{0x1B, 0x1E, 0x24, 0xFF}) {
		t.Errorf("Palette[0] = %v", th.Palette[0])
	}
	if (th.Palette[9] != domain.RGBA{0xFF, 0x6F, 0x78, 0xFF}) {
		t.Errorf("Palette[9] = %v, want spaces around the index to be tolerated", th.Palette[9])
	}
}

func TestParseRejectsGarbageColours(t *testing.T) {
	if _, err := Parse(strings.NewReader("background = nonsense"), "x"); err == nil {
		t.Error("Parse accepted a colour that is not a colour")
	}
}

func TestParseIgnoresOutOfRangePaletteIndices(t *testing.T) {
	if _, err := Parse(strings.NewReader("palette = 300=#ffffff"), "x"); err == nil {
		t.Error("Parse accepted palette index 300")
	}
}

func TestDefaultThemeIsComplete(t *testing.T) {
	th := Default()
	if th.Name != "codeshot-dark" {
		t.Errorf("Name = %q, want codeshot-dark", th.Name)
	}
	if th.Background.A != 0xFF || th.Foreground.A != 0xFF {
		t.Error("default theme has transparent background or foreground")
	}
	for i, c := range th.Palette {
		if c.A != 0xFF {
			t.Errorf("palette entry %d is unset", i)
		}
	}
}

func TestSourceFindsEmbeddedThemesByName(t *testing.T) {
	var s Source
	for _, name := range []string{"codeshot-dark", "codeshot-light"} {
		if _, err := s.Theme(name); err != nil {
			t.Errorf("Theme(%q): %v", name, err)
		}
	}
}

func TestSourceFallsBackToAPathOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mine.conf")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	var s Source
	th, err := s.Theme(path)
	if err != nil {
		t.Fatalf("Theme(path): %v", err)
	}
	if th.Name != "mine" {
		t.Errorf("Name = %q, want the file's base name without its extension", th.Name)
	}
}

func TestSourceSearchesConfiguredDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tokyonight"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Source{Dirs: []string{dir}}
	if _, err := s.Theme("tokyonight"); err != nil {
		t.Errorf("Theme: %v", err)
	}
}

func TestSourceReportsAnUnknownTheme(t *testing.T) {
	var s Source
	if _, err := s.Theme("no-such-theme"); err == nil {
		t.Error("Theme accepted a name it cannot resolve")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/theme/`
Expected: FAIL — no such package.

- [ ] **Step 3: Write the theme assets**

Create `internal/adapters/theme/assets/codeshot-dark.conf`:

```
# codeshot's own dark theme. Ghostty's theme file format, so anything you can
# point --theme at will parse the same way.
background = #15181d
foreground = #c3c8d1
cursor-color = #4d9fe8
palette = 0=#1b1e24
palette = 1=#e0555f
palette = 2=#8fc65a
palette = 3=#e2b23c
palette = 4=#4d9fe8
palette = 5=#b07ce8
palette = 6=#3fb8bd
palette = 7=#c3c8d1
palette = 8=#4b5261
palette = 9=#ff6f78
palette = 10=#a7de72
palette = 11=#f5c95a
palette = 12=#6fb6ff
palette = 13=#c99bff
palette = 14=#5fd6da
palette = 15=#eef1f6
```

Create `internal/adapters/theme/assets/codeshot-light.conf`:

```
# codeshot's own light theme.
background = #fbfbfd
foreground = #2b303b
cursor-color = #2b6fd1
palette = 0=#2b303b
palette = 1=#c23a45
palette = 2=#487f2c
palette = 3=#9a6b00
palette = 4=#2b6fd1
palette = 5=#8a4bc0
palette = 6=#0f7d84
palette = 7=#8a9099
palette = 8=#5a6169
palette = 9=#e05561
palette = 10=#5da337
palette = 11=#c08a00
palette = 12=#4d90e8
palette = 13=#a468d8
palette = 14=#1c9aa2
palette = 15=#2b303b
```

- [ ] **Step 4: Write the implementation**

Create `internal/adapters/theme/theme.go`:

```go
// Package theme reads colour schemes in Ghostty's theme file format. codeshot
// embeds two of its own and parses anyone else's, which is how a shot can look
// like the terminal it came from without Ghostty being installed.
package theme

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codeshot/internal/domain"
)

//go:embed assets/*.conf
var assets embed.FS

// Source resolves a theme by name. Dirs are searched after the embedded
// themes and before the name is tried as a path.
type Source struct {
	Dirs []string
}

func (s Source) Theme(name string) (domain.Theme, error) {
	if data, err := assets.ReadFile("assets/" + name + ".conf"); err == nil {
		return Parse(strings.NewReader(string(data)), name)
	}
	for _, dir := range s.Dirs {
		if th, err := parseFile(filepath.Join(dir, name)); err == nil {
			return th, nil
		}
	}
	th, err := parseFile(name)
	if err != nil {
		return domain.Theme{}, fmt.Errorf("theme %q: not embedded, not in any theme directory, and not a readable file", name)
	}
	return th, nil
}

func Default() domain.Theme {
	th, err := Source{}.Theme("codeshot-dark")
	if err != nil {
		// The default theme is embedded in the binary; if it will not parse,
		// the binary is broken and no shot it takes could be trusted.
		panic("codeshot: embedded default theme is unreadable: " + err.Error())
	}
	return th
}

func parseFile(path string) (domain.Theme, error) {
	f, err := os.Open(path)
	if err != nil {
		return domain.Theme{}, err
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Parse(f, name)
}

// Parse reads the handful of keys that describe colour. Anything else in the
// file - and Ghostty configs carry a great deal else - is ignored.
func Parse(r io.Reader, name string) (domain.Theme, error) {
	th := domain.Theme{Name: name}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		var err error
		switch key {
		case "background":
			th.Background, err = parseColor(value)
		case "foreground":
			th.Foreground, err = parseColor(value)
		case "cursor-color":
			th.Cursor, err = parseColor(value)
		case "palette":
			err = parsePalette(&th, value)
		}
		if err != nil {
			return domain.Theme{}, fmt.Errorf("theme %q: %s: %w", name, key, err)
		}
	}
	return th, sc.Err()
}

func parsePalette(th *domain.Theme, value string) error {
	idxText, colorText, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("want index=colour, got %q", value)
	}
	i, err := strconv.Atoi(strings.TrimSpace(idxText))
	if err != nil {
		return fmt.Errorf("index %q: %w", idxText, err)
	}
	if i < 0 || i > 15 {
		return fmt.Errorf("index %d is outside the 16-colour palette", i)
	}
	c, err := parseColor(strings.TrimSpace(colorText))
	if err != nil {
		return err
	}
	th.Palette[i] = c
	return nil
}

func parseColor(s string) (domain.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return domain.RGBA{}, fmt.Errorf("%q is not a six-digit hex colour", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return domain.RGBA{}, fmt.Errorf("%q is not hexadecimal", s)
	}
	return domain.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xFF}, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/adapters/theme/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/theme/
git commit -m "Parse Ghostty theme files and embed two of our own"
```

---

### Task 10: Fonts

**Files:**
- Create: `internal/adapters/fonts/fonts.go`, `internal/adapters/fonts/assets/` (four TTFs, `OFL.txt`, `SOURCES.md`), `docs/adr/0001-prompt-glyph.md`
- Test: `internal/adapters/fonts/fonts_test.go`

**Interfaces:**
- Consumes: `domain.Style`, `domain.AttrBold`, `domain.AttrItalic`.
- Produces: `fonts.Set` with `fonts.Embedded() (*Set, error)`, `(*Set).Face(st domain.Style, sizePx float64) (font.Face, error)`, `(*Set).Metrics(sizePx, lineHeight float64) (Metrics, error)`, `(*Set).CoversRune(r rune) bool`, `(*Set).SyntheticBold(st domain.Style) bool`; `fonts.Metrics{CellW, CellH, Ascent int}`.

- [ ] **Step 1: Vendor JetBrains Mono**

The no-ligature cut is the right one: codeshot draws cell by cell and never shapes, so ligatures could not render even if they were present.

```bash
cd ~/code/codeshot
mkdir -p internal/adapters/fonts/assets
curl -fL -o /tmp/jbm.zip https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip
unzip -l /tmp/jbm.zip | grep -E 'JetBrainsMonoNL-(Regular|Bold|Italic|BoldItalic)\.ttf|OFL.txt'
unzip -o -j /tmp/jbm.zip \
  'fonts/ttf/JetBrainsMonoNL-Regular.ttf' \
  'fonts/ttf/JetBrainsMonoNL-Bold.ttf' \
  'fonts/ttf/JetBrainsMonoNL-Italic.ttf' \
  'fonts/ttf/JetBrainsMonoNL-BoldItalic.ttf' \
  'OFL.txt' -d internal/adapters/fonts/assets/
shasum -a 256 /tmp/jbm.zip
```

Record the URL, the version and the printed checksum in `internal/adapters/fonts/assets/SOURCES.md`, so the next person can verify the bytes rather than trust them. If the release layout differs from the paths above, use whatever `unzip -l` actually shows and note the difference in `SOURCES.md`.

- [ ] **Step 2: Write the failing test**

Create `internal/adapters/fonts/fonts_test.go`:

```go
package fonts

import (
	"testing"

	"codeshot/internal/domain"
)

func TestEmbeddedLoadsAllFourFaces(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
	styles := []domain.Style{
		{},
		{Attrs: domain.AttrBold},
		{Attrs: domain.AttrItalic},
		{Attrs: domain.AttrBold | domain.AttrItalic},
	}
	for _, st := range styles {
		if _, err := s.Face(st, 26); err != nil {
			t.Errorf("Face(%+v): %v", st, err)
		}
		if s.SyntheticBold(st) {
			t.Errorf("style %+v reported synthetic bold, but a real face is embedded", st)
		}
	}
}

func TestMetricsAreSquareIshAndPositive(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Metrics(26, 1.0)
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if m.CellW <= 0 || m.CellH <= 0 || m.Ascent <= 0 {
		t.Fatalf("metrics = %+v, want all positive", m)
	}
	if m.CellH <= m.CellW {
		t.Errorf("metrics = %+v, want a cell taller than it is wide", m)
	}
	if m.Ascent >= m.CellH {
		t.Errorf("ascent %d does not fit in cell height %d", m.Ascent, m.CellH)
	}
}

func TestLineHeightStretchesTheCell(t *testing.T) {
	s, _ := Embedded()
	tight, _ := s.Metrics(26, 1.0)
	loose, _ := s.Metrics(26, 1.5)
	if loose.CellH <= tight.CellH {
		t.Errorf("line height 1.5 gave %d, tighter than 1.0's %d", loose.CellH, tight.CellH)
	}
	if loose.CellW != tight.CellW {
		t.Errorf("line height changed the cell width: %d vs %d", loose.CellW, tight.CellW)
	}
}

func TestCoverage(t *testing.T) {
	s, _ := Embedded()
	for _, r := range []rune{'a', 'Z', '0', 'č', '─', '│', '█'} {
		if !s.CoversRune(r) {
			t.Errorf("embedded font does not cover %q", r)
		}
	}
	// A private-use codepoint the embedded font genuinely lacks, to prove
	// CoversRune can say no. Do NOT use U+E0A0 here: JetBrains Mono NL ships
	// its own small Powerline set, so that codepoint IS covered.
	if s.CoversRune('\uF000') {
		t.Error("CoversRune claims a Nerd Font glyph the embedded font does not have")
	}
}

func TestFaceCacheReturnsTheSameFace(t *testing.T) {
	s, _ := Embedded()
	a, _ := s.Face(domain.Style{}, 26)
	b, _ := s.Face(domain.Style{}, 26)
	if a != b {
		t.Error("Face rebuilt an identical face; the cache is not working")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/adapters/fonts/`
Expected: FAIL — no such package.

- [ ] **Step 4: Write the implementation**

```bash
go get golang.org/x/image@latest
```

Create `internal/adapters/fonts/fonts.go`:

```go
// Package fonts turns the embedded typeface into the faces and cell metrics
// the rasteriser needs. JetBrains Mono is Ghostty's own default, so shipping it
// is what makes a codeshot resemble a Ghostty window for free.
package fonts

import (
	"embed"
	"fmt"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	"codeshot/internal/domain"
)

//go:embed assets/*.ttf
var assets embed.FS

// Metrics is the cell grid's geometry in whole pixels. Rounding the advance to
// an integer is what keeps a hundred columns from drifting half a glyph wide.
type Metrics struct {
	CellW  int
	CellH  int
	Ascent int
}

type faceKey struct {
	variant int
	size    float64
}

// Set is the four faces of one family, plus a cache of the sized faces built
// from them.
type Set struct {
	fonts [4]*sfnt.Font
	faces map[faceKey]font.Face
	buf   sfnt.Buffer
}

const (
	variantRegular = iota
	variantBold
	variantItalic
	variantBoldItalic
)

func Embedded() (*Set, error) {
	s := &Set{faces: map[faceKey]font.Face{}}
	files := [4]string{
		"assets/JetBrainsMonoNL-Regular.ttf",
		"assets/JetBrainsMonoNL-Bold.ttf",
		"assets/JetBrainsMonoNL-Italic.ttf",
		"assets/JetBrainsMonoNL-BoldItalic.ttf",
	}
	for i, name := range files {
		data, err := assets.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("embedded font %s: %w", name, err)
		}
		f, err := opentype.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("embedded font %s: %w", name, err)
		}
		s.fonts[i] = f
	}
	return s, nil
}

func variant(st domain.Style) int {
	switch {
	case st.Has(domain.AttrBold) && st.Has(domain.AttrItalic):
		return variantBoldItalic
	case st.Has(domain.AttrBold):
		return variantBold
	case st.Has(domain.AttrItalic):
		return variantItalic
	default:
		return variantRegular
	}
}

// SyntheticBold reports whether the rasteriser must fake bold by double
// striking. With the embedded family it never has to; a fallback font in a
// later phase may say otherwise.
func (s *Set) SyntheticBold(st domain.Style) bool {
	return st.Has(domain.AttrBold) && s.fonts[variant(st)] == nil
}

// Face returns a cached face. sizePx is already scaled: DPI is fixed at 72 so
// that one point is one pixel and callers do the scaling arithmetic once.
func (s *Set) Face(st domain.Style, sizePx float64) (font.Face, error) {
	key := faceKey{variant(st), sizePx}
	if f, ok := s.faces[key]; ok {
		return f, nil
	}
	src := s.fonts[key.variant]
	if src == nil {
		src = s.fonts[variantRegular]
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	s.faces[key] = f
	return f, nil
}

func (s *Set) Metrics(sizePx, lineHeight float64) (Metrics, error) {
	f, err := s.Face(domain.Style{}, sizePx)
	if err != nil {
		return Metrics{}, err
	}
	adv, ok := f.GlyphAdvance('M')
	if !ok {
		return Metrics{}, fmt.Errorf("font has no advance for M")
	}
	fm := f.Metrics()
	return Metrics{
		CellW:  int(math.Round(float64(adv) / 64)),
		CellH:  int(math.Round(float64(fm.Height) / 64 * lineHeight)),
		Ascent: int(math.Round(float64(fm.Ascent) / 64)),
	}, nil
}

// CoversRune reports whether the regular face has a glyph for r. Everything
// else renders as tofu, and saying so is better than drawing a lie.
func (s *Set) CoversRune(r rune) bool {
	idx, err := s.fonts[variantRegular].GlyphIndex(&s.buf, r)
	return err == nil && idx != 0
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/adapters/fonts/`
Expected: PASS.

- [ ] **Step 6: Settle the prompt glyph question (spec risk 2)**

```bash
cat > /tmp/coverage_test.go <<'EOF'
package fonts

import "testing"

func TestReportPromptGlyphCoverage(t *testing.T) {
	s, _ := Embedded()
	for _, r := range []rune{'❯', '›', '»', '▸', '✓', '✗', '─', '│', '·'} {
		t.Logf("%q covered=%v", r, s.CoversRune(r))
	}
}
EOF
cp /tmp/coverage_test.go internal/adapters/fonts/coverage_test.go
go test ./internal/adapters/fonts/ -run PromptGlyph -v
rm internal/adapters/fonts/coverage_test.go
```

Write `docs/adr/0001-prompt-glyph.md` recording what the run printed and the decision it forces: if `❯` is covered, the default prompt template in Task 14 uses it; if it is not, the default becomes `>` and `❯` waits for the fallback chain in phase 3. Either way the ADR states the observed fact, not a guess.

- [ ] **Step 7: Commit**

```bash
gofmt -l . && go test ./...
git add go.mod go.sum internal/adapters/fonts/ docs/adr/
git commit -m "Embed JetBrains Mono and derive cell metrics from it"
```

---

### Task 11: Rasteriser — cells to pixels

**Files:**
- Create: `internal/adapters/render/raster/render.go`, `internal/adapters/render/raster/text.go`
- Test: `internal/adapters/render/raster/text_test.go`

**Interfaces:**
- Consumes: `domain.Window`, `domain.Frame`, `domain.Grid`, `domain.Theme`, `fonts.Set`, `fonts.Metrics`.
- Produces: `raster.Options{FontSize, LineHeight float64}`; `raster.New(f *fonts.Set, opt Options) Renderer`; `(Renderer).Render(domain.Window) (image.Image, error)` — the concrete type behind the `app.Renderer` port; `raster.DefaultOptions() Options` (`FontSize: 13, LineHeight: 1.0`).

Layout, all at the window's `Scale`: the image is `margin | window | margin`; the window is `titlebar` above `padding | grid | padding`. Task 12 draws the titlebar and rounds the corners, Task 13 adds the shadow; this task draws a square window with a titlebar-shaped gap and fills the grid.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/render/raster/text_test.go`:

```go
package raster

import (
	"image"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

// bare is a window with nothing around the grid, so tests can address cells by
// arithmetic instead of hunting for them.
func bare(g domain.Grid, th domain.Theme) domain.Window {
	return domain.Window{
		Frame: domain.Frame{Grid: g},
		Theme: th,
		Chrome: domain.Chrome{
			Controls: domain.ControlsNone,
			Scale:    1,
		},
	}
}

func testTheme() domain.Theme {
	th := domain.Theme{
		Background: domain.RGBA{0x00, 0x00, 0x00, 0xFF},
		Foreground: domain.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
	}
	th.Palette[1] = domain.RGBA{0xFF, 0x00, 0x00, 0xFF}
	th.Palette[4] = domain.RGBA{0x00, 0x00, 0xFF, 0xFF}
	return th
}

func cells(runes string, st domain.Style) []domain.Cell {
	out := make([]domain.Cell, 0, len(runes))
	for _, r := range runes {
		out = append(out, domain.Cell{Rune: r, Style: st, Width: 1})
	}
	return out
}

func render(t *testing.T, w domain.Window) *image.RGBA {
	t.Helper()
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	img, err := New(set, DefaultOptions()).Render(w)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return img.(*image.RGBA)
}

func TestImageIsSizedFromTheGrid(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	g := domain.Grid{Cols: 10, Lines: [][]domain.Cell{cells("hi", domain.Style{})}}
	img := render(t, bare(g, testTheme()))
	if got, want := img.Bounds().Dx(), 10*m.CellW; got != want {
		t.Errorf("width = %d, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 1*m.CellH; got != want {
		t.Errorf("height = %d, want %d", got, want)
	}
}

func TestBackgroundFillsTheImage(t *testing.T) {
	g := domain.Grid{Cols: 4, Lines: [][]domain.Cell{cells("    ", domain.Style{})}}
	img := render(t, bare(g, testTheme()))
	if r, gg, b, a := img.At(1, 1).RGBA(); r != 0 || gg != 0 || b != 0 || a != 0xFFFF {
		t.Errorf("pixel = %d,%d,%d,%d, want opaque black from the theme", r, gg, b, a)
	}
}

func TestCellBackgroundIsPainted(t *testing.T) {
	set, _ := fonts.Embedded()
	m, _ := set.Metrics(DefaultOptions().FontSize, DefaultOptions().LineHeight)
	st := domain.Style{BG: domain.IndexedColor(4)}
	g := domain.Grid{Cols: 2, Lines: [][]domain.Cell{{
		domain.Cell{Rune: ' ', Width: 1},
		domain.Cell{Rune: ' ', Style: st, Width: 1},
	}}}
	img := render(t, bare(g, testTheme()))
	if r, _, b, _ := img.At(m.CellW+1, 1).RGBA(); r != 0 || b != 0xFFFF {
		t.Errorf("second cell = %d,-,%d, want the palette's blue", r, b)
	}
	if _, _, b, _ := img.At(1, 1).RGBA(); b != 0 {
		t.Error("the blue background bled into the first cell")
	}
}

func TestGlyphsAreDrawn(t *testing.T) {
	blankGrid := domain.Grid{Cols: 3, Lines: [][]domain.Cell{cells("   ", domain.Style{})}}
	textGrid := domain.Grid{Cols: 3, Lines: [][]domain.Cell{cells("WWW", domain.Style{})}}
	blank := countNonBackground(render(t, bare(blankGrid, testTheme())))
	text := countNonBackground(render(t, bare(textGrid, testTheme())))
	if blank != 0 {
		t.Errorf("%d marks on a blank grid, want none", blank)
	}
	if text == 0 {
		t.Error("no marks drawn for WWW")
	}
}

func TestZeroWidthContinuationCellsDrawNoGlyph(t *testing.T) {
	// A wide rune owns two cells; the second must not be drawn over.
	wide := domain.Grid{Cols: 4, Lines: [][]domain.Cell{{
		{Rune: '日', Width: 2},
		{Width: 0},
		{Rune: ' ', Width: 1},
		{Rune: ' ', Width: 1},
	}}}
	if countNonBackground(render(t, bare(wide, testTheme()))) == 0 {
		t.Error("the wide rune was not drawn at all")
	}
}

func TestUnderlineAddsPixelsBelowTheBaseline(t *testing.T) {
	plain := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{})}}
	under := domain.Grid{Cols: 2, Lines: [][]domain.Cell{cells("x ", domain.Style{}.Set(domain.AttrUnderline))}}
	if countNonBackground(render(t, bare(under, testTheme()))) <= countNonBackground(render(t, bare(plain, testTheme()))) {
		t.Error("underline drew no extra pixels")
	}
}

func countNonBackground(img *image.RGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bb, _ := img.At(x, y).RGBA(); r|g|bb != 0 {
				n++
			}
		}
	}
	return n
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/render/raster/`
Expected: FAIL — no such package.

- [ ] **Step 3: Write the layout and orchestration**

Create `internal/adapters/render/raster/render.go`:

```go
// Package raster draws a Window with nothing but golang.org/x/image: no
// browser, no SVG toolchain, no terminal. That is what lets the same bytes
// produce the same PNG on a laptop and in CI.
package raster

import (
	"image"
	"image/color"
	"image/draw"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

type Options struct {
	FontSize   float64
	LineHeight float64
}

func DefaultOptions() Options { return Options{FontSize: 13, LineHeight: 1.0} }

type Renderer struct {
	fonts *fonts.Set
	opt   Options
}

func New(f *fonts.Set, opt Options) Renderer { return Renderer{fonts: f, opt: opt} }

// layout is every measurement the drawing code needs, all in device pixels.
type layout struct {
	scale    int
	metrics  fonts.Metrics
	sizePx   float64
	margin   int
	padding  int
	titlebar int
	window   image.Rectangle // within the whole image
	grid     image.Point     // top-left of the first cell
	cols     int
	rows     int
}

func (r Renderer) layout(w domain.Window) (layout, error) {
	scale := w.Chrome.Scale
	if scale < 1 {
		scale = 1
	}
	l := layout{scale: scale, sizePx: r.opt.FontSize * float64(scale)}
	m, err := r.fonts.Metrics(l.sizePx, r.opt.LineHeight)
	if err != nil {
		return layout{}, err
	}
	l.metrics = m
	l.margin = w.Chrome.Margin * scale
	l.padding = w.Chrome.Padding * scale
	l.titlebar = 0
	if w.Chrome.Controls != domain.ControlsNone || w.Chrome.ShowTitle {
		l.titlebar = w.Chrome.TitlebarHeight * scale
	}
	l.cols = w.Frame.Grid.Cols
	l.rows = w.Frame.Grid.Rows()
	winW := l.cols*m.CellW + 2*l.padding
	winH := l.rows*m.CellH + 2*l.padding + l.titlebar
	l.window = image.Rect(l.margin, l.margin, l.margin+winW, l.margin+winH)
	l.grid = image.Pt(l.window.Min.X+l.padding, l.window.Min.Y+l.titlebar+l.padding)
	return l, nil
}

func (r Renderer) Render(w domain.Window) (image.Image, error) {
	l, err := r.layout(w)
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, l.window.Max.X+l.margin, l.window.Max.Y+l.margin))
	if w.Chrome.Background != nil {
		draw.Draw(img, img.Bounds(), image.NewUniform(rgba(*w.Chrome.Background)), image.Point{}, draw.Src)
	}
	if err := r.drawWindow(img, l, w); err != nil {
		return nil, err
	}
	return img, nil
}

// drawWindow paints the window body and its contents. Task 12 replaces the
// square fill with rounded corners and a titlebar.
func (r Renderer) drawWindow(img *image.RGBA, l layout, w domain.Window) error {
	draw.Draw(img, l.window, image.NewUniform(rgba(w.Theme.Background)), image.Point{}, draw.Src)
	return r.drawCells(img, l, w)
}

func rgba(c domain.RGBA) color.RGBA { return color.RGBA{c.R, c.G, c.B, c.A} }
```

- [ ] **Step 4: Write the text pass**

Create `internal/adapters/render/raster/text.go`:

```go
package raster

import (
	"image"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"codeshot/internal/domain"
)

// drawCells paints backgrounds first, in runs, then glyphs. Merging runs of
// equal background into one rectangle is not an optimisation: filling cell by
// cell leaves visible seams where anti-aliased edges meet.
func (r Renderer) drawCells(img *image.RGBA, l layout, w domain.Window) error {
	for y, line := range w.Frame.Grid.Lines {
		top := l.grid.Y + y*l.metrics.CellH
		r.drawBackgrounds(img, l, w.Theme, line, top)
		if err := r.drawGlyphs(img, l, w.Theme, line, top); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) drawBackgrounds(img *image.RGBA, l layout, th domain.Theme, line []domain.Cell, top int) {
	x := 0
	for x < len(line) {
		_, bg := th.Resolve(line[x].Style)
		run := x + 1
		for run < len(line) {
			if _, next := th.Resolve(line[run].Style); next != bg {
				break
			}
			run++
		}
		if bg != th.Background {
			rect := image.Rect(
				l.grid.X+x*l.metrics.CellW, top,
				l.grid.X+run*l.metrics.CellW, top+l.metrics.CellH,
			)
			draw.Draw(img, rect, image.NewUniform(rgba(bg)), image.Point{}, draw.Src)
		}
		x = run
	}
}

func (r Renderer) drawGlyphs(img *image.RGBA, l layout, th domain.Theme, line []domain.Cell, top int) error {
	baseline := top + l.metrics.Ascent
	for x, cell := range line {
		if cell.Width == 0 || ((cell.Rune == 0 || cell.Rune == ' ') && cell.Combining == "") {
			// The trailing half of a wide rune draws no glyph, but it must
			// still draw its own decorations: a rule one cell wide under a
			// two-cell character stops halfway across it. The emulator copies
			// the leading cell's style onto the continuation cell precisely so
			// this works.
			r.drawDecorations(img, l, th, cell, x, top, baseline)
			continue
		}
		face, err := r.fonts.Face(cell.Style, l.sizePx)
		if err != nil {
			return err
		}
		fg, _ := th.Resolve(cell.Style)
		d := font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(rgba(fg)),
			Face: face,
			Dot:  fixed.P(l.grid.X+x*l.metrics.CellW, baseline),
		}
		start := d.Dot
		d.DrawString(string(cell.Rune) + cell.Combining)
		if r.fonts.SyntheticBold(cell.Style) {
			d.Dot = start
			d.Dot.X += fixed.I(l.scale)
			d.DrawString(string(cell.Rune) + cell.Combining)
		}
		r.drawDecorations(img, l, th, cell, x, top, baseline)
	}
	return nil
}

// drawDecorations adds the rules SGR asks for but a font cannot supply.
func (r Renderer) drawDecorations(img *image.RGBA, l layout, th domain.Theme, cell domain.Cell, x, top, baseline int) {
	if !cell.Style.Has(domain.AttrUnderline) && !cell.Style.Has(domain.AttrStrike) {
		return
	}
	fg, _ := th.Resolve(cell.Style)
	thickness := l.scale
	left := l.grid.X + x*l.metrics.CellW
	right := left + l.metrics.CellW
	if cell.Style.Has(domain.AttrUnderline) {
		y := baseline + 2*l.scale
		draw.Draw(img, image.Rect(left, y, right, y+thickness), image.NewUniform(rgba(fg)), image.Point{}, draw.Src)
	}
	if cell.Style.Has(domain.AttrStrike) {
		y := baseline - l.metrics.Ascent/3
		draw.Draw(img, image.Rect(left, y, right, y+thickness), image.NewUniform(rgba(fg)), image.Point{}, draw.Src)
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/adapters/render/raster/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/render/
git commit -m "Rasterise the cell grid"
```

---

### Task 12: Rasteriser — window chrome

**Files:**
- Create: `internal/adapters/render/raster/chrome.go`
- Modify: `internal/adapters/render/raster/render.go` (`drawWindow` calls the chrome)
- Test: `internal/adapters/render/raster/chrome_test.go`

**Interfaces:**
- Consumes: `layout`, `rgba`, `domain.Chrome`, `domain.Controls`, `fonts.Set`.
- Produces: `fillRoundRect(dst *image.RGBA, r image.Rectangle, radius float64, c color.RGBA)`, `fillCircle(dst *image.RGBA, cx, cy, radius float64, c color.RGBA)`, `(Renderer).drawChrome(img *image.RGBA, l layout, w domain.Window) error`.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/render/raster/chrome_test.go`:

```go
package raster

import (
	"image"
	"testing"

	"codeshot/internal/domain"
)

func chromed(g domain.Grid, th domain.Theme, mut func(*domain.Chrome)) domain.Window {
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Shadow = false
	c.Margin = 0
	c.Title = "paradajz danas"
	if mut != nil {
		mut(&c)
	}
	return domain.Window{Frame: domain.Frame{Grid: g}, Theme: th, Chrome: c}
}

func wideGrid() domain.Grid {
	return domain.Grid{Cols: 30, Lines: [][]domain.Cell{cells("hello", domain.Style{})}}
}

func TestCornersAreTransparent(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("top-left corner alpha = %d, want 0: the window is rounded", a)
	}
	b := img.Bounds()
	if _, _, _, a := img.At(b.Max.X-1, b.Max.Y-1).RGBA(); a != 0 {
		t.Error("bottom-right corner is opaque; all four corners are rounded")
	}
}

func TestWindowCentreIsOpaque(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	b := img.Bounds()
	if _, _, _, a := img.At(b.Dx()/2, b.Dy()/2).RGBA(); a != 0xFFFF {
		t.Error("the middle of the window is not opaque")
	}
}

func TestTrafficLightsArePaintedInTheTitlebar(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), nil))
	// The close button's centre: x=20, y=titlebar/2, at scale 1.
	r, g, b, _ := img.At(20, 14).RGBA()
	if !(r > 0xC000 && g < 0x9000 && b < 0x9000) {
		t.Errorf("close button = %d,%d,%d, want the red traffic light", r, g, b)
	}
	r, g, b, _ = img.At(60, 14).RGBA()
	if !(g > 0x9000 && r < 0x9000) {
		t.Errorf("zoom button = %d,%d,%d, want the green traffic light", r, g, b)
	}
}

func TestControlsNoneDrawsNoButtons(t *testing.T) {
	img := render(t, chromed(wideGrid(), testTheme(), func(c *domain.Chrome) {
		c.Controls = domain.ControlsNone
		c.ShowTitle = false
	}))
	if r, g, b, _ := img.At(20, 14).RGBA(); r > 0xC000 && g < 0x9000 && b < 0x9000 {
		t.Error("a red traffic light was drawn although controls are off")
	}
}

func TestTitleIsDrawnAndCentred(t *testing.T) {
	with := render(t, chromed(wideGrid(), testTheme(), nil))
	without := render(t, chromed(wideGrid(), testTheme(), func(c *domain.Chrome) { c.Title = "" }))
	if titlebarMarks(with) <= titlebarMarks(without) {
		t.Error("the title drew no pixels")
	}
}

// titlebarMarks counts pixels in the middle third of the titlebar, away from
// the traffic lights on the left.
func titlebarMarks(img *image.RGBA) int {
	b := img.Bounds()
	n := 0
	for y := 0; y < 28; y++ {
		for x := b.Dx() / 3; x < 2*b.Dx()/3; x++ {
			if r, g, bb, _ := img.At(x, y).RGBA(); r|g|bb != 0 {
				n++
			}
		}
	}
	return n
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/render/raster/ -run 'Corner|Traffic|Title|Controls|Centre'`
Expected: FAIL — the window is still a square with no titlebar.

- [ ] **Step 3: Write the chrome**

Create `internal/adapters/render/raster/chrome.go`:

```go
package raster

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"codeshot/internal/domain"
)

// The macOS traffic lights, each with the slightly darker ring the real ones
// have. Without the ring they read as flat stickers.
var (
	closeFill = color.RGBA{0xFF, 0x5F, 0x57, 0xFF}
	closeRing = color.RGBA{0xE0, 0x44, 0x3E, 0xFF}
	minFill   = color.RGBA{0xFE, 0xBC, 0x2E, 0xFF}
	minRing   = color.RGBA{0xDE, 0xA1, 0x23, 0xFF}
	zoomFill  = color.RGBA{0x28, 0xC8, 0x40, 0xFF}
	zoomRing  = color.RGBA{0x1A, 0xAB, 0x29, 0xFF}
	linuxDot  = color.RGBA{0x6B, 0x70, 0x7B, 0xFF}
)

// drawChrome paints the window body - rounded, in the terminal's own
// background colour, titlebar included, which is what Ghostty's transparent
// titlebar style looks like - and then its buttons and title.
func (r Renderer) drawChrome(img *image.RGBA, l layout, w domain.Window) error {
	fillRoundRect(img, l.window, float64(w.Chrome.Radius*l.scale), rgba(w.Theme.Background))
	if l.titlebar == 0 {
		return nil
	}
	mid := float64(l.window.Min.Y + l.titlebar/2)
	radius := 6 * float64(l.scale)
	switch w.Chrome.Controls {
	case domain.ControlsMacOS:
		for i, pair := range [3][2]color.RGBA{{closeFill, closeRing}, {minFill, minRing}, {zoomFill, zoomRing}} {
			cx := float64(l.window.Min.X + (20+20*i)*l.scale)
			fillCircle(img, cx, mid, radius, pair[1])
			fillCircle(img, cx, mid, radius-float64(l.scale)*0.75, pair[0])
		}
	case domain.ControlsLinux:
		for i := 0; i < 3; i++ {
			cx := float64(l.window.Max.X - (20+20*(2-i))*l.scale)
			fillCircle(img, cx, mid, radius, linuxDot)
		}
	}
	return r.drawTitle(img, l, w)
}

func (r Renderer) drawTitle(img *image.RGBA, l layout, w domain.Window) error {
	if !w.Chrome.ShowTitle || w.Chrome.Title == "" {
		return nil
	}
	face, err := r.fonts.Face(domain.Style{}, 11*float64(l.scale))
	if err != nil {
		return err
	}
	width := font.MeasureString(face, w.Chrome.Title)
	x := l.window.Min.X + (l.window.Dx()-width.Round())/2
	// A title at full foreground strength shouts over the output it labels.
	fg, bg := w.Theme.Foreground, w.Theme.Background
	dim := color.RGBA{
		uint8((int(fg.R) + int(bg.R)) / 2),
		uint8((int(fg.G) + int(bg.G)) / 2),
		uint8((int(fg.B) + int(bg.B)) / 2),
		0xFF,
	}
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(dim),
		Face: face,
		Dot:  fixed.P(x, l.window.Min.Y+l.titlebar/2+4*l.scale),
	}
	d.DrawString(w.Chrome.Title)
	return nil
}

// fillRoundRect fills r with c, rounding every corner. Anti-aliasing comes
// from x/image's rasteriser, so the edges hold up when the shot is scaled.
func fillRoundRect(dst *image.RGBA, full image.Rectangle, radius float64, c color.RGBA) {
	// x/image's rasteriser does not clip: it indexes dst.Pix from r.Min
	// directly, so a rectangle poking outside the image panics or wraps into
	// the wrong rows. Clipping here is what makes every caller safe.
	r := full.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	w, h := float64(r.Dx()), float64(r.Dy())
	radius = math.Min(radius, math.Min(w, h)/2)
	ra := vector.NewRasterizer(r.Dx(), r.Dy())
	ra.MoveTo(float32(radius), 0)
	ra.LineTo(float32(w-radius), 0)
	ra.QuadTo(float32(w), 0, float32(w), float32(radius))
	ra.LineTo(float32(w), float32(h-radius))
	ra.QuadTo(float32(w), float32(h), float32(w-radius), float32(h))
	ra.LineTo(float32(radius), float32(h))
	ra.QuadTo(0, float32(h), 0, float32(h-radius))
	ra.LineTo(0, float32(radius))
	ra.QuadTo(0, 0, float32(radius), 0)
	ra.ClosePath()
	ra.Draw(dst, r, image.NewUniform(c), image.Point{})
}

// kappa is the control-point distance that turns four cubic segments into a
// circle no eye can tell from the real thing. Four quadratics give a diamond.
const kappa = 0.5522847498

func fillCircle(dst *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	full := image.Rect(int(cx-radius)-2, int(cy-radius)-2, int(cx+radius)+2, int(cy+radius)+2)
	// Same clipping rule as fillRoundRect, and it bites here first: the
	// button offsets are fixed at 20/40/60, so a narrow window puts a circle
	// clean outside the image.
	r := full.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	ra := vector.NewRasterizer(r.Dx(), r.Dy())
	ox, oy := cx-float64(r.Min.X), cy-float64(r.Min.Y)
	k := radius * kappa
	ra.MoveTo(float32(ox+radius), float32(oy))
	ra.CubeTo(float32(ox+radius), float32(oy+k), float32(ox+k), float32(oy+radius), float32(ox), float32(oy+radius))
	ra.CubeTo(float32(ox-k), float32(oy+radius), float32(ox-radius), float32(oy+k), float32(ox-radius), float32(oy))
	ra.CubeTo(float32(ox-radius), float32(oy-k), float32(ox-k), float32(oy-radius), float32(ox), float32(oy-radius))
	ra.CubeTo(float32(ox+k), float32(oy-radius), float32(ox+radius), float32(oy-k), float32(ox+radius), float32(oy))
	ra.ClosePath()
	ra.Draw(dst, r, image.NewUniform(c), image.Point{})
}
```

- [ ] **Step 4: Call the chrome from the renderer**

In `internal/adapters/render/raster/render.go`, replace the body of `drawWindow`:

```go
func (r Renderer) drawWindow(img *image.RGBA, l layout, w domain.Window) error {
	if err := r.drawChrome(img, l, w); err != nil {
		return err
	}
	return r.drawCells(img, l, w)
}
```

The now-unused `draw` import in `render.go` stays: `Render` still uses it for the margin background.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/adapters/render/raster/`
Expected: PASS, including Task 11's tests — `bare()` builds its `Chrome` literally, so `Radius` is zero there and the window stays square for the tests that address pixels by arithmetic.

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/render/raster/
git commit -m "Draw the window: rounded corners, titlebar, traffic lights"
```

---

### Task 13: Rasteriser — shadow, and golden tests

**Files:**
- Create: `internal/adapters/render/raster/shadow.go`, `internal/adapters/render/raster/golden_test.go`, `internal/adapters/render/raster/testdata/`
- Modify: `internal/adapters/render/raster/render.go` (draw the shadow before the window)
- Test: as above

**Interfaces:**
- Consumes: `layout`, `fillRoundRect`.
- Produces: `drawShadow(dst *image.RGBA, window image.Rectangle, radius float64, blur, offsetY int, alpha float64)`.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/render/raster/golden_test.go`:

```go
package raster

import (
	"bytes"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"codeshot/internal/adapters/fonts"
	"codeshot/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the golden PNGs")

func TestShadowDarkensOutsideTheWindow(t *testing.T) {
	g := domain.Grid{Cols: 20, Lines: [][]domain.Cell{cells("shadow", domain.Style{})}}
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Margin = 40
	c.Title = "x"
	withShadow := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})
	c.Shadow = false
	without := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})

	// A point just below the window, inside the margin.
	x, y := withShadow.Bounds().Dx()/2, withShadow.Bounds().Dy()-20
	_, _, _, a1 := withShadow.At(x, y).RGBA()
	_, _, _, a2 := without.At(x, y).RGBA()
	if a1 == 0 {
		t.Error("no shadow below the window")
	}
	if a2 != 0 {
		t.Error("something is painted in the margin with the shadow off")
	}
	if a1 == 0xFFFF {
		t.Error("the shadow is fully opaque; it must fade")
	}
}

func TestShadowLeavesTheImageEdgeClear(t *testing.T) {
	g := domain.Grid{Cols: 20, Lines: [][]domain.Cell{cells("shadow", domain.Style{})}}
	c := domain.DefaultChrome()
	c.Scale = 1
	img := render(t, domain.Window{Frame: domain.Frame{Grid: g}, Theme: testTheme(), Chrome: c})
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("image corner alpha = %d, want the 64px margin to contain the blur", a)
	}
}

func TestGoldenShots(t *testing.T) {
	set, err := fonts.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]domain.Window{
		"plain":  goldenWindow(false),
		"styled": goldenWindow(true),
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			img, err := New(set, DefaultOptions()).Render(w)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", name+".png")
			if *update {
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run: go test ./internal/adapters/render/raster/ -update)", err)
			}
			if !bytes.Equal(want, buf.Bytes()) {
				got := filepath.Join(t.TempDir(), name+".got.png")
				os.WriteFile(got, buf.Bytes(), 0o644)
				t.Errorf("rendering changed; compare %s with %s", path, got)
			}
		})
	}
}

func goldenWindow(styled bool) domain.Window {
	th := testTheme()
	th.Background = domain.RGBA{0x15, 0x18, 0x1D, 0xFF}
	th.Foreground = domain.RGBA{0xC3, 0xC8, 0xD1, 0xFF}
	line := cells("$ codeshot render session.ansi", domain.Style{})
	if styled {
		st := domain.Style{FG: domain.IndexedColor(4)}.Set(domain.AttrBold)
		line = cells("$ codeshot render session.ansi", st)
	}
	c := domain.DefaultChrome()
	c.Scale = 1
	c.Title = "codeshot"
	return domain.Window{
		Frame: domain.Frame{Grid: domain.Grid{Cols: 34, Lines: [][]domain.Cell{line, cells("done.", domain.Style{})}}},
		Theme: th,
		Chrome: c,
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/adapters/render/raster/ -run Shadow`
Expected: FAIL — nothing is drawn in the margin.

- [ ] **Step 3: Write the shadow**

Create `internal/adapters/render/raster/shadow.go`:

```go
package raster

import (
	"image"
	"image/color"
)

// drawShadow paints a soft dark shape behind the window. It builds the
// window's silhouette as an alpha mask, blurs it with three box passes - which
// is close enough to a gaussian that no eye will argue - and composites black
// through the result.
func drawShadow(dst *image.RGBA, window image.Rectangle, radius float64, blur, offsetY int, alpha float64) {
	if blur < 1 {
		blur = 1
	}
	bounds := dst.Bounds()
	mask := image.NewAlpha(bounds)
	silhouette := image.NewRGBA(bounds)
	fillRoundRect(silhouette, window.Add(image.Pt(0, offsetY)), radius, color.RGBA{0, 0, 0, 0xFF})
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			mask.SetAlpha(x, y, color.Alpha{A: silhouette.RGBAAt(x, y).A})
		}
	}
	radiusPx := blur / 3
	for i := 0; i < 3; i++ {
		mask = boxBlur(mask, radiusPx)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			a := float64(mask.AlphaAt(x, y).A) * alpha
			if a < 1 {
				continue
			}
			dst.SetRGBA(x, y, blendOver(dst.RGBAAt(x, y), color.RGBA{0, 0, 0, uint8(a)}))
		}
	}
}

// boxBlur runs one separable pass. Two nested loops each way beat one clever
// loop nobody can read.
func boxBlur(src *image.Alpha, radius int) *image.Alpha {
	if radius < 1 {
		return src
	}
	b := src.Bounds()
	tmp := image.NewAlpha(b)
	out := image.NewAlpha(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum, n := 0, 0
			for dx := -radius; dx <= radius; dx++ {
				if x+dx < b.Min.X || x+dx >= b.Max.X {
					continue
				}
				sum += int(src.AlphaAt(x+dx, y).A)
				n++
			}
			tmp.SetAlpha(x, y, color.Alpha{A: uint8(sum / n)})
		}
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum, n := 0, 0
			for dy := -radius; dy <= radius; dy++ {
				if y+dy < b.Min.Y || y+dy >= b.Max.Y {
					continue
				}
				sum += int(tmp.AlphaAt(x, y+dy).A)
				n++
			}
			out.SetAlpha(x, y, color.Alpha{A: uint8(sum / n)})
		}
	}
	return out
}

// blendOver is source-over compositing for a straight-alpha RGBA image.
func blendOver(dst, src color.RGBA) color.RGBA {
	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA == 0 {
		return color.RGBA{}
	}
	mix := func(s, d uint8) uint8 {
		return uint8((float64(s)*sa + float64(d)*da*(1-sa)) / outA)
	}
	return color.RGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), uint8(outA * 255)}
}
```

- [ ] **Step 4: Draw the shadow before the window**

In `internal/adapters/render/raster/render.go`, insert into `Render` between the background fill and `drawWindow`:

```go
	if w.Chrome.Shadow {
		drawShadow(img, l.window, float64(w.Chrome.Radius*l.scale), 40*l.scale, 18*l.scale, 0.35)
	}
```

- [ ] **Step 5: Run the shadow tests to verify they pass**

Run: `go test ./internal/adapters/render/raster/ -run Shadow`
Expected: PASS.

- [ ] **Step 6: Create the goldens and confirm they are stable**

```bash
mkdir -p internal/adapters/render/raster/testdata
go test ./internal/adapters/render/raster/ -update
go test ./internal/adapters/render/raster/        # must pass twice in a row
go test ./internal/adapters/render/raster/
```

- [ ] **Step 7: Check determinism on the other platform (spec risk 1)**

If a Linux machine or container is available:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.27 go test ./internal/adapters/render/raster/
```

If the goldens differ across platforms, record the finding in `docs/adr/0002-golden-determinism.md` and replace the byte-exact comparison with a per-pixel tolerance of 1 in each channel — but only after confirming a real difference. Do not pre-emptively weaken the test.

- [ ] **Step 8: Commit**

```bash
gofmt -l . && go test ./...
git add internal/adapters/render/raster/ docs/adr/
git commit -m "Add the window shadow and golden-image tests"
```

---

### Task 14: Ports, the prompt, the file source, and the use case

**Files:**
- Create: `internal/domain/name.go`, `internal/adapters/prompt/prompt.go`, `internal/adapters/capture/file/file.go`, `internal/app/ports.go`, `internal/app/capture.go`
- Test: `internal/domain/name_test.go`, `internal/adapters/prompt/prompt_test.go`, `internal/app/capture_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1-13.
- Produces:
  - `domain.Slug(command string) string`, `domain.ResolveName(raw, command string, exists func(string) bool) string`
  - `prompt.Template{Text string}` with `Header(domain.Capture) []byte`, and `prompt.Default` — the concrete type behind the `app.PromptSource` port
  - `file.Source{Path, Command, Cwd string; Cols, Rows int}` with `Capture() (domain.Capture, error)` — behind `app.CaptureSource`
  - `app.CaptureSource`, `app.Emulator`, `app.PromptSource`, `app.ThemeSource`, `app.Renderer`, `app.Gallery`, `app.Reporter` interfaces
  - `app.Service` and `app.Request` with `(Service).Run(Request) (string, error)`

Header bytes go through the same emulator as the output, so the prompt template must end its lines with `\r\n`: on the master side of a pty a bare line feed does not return to column one, and Task 5 pinned that.

- [ ] **Step 1: Write the failing tests**

Create `internal/domain/name_test.go`:

```go
package domain

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"paradajz danas":            "paradajz-danas",
		"git status --short":        "git-status-short",
		"  ls   -la  ":              "ls-la",
		"cat /etc/hosts":            "cat-etc-hosts",
		"echo 'hi there!'":          "echo-hi-there",
		"":                          "codeshot",
		"!!!":                       "codeshot",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugIsBounded(t *testing.T) {
	long := ""
	for i := 0; i < 50; i++ {
		long += "word "
	}
	if got := Slug(long); len(got) > 60 {
		t.Errorf("Slug produced %d characters, want at most 60", len(got))
	}
}

func TestResolveNamePrefersWhatWasAskedFor(t *testing.T) {
	none := func(string) bool { return false }
	if got := ResolveName("shot.png", "ls", none); got != "shot.png" {
		t.Errorf("got %q", got)
	}
	if got := ResolveName("shot", "ls", none); got != "shot.png" {
		t.Errorf("got %q, want the extension added", got)
	}
	if got := ResolveName("", "paradajz danas", none); got != "paradajz-danas.png" {
		t.Errorf("got %q", got)
	}
}

func TestResolveNameAvoidsCollisions(t *testing.T) {
	taken := map[string]bool{"ls.png": true, "ls-2.png": true}
	got := ResolveName("", "ls", func(n string) bool { return taken[n] })
	if got != "ls-3.png" {
		t.Errorf("got %q, want ls-3.png", got)
	}
}

func TestResolveNameDoesNotRenameAnExplicitChoice(t *testing.T) {
	// Asking for a name is asking for that name; overwriting is the CLI's
	// decision to make with --force, not the domain's.
	got := ResolveName("shot.png", "ls", func(string) bool { return true })
	if got != "shot.png" {
		t.Errorf("got %q, want the explicit name untouched", got)
	}
}
```

Create `internal/adapters/prompt/prompt_test.go`:

```go
package prompt

import (
	"strings"
	"testing"

	"codeshot/internal/domain"
)

func TestHeaderExpandsCwdAndCommand(t *testing.T) {
	tpl := Template{Text: "{cwd}\r\n> "}
	got := string(tpl.Header(domain.Capture{Cwd: "/tmp/x", Command: "ls -la"}))
	if got != "/tmp/x\r\n> ls -la\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestHeaderShortensTheHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "/Users/alen")
	tpl := Template{Text: "{cwd}\r\n> "}
	got := string(tpl.Header(domain.Capture{Cwd: "/Users/alen", Command: "ls"}))
	if !strings.HasPrefix(got, "~\r\n") {
		t.Errorf("got %q, want the home directory as ~", got)
	}
	got = string(tpl.Header(domain.Capture{Cwd: "/Users/alen/code", Command: "ls"}))
	if !strings.HasPrefix(got, "~/code\r\n") {
		t.Errorf("got %q, want a path below home shortened", got)
	}
}

func TestDefaultTemplateHasTwoLinesAndColour(t *testing.T) {
	got := string(Template{Text: Default}.Header(domain.Capture{Cwd: "/tmp", Command: "ls"}))
	if strings.Count(got, "\r\n") != 2 {
		t.Errorf("got %q, want a cwd line and a command line", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Error("the default prompt has no colour in it")
	}
}

func TestHeaderWithoutACommandIsJustThePrompt(t *testing.T) {
	got := string(Template{Text: "> "}.Header(domain.Capture{}))
	if got != "> \r\n" {
		t.Errorf("got %q", got)
	}
}
```

Create `internal/app/capture_test.go`:

```go
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

func (e *stubEmulator) Emulate(c domain.Capture) (domain.Result, error) {
	e.seen = append(e.seen, string(c.Bytes))
	g := domain.Grid{Cols: 10}
	for _, line := range strings.Split(strings.TrimSuffix(string(c.Bytes), "\r\n"), "\r\n") {
		row := make([]domain.Cell, 0, len(line))
		for _, r := range line {
			row = append(row, domain.Cell{Rune: r, Width: 1})
		}
		g.Lines = append(g.Lines, row)
	}
	return domain.Result{Main: g}, nil
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/domain/ ./internal/adapters/prompt/ ./internal/app/`
Expected: FAIL — undefined: `Slug`, `ResolveName`, package `prompt`, package `app`.

- [ ] **Step 3: Write the naming rules**

Create `internal/domain/name.go`:

```go
package domain

import (
	"fmt"
	"strings"
	"unicode"
)

// slugMax keeps a generated filename readable. A command long enough to hit it
// is one nobody wants to see spelled out in a directory listing.
const slugMax = 60

// Slug turns a command line into a filename stem: lowercase words joined by
// hyphens, with punctuation dropped rather than escaped.
func Slug(command string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(command) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > slugMax {
		s = strings.Trim(s[:slugMax], "-")
	}
	if s == "" {
		return "codeshot"
	}
	return s
}

// ResolveName decides what the file is called. An explicit name is honoured
// exactly, collision or not - overwriting is the caller's decision, made with
// --force. A generated name steps aside for whatever is already there.
func ResolveName(raw, command string, exists func(string) bool) string {
	if raw != "" {
		return withPNG(raw)
	}
	stem := Slug(command)
	name := stem + ".png"
	for n := 2; exists(name); n++ {
		name = fmt.Sprintf("%s-%d.png", stem, n)
	}
	return name
}

func withPNG(name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	return name + ".png"
}
```

- [ ] **Step 4: Write the prompt**

Create `internal/adapters/prompt/prompt.go`:

```go
// Package prompt synthesises the two lines above the output. In wrapper mode
// the real prompt was drawn by the parent shell before codeshot existed, so
// the one in the picture is codeshot's own - close to what was on screen, and
// honest about being a stand-in.
package prompt

import (
	"os"
	"strings"

	"codeshot/internal/domain"
)

// Default is a cwd line and a chevron, in the shape most prompts take. The
// glyph choice is recorded in docs/adr/0001-prompt-glyph.md; use whatever that
// ADR settled on.
const Default = "\x1b[36m{cwd}\x1b[0m\r\n\x1b[92m❯\x1b[0m "

type Template struct {
	Text string
}

// Header returns the bytes that go through the emulator ahead of the output.
// They end in CRLF because the emulator reads what a pty master would emit,
// where a bare line feed does not return to column one.
func (t Template) Header(c domain.Capture) []byte {
	text := t.Text
	if text == "" {
		text = Default
	}
	text = strings.ReplaceAll(text, "{cwd}", tildify(c.Cwd))
	return []byte(text + c.Command + "\r\n")
}

func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
```

- [ ] **Step 5: Write the file capture source**

Create `internal/adapters/capture/file/file.go`:

```go
// Package file reads a saved ANSI dump. It is the simplest CaptureSource there
// is, and the one that lets the whole pipeline be exercised without a terminal.
package file

import (
	"fmt"
	"os"

	"codeshot/internal/domain"
)

type Source struct {
	Path    string
	Command string
	Cwd     string
	Cols    int
	Rows    int
}

func (s Source) Capture() (domain.Capture, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return domain.Capture{}, fmt.Errorf("reading %s: %w", s.Path, err)
	}
	cwd := s.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return domain.Capture{
		Command: s.Command,
		Cwd:     cwd,
		Cols:    s.Cols,
		Rows:    s.Rows,
		Bytes:   data,
	}, nil
}
```

- [ ] **Step 6: Write the ports and the use case**

Create `internal/app/ports.go`:

```go
// Package app holds codeshot's one use case and the ports it drives. Nothing
// here knows what a pty, a font file or a PNG encoder is; that is the point.
package app

import (
	"image"

	"codeshot/internal/domain"
)

// CaptureSource yields everything one command produced, however it got hold
// of it: a pty it owns, a pipe, or a file on disk.
type CaptureSource interface {
	Capture() (domain.Capture, error)
}

// Emulator turns raw bytes into grids of styled cells.
type Emulator interface {
	Emulate(domain.Capture) (domain.Result, error)
}

// PromptSource supplies the bytes of the prompt and command lines that sit
// above the output. They are emulated like any other bytes.
type PromptSource interface {
	Header(domain.Capture) []byte
}

type ThemeSource interface {
	Theme(name string) (domain.Theme, error)
}

type Renderer interface {
	Render(domain.Window) (image.Image, error)
}

// Gallery is where shots are kept. Exists lets the naming rules step around a
// file rather than over it.
type Gallery interface {
	Exists(name string) bool
	Store(name string, img image.Image) (path string, err error)
}

type Reporter interface {
	Stored(path string)
	Warn(msg string)
}
```

Create `internal/app/capture.go`:

```go
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
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -l . && go test ./...
git add internal/
git commit -m "Wire the ports, the prompt and the capture use case"
```

---

### Task 15: The gallery, the CLI, and a working binary

**Files:**
- Create: `internal/adapters/gallery/fs.go`, `internal/adapters/report/report.go`, `internal/cli/cli.go`, `cmd/codeshot/main.go`, `Taskfile`, `README.md`
- Test: `internal/adapters/gallery/fs_test.go`, `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: every port from Task 14 and every adapter from Tasks 8-13.
- Produces: `gallery.FS{Dir string}` with `Exists(string) bool` and `Store(string, image.Image) (string, error)`; `report.Writer{Err io.Writer}` with `Stored(string)` and `Warn(string)`; `cli.Run(args []string, stdout, stderr io.Writer) int`.

The CLI uses `flag.FlagSet` from the standard library — stdlib, not a framework, and it keeps the parsing honest. Phase 2 adds `--` splitting for the wrapper before handing the remainder to the FlagSet.

- [ ] **Step 1: Write the failing tests**

Create `internal/adapters/gallery/fs_test.go`:

```go
package gallery

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func pixel() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	return img
}

func TestStoreWritesAPNGIntoTheGallery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Codeshots")
	g := FS{Dir: dir}
	path, err := g.Store("shot.png", pixel())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != filepath.Join(dir, "shot.png") {
		t.Errorf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[1:4]) != "PNG" {
		t.Error("the file is not a PNG")
	}
}

func TestStoreCreatesTheGalleryDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "Codeshots")
	if _, err := (FS{Dir: dir}).Store("x.png", pixel()); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

func TestStoreTreatsANameWithASeparatorAsAPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere", "shot.png")
	path, err := (FS{Dir: filepath.Join(dir, "Codeshots")}).Store(target, pixel())
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if path != target {
		t.Errorf("path = %q, want %q", path, target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("nothing written to the requested path: %v", err)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	g := FS{Dir: dir}
	if g.Exists("nope.png") {
		t.Error("Exists lied about a missing file")
	}
	os.WriteFile(filepath.Join(dir, "yes.png"), []byte("x"), 0o644)
	if !g.Exists("yes.png") {
		t.Error("Exists missed a file that is there")
	}
}
```

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeANSI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.ansi")
	body := "\x1b[1;32mhello\x1b[0m\r\nsecond line\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderWritesAShot(t *testing.T) {
	src := writeANSI(t)
	out := filepath.Join(t.TempDir(), "shot.png")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", src, out, "--command", "echo hello"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("no image written: %v", err)
	}
	if !strings.Contains(stderr.String(), "Stored codeshot in") {
		t.Errorf("stderr = %q, want the stored line", stderr.String())
	}
}

func TestRenderRejectsAnUnknownTheme(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"render", writeANSI(t), "x.png", "--theme", "no-such"}, &stdout, &stderr)
	if code == 0 {
		t.Error("exit 0 for an unknown theme")
	}
	if !strings.Contains(stderr.String(), "theme") {
		t.Errorf("stderr = %q, want the theme named in the error", stderr.String())
	}
}

func TestRenderRejectsAMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"render", "/no/such/file.ansi"}, &stdout, &stderr); code == 0 {
		t.Error("exit 0 for a missing input file")
	}
}

func TestHelpAndVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Errorf("--help exited %d", code)
	}
	if !strings.Contains(stdout.String(), "codeshot") {
		t.Errorf("help = %q", stdout.String())
	}
	stdout.Reset()
	if code := Run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Errorf("version exited %d", code)
	}
}

func TestThemesListsWhatIsEmbedded(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"themes"}, &stdout, &stderr)
	for _, want := range []string{"codeshot-dark", "codeshot-light"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("themes output %q is missing %s", stdout.String(), want)
		}
	}
}

func TestNoArgumentsExplainsItself(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code == 0 {
		t.Error("exit 0 with no arguments")
	}
	if !strings.Contains(stderr.String(), "render") {
		t.Errorf("stderr = %q, want a hint about the render subcommand", stderr.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/adapters/gallery/ ./internal/cli/`
Expected: FAIL — no such packages.

- [ ] **Step 3: Write the gallery and the reporter**

Create `internal/adapters/gallery/fs.go`:

```go
// Package gallery keeps shots on disk. A bare name lands in the gallery
// directory; anything with a separator in it is a path the caller chose, and
// is honoured as given.
package gallery

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type FS struct {
	Dir string
}

func (g FS) path(name string) string {
	if strings.ContainsRune(name, filepath.Separator) {
		return name
	}
	return filepath.Join(g.Dir, name)
}

func (g FS) Exists(name string) bool {
	_, err := os.Stat(g.path(name))
	return err == nil
}

func (g FS) Store(name string, img image.Image) (string, error) {
	path := g.path(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", fmt.Errorf("encoding %s: %w", path, err)
	}
	return path, f.Close()
}
```

`f.Close` runs twice — once here and once from the defer — which is harmless for a file and means a failed flush is reported rather than swallowed.

Create `internal/adapters/report/report.go`:

```go
// Package report writes codeshot's one line of output. It goes to stderr, so
// that a wrapper's passthrough on stdout stays exactly what the command
// printed.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type Writer struct {
	Err io.Writer
}

func (w Writer) Stored(path string) {
	fmt.Fprintf(w.Err, "Stored codeshot in %s\n", tildify(path))
}

func (w Writer) Warn(msg string) {
	fmt.Fprintf(w.Err, "codeshot: %s\n", msg)
}

func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
```

- [ ] **Step 4: Write the CLI**

Create `internal/cli/cli.go`:

```go
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
```

- [ ] **Step 5: Export the colour parser the CLI needs**

`--background` needs the hex parser Task 9 kept unexported. In `internal/adapters/theme/theme.go`, rename `parseColor` to `ParseColor` (updating its three call sites inside the package) and give it a doc comment:

```go
// ParseColor reads #rrggbb, rrggbb or #rgb. It is exported because the CLI
// parses --background with exactly the same rules a theme file uses.
func ParseColor(s string) (domain.RGBA, error) {
```

- [ ] **Step 6: Write the composition root**

Create `cmd/codeshot/main.go`:

```go
// Command codeshot renders a command and its output as a picture of a terminal
// window. This file exists to choose the adapters and get out of the way.
package main

import (
	"os"

	"codeshot/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Take a shot of something real**

```bash
go build -o codeshot ./cmd/codeshot
ls --color=always -la > /tmp/ls.ansi 2>/dev/null || ls -G -la > /tmp/ls.ansi
./codeshot render /tmp/ls.ansi ls-demo.png --command "ls -la" --cols 100
open ~/Codeshots/ls-demo.png
```

Look at it. The traffic lights should sit where macOS puts them, the corners should be round, the shadow soft, the colours should be the ones the terminal showed. Anything that looks wrong here is a bug worth fixing before phase 2 builds on it.

- [ ] **Step 9: Write the Taskfile and the README**

Create `Taskfile`, in the shape `~/code/paradajz` uses:

```bash
#!/usr/bin/env bash
# =========================================================
# Taskfile gives you a set of quick tasks for your project
# More info: https://github.com/Enrise/Taskfile
# =========================================================

function task:test { ## Run every test
	go test ./...
}

function task:build { ## Build the binary
	go build -o codeshot ./cmd/codeshot
}

function task:golden { ## Rewrite the golden images after a deliberate change
	go test ./internal/adapters/render/raster/ -update
}

function task:demo { ## Take a shot of ls, to look at
	task:build
	ls --color=always -la > /tmp/codeshot-demo.ansi 2>/dev/null || ls -G -la > /tmp/codeshot-demo.ansi
	./codeshot render /tmp/codeshot-demo.ansi demo.png --command "ls -la"
}

function task:fmt { ## Format everything
	gofmt -w .
}

function task:help { ## Show this help
	grep -E '^function task:.*##' "$0" | sed 's/function task:/  /; s/ *{ *## /\t/' | expand -t 24
}

"task:${1:-help}" "${@:2}"
```

```bash
chmod +x Taskfile
```

Write `README.md`: what codeshot is, the constraint that it never re-runs a command and therefore has to be told about it up front, the `render` usage above, a note that the wrapper (`codeshot x.png -- cmd`) arrives in phase 2, and the theme and font story. Keep it to a page.

- [ ] **Step 10: Commit**

```bash
gofmt -l . && go test ./...
git add -A
git commit -m "Add the gallery, the CLI and a binary that renders a dump"
```

---

## Self-review

Checked after the plan was written, against the spec.

**Spec coverage.** §4 domain: Tasks 1-4 and 14 (`Slug`/`ResolveName`). §5 hexagon: Task 14 declares the ports, Task 15 wires them, every adapter sits under `internal/adapters`. §6 flows: only the `render` flow is in this phase; wrapper and pipe are phase 2 by design. §7 rendering: geometry, text passes, decorations in Tasks 11-13; fonts in Task 10; themes in Task 9; the VT scope list in Tasks 5-8. §8 framing: Task 4, with `--rows`/`--tail` reaching it through Task 15. §9 CLI: the `render` subcommand and its flags in Task 15. §10 exit codes: Task 15 returns 2 for usage errors and 1 for failures; the child's exit code is a phase 2 concern. §11 testing: every task is test-first, goldens in Task 13. §12 risks: risk 1 in Task 13 step 7, risk 2 in Task 10 step 6, risk 3 bounded by Task 6's `Unknown` reporting, risk 4 in Task 5's combining-mark tests.

**Deferred to later phases, deliberately, not forgotten:**

- Wrapper and pipe capture sources, `SIGWINCH`, exit-code propagation, the shell shims — phase 2.
- `--clip`, `--stdout`, `--force`, `--out`, `--radius`, `--font` — phase 2's CLI pass; this phase's flag set covers what `render` can actually use.
- Reading `~/.config/ghostty/config`, `~/.config/codeshot/config.toml`, the system font index, named-theme directories, `doctor`, `install.sh` — phase 3. `theme.Source.Dirs` and `fonts.Set` are already shaped to receive them.
- Synthetic italic shear: the embedded family ships a real italic, so it is unreachable until fallback fonts arrive in phase 3.
- Colour emoji: `sbix` bitmaps are outside `x/image`; recorded as a known limitation in the spec.

**Type consistency.** `domain.Cell` gains `Combining string` in Task 5 and stays comparable, which Tasks 11-13 rely on when they compare styles and colours. `Grid.Cols`/`Lines`, `Result.Main`/`Alt`/`UsedAlt`/`Title`, `Chrome`'s field names and `fonts.Metrics{CellW, CellH, Ascent}` are used identically wherever they appear. `theme.parseColor` becomes `theme.ParseColor` in Task 15 step 5, which is the only rename in the plan.
