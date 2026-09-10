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

Flags for `render`:

```
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
```

A dump's lines need to be terminated the way a real pty would have sent them
(`\r\n`, not a bare `\n`) - codeshot's terminal emulator treats a line feed
exactly as a real terminal does, moving the cursor down without returning it
to column one, and a file assembled by plain shell redirection (`cmd > file`)
never passes through a pty to pick up that translation. `script` (or the
wrapper below) captures it correctly.

**Coming in phase 2**: a wrapper that runs your command for you and captures
it live - `codeshot shot.png -- npm test` - so you never have to make your
own dump. This phase only has `render`, which is the half of the pipeline
that exists without a pty in the picture.

## Themes and fonts

Colours come from Ghostty theme files: `--theme codeshot-dark` and
`--theme codeshot-light` are embedded in the binary, and `--theme
/path/to/a/ghostty/theme` reads anyone else's, using the same handful of
keys (`background`, `foreground`, `cursor-color`, `palette`) Ghostty itself
understands. No Ghostty installation is required either way.

Text is set in JetBrains Mono NL, embedded in the binary in all four
weights (regular, bold, italic, bold-italic), so a shot renders identically
on a machine that has never seen that font installed.
