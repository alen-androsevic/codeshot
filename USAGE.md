# codeshot examples

Commands for common shots, and fixes for common problems. Install steps, flags, configuration and exit codes are in [README.md](README.md).

## Examples

Each comment is the line codeshot prints on standard error.

```bash
# Name the shot
codeshot shot.png -- go test ./...                  # Stored the shot in ~/Codeshots/shot.png
codeshot --out shot.png -- go test ./...            # Stored the shot in ~/Codeshots/shot.png
codeshot ~/Desktop/shot.png -- go test ./...        # Stored the shot in ~/Desktop/shot.png

# Send it somewhere other than a file
codeshot --clip -- git status                       # Stored the shot in the clipboard
codeshot --stdout -- go version > version.png

# Crop to the first 12 lines, or the last 12
codeshot --rows 12 -- go vet ./...                  # Stored the shot in ~/Codeshots/go-vet.png
codeshot --rows 12 --tail -- go vet ./...           # Stored the shot in ~/Codeshots/go-vet-2.png

# Pick a theme
codeshot --theme codeshot-light -- git status --short
codeshot --theme "Catppuccin Latte" -- git status --short
codeshot --theme ~/.config/ghostty/themes/tokyonight -- git status --short
codeshot themes | grep -i tokyo

# Change the window
codeshot --background '#1e1e2e' -- git --no-pager log --oneline -5
codeshot --no-shadow -- git diff --stat
codeshot --controls none --no-title -- go version   # Stored the shot in ~/Codeshots/go-version.png
codeshot --controls linux -- uname -a               # Stored the shot in ~/Codeshots/uname-a.png
codeshot --no-prompt -- go version
codeshot --cols 80 -- git --no-pager log --graph --oneline

# Pick a font installed on the machine
codeshot --font Menlo -- git status --short

# Show a different command line than the one that ran
codeshot --command "curl https://api.github.com/user" -- curl -H "Authorization: Bearer $GH_TOKEN" https://api.github.com/user
# Stored the shot in ~/Codeshots/curl-https-api-github-com-user.png

# Pipe mode, with standard error included
go vet ./... 2>&1 | codeshot --command "go vet ./..."

# A saved dump
codeshot render shim.ansi --command "ls -G shim"    # Stored the shot in ~/Codeshots/ls-g-shim.png
```

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `ls` has no colour on macOS | Use `ls -G`, or `export CLICOLOR=1` |
| The wrapper waits and the shot never comes | The command opened a pager. Use `git --no-pager …`, or set `PAGER=cat` |
| Piped output is grey | Use the wrapper, or the tool's `--color=always` |
| A piped `ls` or `eza` lost its colour and icons | Use the wrapper, or `--color=always --icons=always` |
| Piped output has no prompt line | Pass `--command`, or source a [shim](README.md#shims) |
| Most of a piped run is missing from the shot | The command writes to standard error. Use `cmd 2>&1 \| codeshot`, or the wrapper |
| Wrapping differs from your terminal | Standard error is redirected, so the width is `100`. Pass `--cols` |
| `install wl-clipboard (Wayland) or xclip (X11) for --clip` | Install one of them. macOS needs neither |
| `--stdout` printed binary in the terminal | Redirect it: `codeshot --stdout -- cmd > shot.png` |
| `render`: lines step to the right | The dump was not written through a pty. Use `codeshot -- cmd`, or `script` |
| `no background or foreground colour` | `--theme` points at a Ghostty config. Point it at a theme file |
| Emoji are boxes on Linux | Colour emoji draw on macOS only |
| Nerd Font icons are boxes | Install one: `brew install --cask font-symbols-only-nerd-font` |
| `codeshot -- ll` says `command not found` | Source a [shim](README.md#shims) |
| A glyph is a box and the font is installed | Run `codeshot doctor` to see the font index and the font a shot uses |
| The wrong theme or size | Run `codeshot doctor` to see which config file sets it |
| An escape sequence draws nothing | Run with `--debug` to list the sequences the emulator ignored |
