# codeshot

codeshot turns a command and the output it printed into a PNG that looks like
a terminal window: rounded corners, traffic lights, a soft drop shadow, and
text in whatever colours the terminal actually showed.

codeshot never re-runs your command. It has no idea how to reproduce a build
failure, a flaky test, or a program that reads a password from stdin, so it
doesn't try. You tell it up front what happened - a saved dump of raw ANSI
bytes, plus (optionally) the command line and working directory to show above
it - and codeshot renders exactly that.

## Usage

```
codeshot render <file.ansi> [name] [flags]   render a saved ANSI dump
codeshot themes                              list the embedded themes
codeshot version
```

`file.ansi` is the raw bytes a terminal would have received - escape
sequences and all. `name` is optional: given, it is honoured exactly (a bare
name lands in the gallery, anything with a path separator is used as given);
omitted, codeshot derives one from the command (`ls -la` becomes
`ls-la.png`) and steps around whatever is already in the gallery rather than
overwriting it.

See [USAGE.md](USAGE.md) for worked examples, and `codeshot render --help`
for the full flag list.

A dump's lines need to be terminated the way a real pty would have sent them
(`\r\n`, not a bare `\n`) - codeshot's terminal emulator treats a line feed
exactly as a real terminal does, moving the cursor down without returning it
to column one, and a file assembled by plain shell redirection (`cmd > file`)
never passes through a pty to pick up that translation. `script` captures it
correctly, and codeshot warns when it is handed a dump that looks redirected.

**Coming in phase 2**: a wrapper that runs your command for you and captures
it live - `codeshot shot.png -- npm test` - so you never have to make your
own dump. See [plan.md](plan.md).

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
