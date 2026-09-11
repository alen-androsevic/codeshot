# codeshot — what's left

Phase 1 is merged: the pure `bytes → PNG` core, a VT emulator, a native
rasteriser, and `codeshot render`. This file is everything still owed,
in the order it makes sense to do it. Design rationale lives in
`docs/design.md`; decisions already settled are in `docs/adr/`.

---

## Phase 2 — capture

The point of the whole tool. Today codeshot can only render a dump you
captured yourself, which is why `USAGE.md` has to open with a `script`
incantation. Phase 2 removes that step.

**2.1 The wrapper — `codeshot shot.png -- npm test`**
Run the command once in a pty codeshot owns, stream the output to the
terminal live so it looks like a normal run, tee a copy, render it. Size the
pty from **stderr** (stdout may be redirected), forward `SIGWINCH`, and exit
with the child's status so codeshot is drop-in in front of anything. The
"Stored codeshot in …" line goes to stderr so a redirect of stdout stays
exactly what the command printed.

Blockers already cleared: `--` is a proper terminator, `--cols` defaults to 0
so "unset" is distinguishable, `fonts.Set` is mutex-guarded for a goroutine
copy loop.

**2.2 Pipe mode — `npm test | codeshot shot.png`**
Read stdin to EOF. Ship optional shell shims (`shim/codeshot.zsh`,
`shim/codeshot.bash`) that recover the command text from `fc -ln -1` and
strip the trailing `| codeshot …`, so the prompt line is right without
`--command`. Document plainly that a pipe is not a tty and most tools drop
their colour — the wrapper is the answer, this is the fallback.

**2.3 Move the wiring to `cmd/codeshot`**
`render()` in `internal/cli` is already ~100 lines of parse-validate-wire-run.
Two more capture sources, tty sizing, signal handling and `--stdout` stream
switching will make it unreadable. Extract a `cli.Build()` that returns the
service, keep `internal/cli` testable, and let `cmd/codeshot` own the
composition. Do this *before* 2.1 lands, not after.

**2.4 Move the carriage-return warning behind the port**
`warnAboutMissingCarriageReturns` reaches around `CaptureSource` to read the
file itself (`internal/cli/cli.go`). It cannot exist for the pty or pipe
sources, so phase 2 would have to special-case "is this the file source?".
Push it into `file.Source`, or a `CaptureSource` decorator.

**2.5 The flags phase 2 implies**
`--out`, `--stdout` (PNG to stdout, passthrough moves to stderr), `--clip`
(pbcopy / wl-copy / xclip behind a `Clipboard` port), `--force`.

---

## Phase 3 — make it look like *your* terminal

**3.1 Read `~/.config/ghostty/config` when it exists**
`theme`, `font-family`, `font-size`, `window-padding-x/y`. Opportunistic —
its absence is never an error. This is what makes a shot match the terminal
it came from.

**3.2 Resolve named themes from a Ghostty install**
`theme.Source.Dirs` already exists and is empty. Fill it with the local
themes directory so `--theme tokyonight` works against the several hundred
Ghostty ships.

**3.3 System font index + fallback chain**
Scan the OS font directories, index family names from each font's `name`
table, cache to `~/.cache/codeshot/fonts.json` with mtime invalidation. Then
`--font` works, Nerd Font icons render if one is installed, and the dead
nil-variant fallback in `fonts.Face` becomes live. Add the 12° synthetic
italic shear here — it is unreachable until a fallback font without an italic
cut is in play.

**3.4 `doctor`, `install.sh`, `--config`**
A `doctor` that reports what it found (Ghostty config, fonts, gallery
permissions), an `install.sh` that stamps the version, and
`~/.config/codeshot/config.toml` for defaults.

**3.5 Colour emoji**
Apple Color Emoji is `sbix`, a bitmap format `golang.org/x/image` cannot
read. Needs a separate adapter that decodes the bitmap strike and composites
it. Currently documented as a known limitation in `USAGE.md`.

---

## Correctness and coverage debt

Carried from the final review's triage. None block use; all are real.

**Behaviour**
- `scrollUp`/`scrollDown` fill revealed lines with `domain.Style{}` instead of
  the caller's current background. ECMA-48 says SU/SD fill with the current
  SGR background, and real TUIs rely on it. Same colour-fidelity family as
  the `blank()` gap that phase 1 fixed.
- `domain.Slug` truncates with `s[:slugMax]`, slicing bytes — a non-ASCII
  command can be cut mid-rune into an invalid-UTF-8 filename.
- `withPNG` treats any `.` as an existing extension, so
  `codeshot render x.ansi my.backup` writes PNG bytes to `my.backup`.
- The carriage-return heuristic fires on output that resets the column with
  `ESC[G` rather than `\r`. It renders correctly and still warns; a warning
  that cries wolf gets ignored when it matters.
- `Emulator.osc` grows without bound on an unterminated OSC string.
- A malformed (not merely unknown) string escape drops to ground without
  calling `Unknown`, so it goes unlogged under `--debug`.

**Tests**
- `internal/adapters/capture/file` has no test at all — the `os.Getwd()`
  fallback and the read-error wrapping are unexercised, and it is phase 1's
  entry point to the whole pipeline.
- `internal/adapters/report` has no test; `tildify`'s `~` contraction is
  never exercised because every test uses `t.TempDir()`.
- No CLI test for `--background`, either the reject or the accept path.
- CSI `E`, `F` and `d` are dispatched but untested.
- SGR truncation is tested only for `38;5`; the `38;2`/`48;2` short-triple
  path is structurally identical and uncovered.
- Theme lookup has no precedence test (embedded beats `Dirs` beats literal
  path) — phase 3 layers a Ghostty directory onto exactly that ordering.
- 3-digit shorthand hex (`#abc`) is implemented but unexercised.
- `TestControlsNoneDrawsNoButtons` cannot reach the branch it names:
  `layout()` zeroes the titlebar for that configuration before `drawChrome`
  runs. Either make it reach the branch or rename it.
- The traffic-light test colour-checks only red and green.
- No `.ansi` corpus fixtures (`ls --color`, `git status`, a `\r` spinner, an
  alt-screen TUI) and no end-to-end smoke task — `task demo` renders but
  asserts nothing.

**Tidying**
- `tildify` is implemented twice, in `report` and `prompt`. One helper.
- `domain.Frame.Title` is set by `Compose` and read by nothing; the renderer
  uses `Chrome.Title`. Delete the field or use it.
- `codeshot themes` hardcodes the two theme names instead of asking
  `theme.Source`. A third embedded theme would make the subcommand lie.
- `gallery.FS.Store` closes the file twice (deferred plus returned).
- `fitTitle` shrinks one rune at a time with a `MeasureString` per step.
  Fine for command lines; worth a comment if it ever sees hostile input.

---

## Infrastructure

- **No CI.** Nothing runs the suite on push. Golden-image comparison is
  byte-exact and verified identical on macOS and Linux arm64 — but never on
  amd64, and it is hostage to any change in `compress/flate` across Go
  toolchains (`docs/adr/0002`). A CI matrix is how that stays honest.
- **No remote.** The repo is local-only.
- **Missing phase-1-able flags** the design names: `--radius` (the `Chrome`
  field is plumbed, just not exposed), `--prompt`, `--font`.

---

## Known limitations to keep documenting

- Emoji and unpatched Nerd Font glyphs render as tofu (3.5, 3.3).
- codeshot cannot reach a command that has already finished. That is a
  deliberate consequence of never re-running and keeping no recorder — the
  reasoning is in `docs/design.md` §2.
