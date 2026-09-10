# codeshot — design

**Date:** 2026-09-10
**Status:** approved, ready for planning

## 1. Purpose

`codeshot` turns one command and its output into a PNG that looks like a Ghostty
window — rounded corners, macOS traffic lights, your terminal's theme and font.
It is a documentation and sharing tool: the image you paste into a README, an
issue, or a chat message.

```
❯ codeshot danas-usage-example.png -- paradajz danas
… paradajz's output, live, in colour, exactly as if you had run it bare …
Stored codeshot in ~/Codeshots/danas-usage-example.png
```

## 2. Constraints

These are hard requirements, and between them they determine the architecture.

1. **A command is never executed twice.** The command runs once, under codeshot,
   or not at all.
2. **Neither Ghostty nor cmux is a dependency.** The image is synthesised from
   bytes. No terminal is spawned, nothing is screenshotted, no Screen Recording
   permission is requested.
3. **The image contains the exact output and the exact command line.** Not a
   re-render, not an approximation of what probably happened.
4. **It works in a bare zsh or bash shell**, with no shell integration required.
5. **No terminal output is ever written to disk.** There is no session
   recorder and no ring buffer. The only files codeshot writes are the image
   itself and a font-metadata index cache (family names and paths, never
   terminal content).

### What these constraints rule out

A process cannot read its parent terminal's scrollback. With re-execution and
terminal-specific APIs both excluded, the only way to hold the exact bytes of a
command that has already finished would be to have been recording when it ran —
a pty session recorder wrapping the interactive shell, in the manner of `script`
or `asciinema`. Constraint 5 rules that out.

**Therefore codeshot cannot reach the previous command (`!`).** It only sees
what it is handed, which means the decision to take a shot is made *before* the
command runs. This was a deliberate, informed trade: no background recorder, no
terminal output on disk.

## 3. Non-goals

- Capturing a command that has already finished.
- Recording or replaying whole sessions (asciinema's job).
- Animated output (GIF/APNG).
- Editing, annotating or arranging images after the fact.
- Being a terminal emulator anyone would type into.

## 4. Domain

The core insight: **codeshot is a pure function from bytes to an image.**
Everything impure — pty, filesystem, fonts, clipboard — lives at the edge.

### Ubiquitous language

- **Capture** — everything one command produced: command text (or argv), cwd,
  exit code, duration, terminal size, and the raw ANSI byte stream. The only
  thing a `CaptureSource` ever yields.
- **Screen**, **Cell**, **Style** — the emulated grid. A Cell is a rune plus a
  Style (fg, bg, bold, italic, underline, strikethrough, inverse, dim). A
  Style's colours are **palette-relative** — indexed 0–15, 256-colour, or true
  colour — and resolve against a **Theme** only at render time. That
  indirection is what lets one Capture be rendered in any theme.
- **Theme** — background, foreground, cursor, and a 16-colour palette, in
  Ghostty's theme file format.
- **Prompt** — a template resolved against the Capture's cwd (`{cwd}` → `~`).
  In wrapper mode the real prompt was drawn by the parent shell before codeshot
  ever ran, so the prompt line in the image is synthesised.
- **Transcript** — Screen plus the Prompt line and command line above it.
- **Frame** — the Transcript cropped to a viewport by an explicit rule (§8).
- **Chrome** — control style (`macos` | `linux` | `none`), title, padding,
  corner radius, shadow, margin, scale.
- **Window** — Frame plus Chrome. The thing that gets rasterised.
- **Codeshot** — the artifact: a Window, a name, and a destination in the
  **Gallery** (`~/Codeshots` by default).

### Profile resolution

Theme, font and prompt are *profile* concerns, resolved by precedence:

1. command-line flags
2. `~/.config/codeshot/config.toml`
3. `~/.config/ghostty/config`, **if it happens to exist** — `theme`,
   `font-family`, `font-size`, `window-padding-x`, `window-padding-y`
4. built-in defaults

Step 3 is what makes the image look like *your* terminal while keeping Ghostty
entirely optional. Its absence is never an error.

## 5. Architecture

Hexagonal, with a DDD-shaped core.

**Driving side.** `internal/cli` parses argv and is the only code that knows
flags exist. `cmd/codeshot` is the composition root: it wires concrete adapters
into the use case and does nothing else. One application service,
`app.Capture`, executes the pipeline: *source → emulate → frame → render →
store → report*.

**Driven ports**, declared in `internal/app/ports.go`, implemented under
`internal/adapters/`:

| Port | Adapters in scope | Adapters the port leaves room for |
|---|---|---|
| `CaptureSource` | `pty` (wrapper), `pipe` (stdin), `file` (saved dump) | session recorder, tmux/cmux scrollback |
| `Emulator` | in-house VT | `charmbracelet/x/vt` |
| `Renderer` | `raster` (native Go) | `svg`, `html` |
| `FontResolver` | embedded JetBrains Mono + system directory scan | CoreText, fontconfig |
| `ThemeSource` | embedded themes, Ghostty config/theme parser | iTerm2 `.itermcolors` |
| `PromptSource` | template | `--prompt-command` (e.g. `starship prompt`) |
| `Gallery` | filesystem | — |
| `Clipboard` | `pbcopy` / `wl-copy` / `xclip` | — |
| `Tty` | unix ioctl (size, isatty) | ConPTY / Windows |

### The OS seam

Only four things are per-OS, one file each behind a build tag: **pty creation**,
**font directories**, **clipboard**, **config/cache paths**. Chrome style is
deliberately *not* an OS branch — it is a domain option, so a Linux machine can
render macOS traffic lights and vice versa. Porting to another OS is those four
files plus, optionally, a new chrome preset.

### Dependencies

`github.com/creack/pty`, `github.com/mattn/go-runewidth`,
`golang.org/x/image`. Nothing else. Go, `module codeshot`, hand-rolled argument
parsing, matching the conventions of the paradajz repo.

### Why native rasterisation

Considered and rejected: emitting SVG and rasterising with resvg/librsvg (drags
in cgo or an external binary), and emitting HTML for a headless browser
(perfect typography, but a browser is a heavier dependency than the Ghostty
just removed). Walking the cell grid with `golang.org/x/image` is deterministic,
dependency-free, identical on macOS, Linux and CI, and golden-image testable.
The cost is owning font fallback, and no bitmap colour emoji (§7).

### Why an in-house VT emulator

The required surface is bounded (§7, scope list) — roughly 800 testable lines —
against pulling in a library and its dependency tail. It sits behind the
`Emulator` port, so swapping in `charmbracelet/x/vt` later is one adapter.

## 6. Flows

### Wrapper — `codeshot danas.png -- paradajz danas`

Primary mode. Allocate a pty sized from **stderr** (stdout may be redirected),
spawn the command with `TERM` and `COLORTERM` set, copy pty → stdout live so
the user watches it run normally, and tee into an in-memory buffer. Forward
`SIGWINCH`; propagate the child's exit code as codeshot's own. The command text
comes from argv, so the rendered line is exactly `❯ paradajz danas`.

The wrapper is transparent: passthrough on stdout, the "Stored codeshot in …"
line on stderr.

### Pipe — `paradajz danas | codeshot danas.png`

Read stdin to EOF. codeshot cannot know the command, so an optional shell shim
(`shim/codeshot.zsh`, `shim/codeshot.bash`) recovers it from `fc -ln -1`,
strips the trailing `| codeshot …`, and passes it as `--command`. Without the
shim, pass `--command "…"` by hand or the prompt line is omitted.

This mode is honestly degraded: a pipe is not a tty, so tools that check
`isatty` drop their colour. The docs say so and point at the wrapper.

### Render — `codeshot render session.ansi out.png`

A saved ANSI dump straight into the pipeline. This is the mode that makes
phase 1 shippable and testable without a terminal.

All three converge on the same `Capture`; from there the pipeline is pure.

## 7. Rendering

### Geometry

Rendered at `--scale 2` by default, so the PNG is retina-crisp. Cell width is
the font's advance rounded to an integer pixel, so the grid never drifts; cell
height is ascent + descent + line gap, times `--line-height` (default 1.0).
Default font size 13.

- Titlebar 28px, painted the same colour as the terminal background — Ghostty's
  transparent-titlebar look, with no separator hairline.
- Traffic lights at x = 20, 40, 60, ø12, vertically centred: `#FF5F57`,
  `#FEBC2E`, `#28C840`, each with a slightly darker ring.
- Title centred, 11px, dimmed foreground. Defaults to the command text.
- Corner radius 10 on all four corners.
- Shadow: three-pass box blur (a good gaussian approximation), offset y+18,
  radius 40, α 0.35. `--margin` defaults to 48 with the shadow on, 0 with it
  off. Outside the margin: transparent, unless `--background #hex`.

### Text

Two passes. First, merge runs of cells sharing a background colour into single
rectangles — this avoids seams between adjacent fills. Then glyphs.

- Bold prefers a real bold face; falls back to synthetic emboldening.
- Italic prefers a real italic face; falls back to a 12° shear.
- Dim blends the foreground toward the background.
- Inverse swaps fg and bg at style-resolution time, before the theme applies.
- Underline and strikethrough use the font's own metrics.
- No cursor is drawn.
- Wide and combining runes are handled at grid level via `go-runewidth`.

### Fonts

A chain: `--font` → embedded JetBrains Mono (Ghostty's own default, so the
resemblance is free) → system monospace candidates → tofu box. The resolver
scans OS font directories once, indexes family names from each font's `name`
table, and caches the index to `~/.cache/codeshot/fonts.json` with mtime
invalidation. Glyph coverage is checked per rune.

**Known limitations.** Apple Color Emoji is a bitmap (`sbix`) format
`golang.org/x/image` cannot read, so emoji render as tofu until a dedicated
adapter exists. Nerd Font icons render only when a Nerd Font is installed and
selected by name.

### Themes

Ghostty's theme file format, so `--theme /path/to/any/ghostty/theme` works
against the several hundred Ghostty ships. A small set is embedded for
zero-config use: Ghostty's default, catppuccin-mocha, tokyonight, nord,
gruvbox-dark, solarized-dark, rose-pine-dawn.

### VT emulator scope

In scope: SGR (including 256-colour and true colour), CUP/CUU/CUD/CUF/CUB,
ED/EL, scroll up/down, alternate screen enter/leave, OSC title, autowrap
(DECAWM), `\r`, `\n`, `\t`, `\b`. Everything else is ignored silently and
logged under `--debug`. The emulator keeps scrollback, so nothing is lost when
output exceeds the terminal height.

## 8. Framing rules

The pty is sized from the caller's terminal, so that wrapping and TUI layout
match what the user saw; without a tty it is 100×24. `--cols` overrides the pty
width. Pty *height* has no flag — it only affects how a TUI lays itself out, and
`--rows` is reserved for cropping the frame, below.

The frame is chosen by one explicit rule:

- **If the program used the alternate screen**, the frame is exactly
  `cols × rows` — the final screen state.
- **Otherwise**, the frame is the prompt line, the command line, and every
  output line, from scrollback.

Then: trailing all-blank lines are trimmed to none; `--rows N` crops to the
first N lines (`--tail` crops to the last N instead).

Defaults: no row crop, and no trailing prompt line after the output.
`--rows 24` reproduces the framing of the motivating example.

## 9. CLI surface

```
codeshot [flags] [name] -- <command> [args…]   wrapper (primary)
codeshot [flags] [name]                        stdin (pipe)
codeshot render <file.ansi> [name]             saved dump
codeshot themes | fonts | doctor | config | version
```

`[name]` is optional; omitted, it slugs the command (`paradajz-danas.png`, then
`-2`, `-3` on collision). A bare word lands in the gallery; anything containing
`/` is treated as a path. A missing extension gets `.png`.

Flags, in four groups, each mirrored by a `~/.config/codeshot/config.toml` key:

- **output** — `--out`, `--gallery`, `--clip`, `--stdout`, `--force`
- **frame** — `--cols`, `--rows`, `--tail`, `--prompt`, `--no-prompt`,
  `--command`, `--cwd`
- **window** — `--controls macos|linux|none`, `--title`, `--no-title`,
  `--padding`, `--radius`, `--shadow`, `--margin`, `--scale`, `--background`
- **style** — `--theme`, `--font`, `--font-size`, `--line-height`

`--stdout` writes the PNG to stdout and moves passthrough to stderr.

## 10. Errors and exit codes

In wrapper mode codeshot exits with the child's exit code, so it is drop-in in
front of anything. A failure in codeshot itself — unwritable gallery, unknown
theme, unreadable font — exits 1 with a message on stderr, *after* the child has
already run and its output has already passed through. Losing the image must
never look like losing the command.

Nonexistent command in wrapper mode: exit 127, matching the shell.
Empty capture (no output at all): still a valid image — prompt and command line
only.

## 11. Testing

TDD throughout. The architecture pays for itself here: everything from bytes to
PNG is pure and needs no terminal.

- **Domain** — unit tests for framing rules, style and theme resolution, name
  slugging and collision handling.
- **Emulator** — table-driven fixtures. A `.ansi` corpus captured from real
  tools (`ls --color`, `git status`, a `\r` progress spinner, an alt-screen
  TUI) asserted against compact grid dumps with a style legend.
- **Renderer** — golden PNGs, rendered with the embedded font only so no system
  font can perturb them, compared byte-exact, with an `-update` flag to
  regenerate.
- **pty adapter** — an integration test asserting captured bytes, exit-code
  propagation and `SIGWINCH` forwarding.
- **End-to-end** — a smoke task in the `Taskfile`.

## 12. Risks

1. **Golden-image determinism across macOS and Linux.** Verify in the very
   first renderer commit; if rasterisation differs, fall back to comparing with
   a per-pixel tolerance.
2. **Does JetBrains Mono cover `❯` (U+276F)?** The answer decides how early the
   fallback chain must be working. Check before relying on the embedded font.
3. **Scope creep in the in-house VT.** Bounded by the explicit list in §7;
   anything outside it is ignored, not implemented.
4. **Combining marks and wide runes in the grid.** Get the corpus fixtures in
   early.

## 13. Phases

Each phase is shippable on its own.

1. **Core.** Domain, emulator, renderer, `codeshot render file.ansi`. No pty,
   no OS-specific code, fully testable.
2. **Capture.** Pipe and wrapper sources, the real CLI, exit-code propagation,
   the gallery.
3. **Profile and polish.** Ghostty config reading, theme and font resolution,
   `doctor`, `install.sh`, `Taskfile`, shell shims.

## 14. Repo layout

```
cmd/codeshot/            composition root
internal/domain/         pure: screen, cell, style, theme, frame, chrome, name
internal/app/            use case + ports
internal/adapters/
  capture/pty/           wrapper mode
  capture/pipe/          stdin mode
  capture/file/          saved dump
  vt/                    emulator
  render/raster/         native Go rasteriser
  fonts/                 embedded + system index
  theme/                 embedded themes + ghostty parser
  gallery/               filesystem
  clipboard/             pbcopy / wl-copy / xclip
  tty/                   size, isatty
internal/cli/            flag parsing
shim/                    codeshot.zsh, codeshot.bash
docs/adr/                decisions as they accrue
Taskfile
install.sh
```
