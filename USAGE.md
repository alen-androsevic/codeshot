# codeshot — usage

Turn a command's output into a PNG that looks like a terminal window.

## Install

```sh
./install.sh          # builds with the version stamped in, into ~/bin
```

or by hand, without the version:

```sh
go build -o codeshot ./cmd/codeshot && mv codeshot ~/bin/
```

## Take a shot

Put `codeshot --` in front of the command:

```sh
codeshot -- ls -la
```

The command runs exactly as it would bare, output live in your terminal, and
the picture lands in `~/Codeshots/ls-la.png`. codeshot exits with the
command's own status, so it's safe in front of anything a script checks.

## Examples

```sh
# Name the file yourself
codeshot shot.png -- npm test
codeshot --out shot.png -- npm test        # the same thing

# Write somewhere specific (any name with a / is a path)
codeshot ~/Desktop/shot.png -- npm test

# Straight to the clipboard, no file at all
codeshot --clip -- git status

# Straight to a pipeline
codeshot --stdout -- git status > shot.png
codeshot --stdout -- git status | pngquant - > small.png

# First 12 lines only, or the last 12
codeshot --rows 12 -- npm test
codeshot --rows 12 --tail -- npm test

# Light theme, or any Ghostty theme file
codeshot --theme codeshot-light -- git status
codeshot --theme ~/.config/ghostty/themes/tokyonight -- git status
codeshot themes                                # what's built in

# Solid background instead of transparency
codeshot --background '#1e1e2e' -- git log --oneline -5

# Tight crop, no shadow (margin drops to 0 automatically)
codeshot --no-shadow -- git diff --stat

# Just the text, no window chrome
codeshot --controls none --no-title -- cargo build

# Linux-style buttons on a Mac, or the reverse
codeshot --controls linux -- uname -a

# No prompt line at all
codeshot --no-prompt -- fortune

# A fixed width, whatever your window is
codeshot --cols 80 -- git log --graph

# Show a different command line than the one that ran - keeps a secret
# out of the picture and out of the filename
codeshot --command "deploy" -- deploy --token="$TOKEN"
```

codeshot's flags go before the `--`. Everything after it belongs to the
command, `--help` included.

Run `codeshot --help` for every flag.

## Piping into codeshot

If the pipeline is already written, codeshot reads standard input:

```sh
npm test | codeshot shot.png --command "npm test"
```

The output still passes through, so you see the run as it happens.

Two things are worse this way, and neither is fixable from codeshot's side.
A pipe is not a terminal, so most tools turn their colour **off** before
codeshot sees a single byte — pass `--color=always` (or the tool's
equivalent) if it has one. And codeshot cannot know what the command was, so
the prompt line is empty unless you pass `--command`.

The shims fix the second one by reading the command from your shell's
history:

```sh
source /path/to/codeshot/shim/codeshot.zsh    # or .bash, from ~/.zshrc
npm test | codeshot shot.png                  # prompt line says "npm test"
```

They only work where you type — a non-interactive shell keeps no history —
and they stand aside for `codeshot -- cmd` and for an explicit `--command`.

The wrapper has neither problem. Prefer it when you can.

## Make it look like your terminal

codeshot reads `~/.config/ghostty/config` when you have one and takes the
four settings it can honour: `theme`, `font-size`, `window-padding-x` and
`window-padding-y`. A shot from a Catppuccin Mocha terminal comes out in
Catppuccin Mocha with nothing passed on the command line. A `theme` naming
both a light and a dark one gets the dark one, so the same command gives the
same picture whatever time of day it is.

Named themes resolve against a Ghostty installation, so any of the several
hundred it ships works:

```sh
codeshot --theme "Catppuccin Latte" -- git status
codeshot themes | grep -i tokyo          # what resolves here
```

Fonts come from the machine:

```sh
codeshot --font Menlo -- git status
codeshot doctor                          # what is installed, and what a shot would use
```

A rune your font has no glyph for is drawn by something that does, so `中文`
and `→` arrive even from a font that never heard of them. On macOS colour
emoji are decoded out of Apple Color Emoji and composited in, so `🎉` is a
picture rather than a box; on Linux and Windows emoji are still tofu, because
those platforms' emoji fonts use formats codeshot does not read yet.

### Your own defaults

`~/.config/codeshot/config` holds the flags you would otherwise always pass.
Every key is a flag name without its dashes:

```
theme = Catppuccin Mocha
font = JetBrains Mono
font-size = 13
padding = 20
no-shadow
```

Flags beat that file, it beats Ghostty's config, and Ghostty's beats the
built-in defaults. `--config <path>` reads somewhere else instead, and
`codeshot doctor` says which files were read and what they changed.

## Rendering a saved dump

If you already have the raw bytes of a terminal session, render them
without running anything:

```sh
codeshot render out.ansi --command "ls -la"
```

The dump has to have come through a pty, e.g. from `script`
(`script -q out.ansi ls -G -la` on macOS,
`script -qc "ls --color=always -la" out.ansi` on Linux). A plain `>`
redirect loses the carriage returns and the picture stair-steps; codeshot
warns when a dump looks like that. `--command` is what shows after the
prompt: a dump holds only output, so pass it or use `--no-prompt`.

## Gotchas

| Symptom | Fix |
| --- | --- |
| `ls` has no colour on macOS | That's `ls`, not codeshot: use `ls -G`, or `export CLICOLOR=1` |
| Piped output is grey | A pipe is not a terminal, so the tool dropped its colour. Use the wrapper, or `--color=always` |
| Piped output has no prompt line | codeshot can't know the command: pass `--command`, or source a shim |
| Wrapping differs from your terminal | codeshot sizes from stderr; if that's redirected it falls back to 100 columns. Pass `--cols` |
| `--clip` says no clipboard tool | Linux needs `wl-clipboard` (Wayland) or `xclip` (X11); macOS needs nothing |
| `--stdout` printed junk in my terminal | That's the PNG. Redirect it: `codeshot --stdout -- cmd > shot.png` |
| `render`: lines march right across the image | The dump didn't go through a pty; use `codeshot -- cmd` instead |
| `no background or foreground colour` | You pointed `--theme` at a Ghostty *config*, not a theme file |
| Emoji are boxes on Linux or Windows | Only Apple's sbix format is read so far; on macOS they render in colour |
| Nerd Font icons are boxes | Install a Nerd Font and name it: `--font "JetBrainsMono Nerd Font"` |
| A glyph is a box and you have the font | `codeshot doctor` lists what codeshot found, and which font it would use |
| The wrong theme, or the wrong size | Something in `~/.config/codeshot/config` or Ghostty's config is supplying it; `codeshot doctor` says which |

`--debug` lists every escape sequence the renderer ignored.
