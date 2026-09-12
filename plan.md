# codeshot — what's left

All three phases are merged: the pure `bytes → PNG` core, a VT emulator, a
native rasteriser, `codeshot render`, capture (`codeshot -- ls -la` under a
pty, `npm test | codeshot` from a pipe, and the flags that decide where the
picture goes), and the profile work that makes a shot look like the terminal
it came from. This file is everything still owed,
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

## Phase 3 — done

Ghostty's config is read (theme, font-size, window padding), named themes
resolve against a Ghostty installation's several hundred, the machine's fonts
are indexed and cached, `--font` draws with any of them, a rune the family
lacks falls back through the platform's faces, a family with no italic cut is
sheared 12°, colour emoji are decoded from Apple's sbix bitmaps, and
`doctor`, `install.sh` and `~/.config/codeshot/config` exist.

Two decisions worth remembering. codeshot's config is Ghostty-flavoured
`key = value` rather than the TOML design §4 named: every key is a flag name,
there are no sections, and TOML would have cost a dependency and a second
format. And a `theme` that names a light and a dark one takes the dark one,
rather than following the system appearance, so that the same command gives
the same picture at any time of day.

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
- No CLI test for `--background`, either the reject or the accept path.
- CSI `E`, `F` and `d` are dispatched but untested.
- SGR truncation is tested only for `38;5`; the `38;2`/`48;2` short-triple
  path is structurally identical and uncovered.
- 3-digit shorthand hex (`#abc`) is implemented but unexercised.
- `TestControlsNoneDrawsNoButtons` cannot reach the branch it names:
  `layout()` zeroes the titlebar for that configuration before `drawChrome`
  runs. Either make it reach the branch or rename it.
- The traffic-light test colour-checks only red and green.
- No `.ansi` corpus fixtures (`ls --color`, `git status`, a `\r` spinner, an
  alt-screen TUI) and no end-to-end smoke task — `task demo` renders but
  asserts nothing.

**Tidying**

---

## Infrastructure

- **No CI.** Nothing runs the suite on push. Golden-image comparison is
  byte-exact and verified identical on macOS and Linux arm64 — but never on
  amd64, and it is hostage to any change in `compress/flate` across Go
  toolchains (`docs/adr/0002`). A CI matrix is how that stays honest.
- **No remote.** The repo is local-only.
- **No `.ansi` corpus** and no end-to-end smoke task; `task demo` renders but
  asserts nothing.

---

## Known limitations to keep documenting

- Colour emoji work on macOS only. Apple's sbix is the one bitmap format
  codeshot reads; Linux's Noto Color Emoji uses CBDT/CBLC or COLR, and
  Windows's Segoe UI Emoji is COLR, so emoji are tofu there.
- Nerd Font icons need a Nerd Font installed and named with `--font`; the
  embedded family carries only a handful of genuine Powerline glyphs.
- An emoji made of several runes - a skin tone modifier, a ZWJ sequence -
  renders as its base emoji: the grid is runes, and one rune is what the
  bitmap lookup gets.
- Typeahead during rendering is lost. The goroutine forwarding a terminal
  stdin into the pty cannot be cancelled - a blocking read on a tty has no
  deadline - so keys typed after the child exits but before codeshot does
  are read and dropped. `script` has the same property.
- A background grandchild holding the pty open keeps codeshot waiting, since
  end-of-output is the last holder of the pty closing it. Same as `script`.
- The shims only work in an interactive shell, because a non-interactive
  one keeps no history. Both halves are tested, the recovery half against a
  real interactive zsh and bash.
- Pipe mode sees standard output alone, because that is all a `|` carries.
  A command that writes to standard error - jest, and most test runners -
  puts the interesting half somewhere codeshot never sees; `2>&1 |` or the
  wrapper is the answer, and no change here can be.
- Pipe mode cannot recover colour a tool dropped on seeing a pipe, and
  cannot learn the command without a shim or `--command`.
- codeshot cannot reach a command that has already finished. That is a
  deliberate consequence of never re-running and keeping no recorder — the
  reasoning is in `docs/design.md` §2.
