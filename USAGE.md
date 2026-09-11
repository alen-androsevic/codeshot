# codeshot — usage

Turn a command's output into a PNG that looks like a terminal window.

## Install

```sh
go build -o codeshot ./cmd/codeshot && mv codeshot ~/bin/
```

## The two-step

codeshot renders raw terminal bytes, so first capture them through a pty,
then render. A plain `>` redirect loses the carriage returns and the picture
stair-steps.

```sh
script -q out.ansi ls -G -la                   # capture (macOS)
script -qc "ls --color=always -la" out.ansi    # capture (Linux)
codeshot render out.ansi --command "ls -la"    # render
```

Output lands in `~/Codeshots/ls-la.png`.

## Examples

```sh
# Name the file yourself
codeshot render out.ansi shot.png --command "npm test"

# Write somewhere specific (any name with a / is a path)
codeshot render out.ansi ~/Desktop/shot.png --command "npm test"

# First 12 lines only, or the last 12
codeshot render out.ansi --command "npm test" --rows 12
codeshot render out.ansi --command "npm test" --rows 12 --tail

# Light theme, or any Ghostty theme file
codeshot render out.ansi --theme codeshot-light
codeshot render out.ansi --theme ~/.config/ghostty/themes/tokyonight
codeshot themes                                # what's built in

# Solid background instead of transparency
codeshot render out.ansi --background '#1e1e2e'

# Tight crop, no shadow (margin drops to 0 automatically)
codeshot render out.ansi --no-shadow

# Just the text, no window chrome
codeshot render out.ansi --controls none --no-title

# Linux-style buttons on a Mac, or the reverse
codeshot render out.ansi --controls linux

# No prompt line at all
codeshot render out.ansi --no-prompt

# Match the width the dump was captured at
codeshot render out.ansi --command "git log" --cols 120
```

`--command` is what shows after the prompt. codeshot can't know it — the dump
holds only output — so pass it or use `--no-prompt`.

Run `codeshot render --help` for every flag.

## Gotchas

| Symptom | Fix |
| --- | --- |
| Lines march right across the image | Capture with `script`, not `>` |
| Everything is grey | Same — no pty means the program turned colour off |
| Wrapping differs from your terminal | Pass the real width with `--cols` |
| `no background or foreground colour` | You pointed `--theme` at a Ghostty *config*, not a theme file |
| Emoji and Nerd Font icons are boxes | Known limitation — the embedded font can't render them |

`--debug` lists every escape sequence the renderer ignored.
