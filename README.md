# codeshot

codeshot turns a command and the output it printed into a PNG that looks like
a terminal window: rounded corners, traffic lights, a soft drop shadow, and
text in whatever colours the terminal actually showed.

codeshot never re-runs your command. It has no idea how to reproduce a build
failure, a flaky test, or a program that reads a password from stdin, so it
doesn't try. You decide up front: put `codeshot --` in front of a command, and
codeshot runs it once, under a terminal it owns, and pictures exactly what it
printed.

## Usage

```
codeshot [name] [flags] -- <command> [args...]   run a command and picture it
codeshot render <file.ansi> [name] [flags]       render a saved ANSI dump
codeshot themes                                  list the embedded themes
codeshot version
```

`codeshot -- ls -la` runs `ls -la` under a pty, passes its output through to
your terminal as it happens, and stores `~/Codeshots/ls-la.png`. The command
gets a real terminal, so it keeps its colour and lays itself out for your
window's width; keystrokes reach it, and resizes follow it. codeshot exits
with the command's own status, so it is drop-in in front of anything a script
gates on, and the "Stored codeshot in ..." line goes to stderr so a
redirected stdout holds exactly what the command printed.

`name` is optional: given, it is honoured exactly (a bare name lands in the
gallery, anything with a path separator is used as given); omitted, codeshot
derives one from the command (`ls -la` becomes `ls-la.png`) and steps around
whatever is already in the gallery rather than overwriting it.

`render` is the same pipeline fed from a file instead: the raw bytes a
terminal would have received, escape sequences and all. They need to have
come through a pty (`\r\n`, not a bare `\n`) - codeshot's terminal emulator
treats a line feed exactly as a real terminal does, moving the cursor down
without returning it to column one, and a file assembled by plain shell
redirection (`cmd > file`) never passes through a pty to pick up that
translation. codeshot warns when it is handed a dump that looks redirected.

See [USAGE.md](USAGE.md) for worked examples, and `codeshot --help` for the
full flag list. What is still to come is in [plan.md](plan.md).

## Themes and fonts

Colours come from Ghostty theme files: `--theme codeshot-dark` and
`--theme codeshot-light` are embedded in the binary, and `--theme
/path/to/a/ghostty/theme` reads anyone else's, using the same handful of
keys (`background`, `foreground`, `cursor-color`, `palette`) Ghostty itself
understands. No Ghostty installation is required either way.

A theme file has to define at least `background` and `foreground`. Every
other key is ignored, so without that requirement any file at all would parse
as a theme — and the likeliest wrong file to reach for is a Ghostty *config*,
which usually just names a theme (`theme = tokyonight`) and holds no colours
of its own. Handing one over is an error rather than a picture of a window
with no colours in it.

Text is set in JetBrains Mono NL, embedded in the binary in all four
weights (regular, bold, italic, bold-italic), so a shot renders identically
on a machine that has never seen that font installed.
