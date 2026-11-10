# codeshot

codeshot turns a command and what it printed into a PNG of a terminal window.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/alen-androsevic/codeshot/main/install.sh | bash
```

That downloads the latest release for macOS or Linux into `~/bin` (or
`~/.local/bin`). From a clone with Go installed, `./install.sh` builds what
you have checked out instead.

```sh
codeshot -- ls -la        # runs ls -la, and stores ~/Codeshots/ls-la.png
```

## What it does

The shot has rounded corners, traffic lights, a soft drop shadow, and text in
whatever colours the terminal actually showed.

codeshot never re-runs your command. It has no idea how to reproduce a build
failure, a flaky test, or a program that reads a password from stdin, so it
doesn't try. You decide up front: put `codeshot --` in front of a command, and
codeshot runs it once, under a terminal it owns, and takes a shot of exactly
what it printed.

## Usage

```
codeshot [name] [flags] -- <command> [args...]   run a command, take a shot
<command> | codeshot [name] [flags]              take a shot of what comes in
codeshot render <file.ansi> [name] [flags]       take a shot of a saved dump
codeshot themes [--json]                         list the themes that resolve
codeshot doctor [--json]                         show what codeshot found
codeshot version
```

`codeshot -- ls -la` runs `ls -la` under a pty, passes its output through to
your terminal as it happens, and stores `~/Codeshots/ls-la.png`. The command
gets a real terminal, so it keeps its colour and lays itself out for your
window's width; keystrokes reach it, and resizes follow it. codeshot exits
with the command's own status, so it is drop-in in front of anything a script
gates on, and the "Stored the shot in ..." line goes to stderr. A stdout
that is a file or a pipe gets what the command would have written without
codeshot in front of it: the colour and the `\r\n` it only produced because
codeshot gave it a terminal are taken out again on the way.

`name` is optional: given, it is honoured exactly (a bare name lands in the
gallery, anything with a path separator is used as given), and a file already
there goes to the Trash first, so it can be put back; omitted, codeshot
derives one from the command (`ls -la` becomes `ls-la.png`) and steps around
whatever is already in the gallery rather than overwriting it.

A pipeline works too, for one already written: `npm test | codeshot shot.png`
reads standard input and passes it through. It is the weaker half - a pipe is
not a terminal, so the command on the left has already dropped its colour,
and codeshot cannot know what that command was. `shim/codeshot.zsh` and
`shim/codeshot.bash` recover the command line from your shell's history;
`--command` says it by hand.

The shot does not have to become a file: `--clip` copies it to the
clipboard instead, and `--stdout` writes the PNG to standard output for
another program to read.

`render` is the same pipeline fed from a file instead: the raw bytes a
terminal would have received, escape sequences and all. They need to have
come through a pty (`\r\n`, not a bare `\n`) - codeshot's terminal emulator
treats a line feed exactly as a real terminal does, moving the cursor down
without returning it to column one, and a file assembled by plain shell
redirection (`cmd > file`) never passes through a pty to pick up that
translation. codeshot warns when it is handed a dump that looks redirected.

See [USAGE.md](USAGE.md) for worked examples, and `codeshot --help` for the
full flag list. What changed is in [CHANGELOG.md](CHANGELOG.md), and what is
still to come is in [plan.md](plan.md).

## Themes and fonts

codeshot reads `~/.config/ghostty/config` when there is one, so a shot looks
like the terminal it came from without being told to: its `theme`,
`font-size` and window padding are taken as defaults, under anything you pass
on the command line. `~/.config/codeshot/config` holds your own defaults in
the same `key = value` shape, and sits between the two.

Colours come from Ghostty theme files: `--theme codeshot-dark` and
`--theme codeshot-light` are embedded in the binary, a named theme resolves
against a local Ghostty installation's several hundred, and `--theme
/path/to/a/ghostty/theme` reads anyone else's, using the same handful of
keys (`background`, `foreground`, `cursor-color`, `palette`) Ghostty itself
understands. No Ghostty installation is required for the embedded two.

A theme file has to define at least `background` and `foreground`. Every
other key is ignored, so without that requirement any file at all would parse
as a theme — and the likeliest wrong file to reach for is a Ghostty *config*,
which usually just names a theme (`theme = tokyonight`) and holds no colours
of its own. Handing one over is an error rather than a shot of a window
with no colours in it.

Text is set in JetBrains Mono NL, embedded in the binary in all four
weights (regular, bold, italic, bold-italic), so a shot renders identically
on a machine that has never seen that font installed. `--font` draws with any
family installed here instead, and a rune the chosen family has no glyph for
is picked up by something that does - `中文` from a system face, `🎉` in
colour from Apple Color Emoji, which is a bitmap format `golang.org/x/image`
cannot read and codeshot decodes itself. A family with no italic cut is
sheared 12°, and one with no bold is double-struck.

`codeshot doctor` reports what was found: the config files read, the theme
directories, the font index and what a shot would be drawn with.

## Uninstall

```sh
rm ~/bin/codeshot                 # or ~/.local/bin/codeshot, or $PREFIX/bin
rm -r ~/.local/share/codeshot     # the shims, if install.sh downloaded them
rm -r ~/Library/Caches/codeshot   # the font index on macOS; ~/.cache/codeshot on Linux
rm -r ~/.config/codeshot          # your defaults, if you made any
```

Your shots are in `~/Codeshots`; delete that too if you want them gone. Take
the `source .../codeshot.zsh` line out of your shell rc if you added one.
