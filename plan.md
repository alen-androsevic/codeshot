# codeshot — what's left

Phases 1 and 2 are merged: the pure `bytes → PNG` core, a VT emulator, a
native rasteriser, `codeshot render`, and capture - `codeshot -- ls -la`
under a pty, `npm test | codeshot` from a pipe, and the flags that decide
where the picture goes. This file is everything still owed,
in the order it makes sense to do it. Design rationale lives in
`docs/design.md`; decisions already settled are in `docs/adr/`.

---

## Phase 2 — done

The wrapper (`capture/pty`), pipe mode (`capture/pipe`), the shared
passthrough (`capture/tap`), the clipboard adapter, `--out`, `--stdout`,
`--clip` and the shell shims are all in, along with the two refactors that
had to precede them: the parse/build split in the CLI and the
carriage-return warning moving into `file.Source`.

`--force` was dropped rather than built: an explicit name still overwrites,
so there is nothing for it to unlock. If refusing to overwrite ever becomes
the default, this is the flag that would make it bearable.

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
- The carriage-return heuristic fires on output that resets the column with
  `ESC[G` rather than `\r`. It renders correctly and still warns; a warning
  that cries wolf gets ignored when it matters.
- `Emulator.osc` grows without bound on an unterminated OSC string.
- A malformed (not merely unknown) string escape drops to ground without
  calling `Unknown`, so it goes unlogged under `--debug`.

**Tests**
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
- Typeahead during rendering is lost. The goroutine forwarding a terminal
  stdin into the pty cannot be cancelled - a blocking read on a tty has no
  deadline - so keys typed after the child exits but before codeshot does
  are read and dropped. `script` has the same property.
- A background grandchild holding the pty open keeps codeshot waiting, since
  end-of-output is the last holder of the pty closing it. Same as `script`.
- The shims only work in an interactive shell, because `fc` needs history.
  The stripping half is tested; the history lookup is not testable.
- Pipe mode cannot recover colour a tool dropped on seeing a pipe, and
  cannot learn the command without a shim or `--command`.
- codeshot cannot reach a command that has already finished. That is a
  deliberate consequence of never re-running and keeping no recorder — the
  reasoning is in `docs/design.md` §2.
