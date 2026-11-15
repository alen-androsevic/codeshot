# codeshot

codeshot turns a command and what it printed into a shot: a PNG of a terminal window.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/alen-androsevic/codeshot/main/install.sh | bash
```

```text
downloading https://github.com/alen-androsevic/codeshot/releases/latest/download/codeshot-darwin-arm64.tar.gz
installed /Users/you/bin/codeshot

to make `npm test | codeshot` show the command in the shot, add to your shell rc:
    source /Users/you/.local/share/codeshot/shim/codeshot.zsh    # or codeshot.bash

run `codeshot -- ls -la` to take your first shot, or `codeshot doctor` to see what codeshot found
```

Release binaries exist for `darwin/arm64`, `darwin/amd64`, `linux/amd64` and `linux/arm64`.

`install.sh` puts `codeshot` in the first of:

1. `$PREFIX/bin`, when `PREFIX` is set
2. `~/bin`, when it exists
3. `~/.local/bin`

The download puts the shims in `${XDG_DATA_HOME:-~/.local/share}/codeshot/shim/`.

From a clone with Go installed, `./install.sh` builds the checkout instead, with the `git describe` version stamped in. The shims stay in `shim/`.

```bash
PREFIX=/usr/local ./install.sh
```

```text
building codeshot v0.1.1
installed /usr/local/bin/codeshot

to make `npm test | codeshot` show the command in the shot, add to your shell rc:
    source /Users/you/codeshot/shim/codeshot.zsh    # or codeshot.bash

run `codeshot -- ls -la` to take your first shot, or `codeshot doctor` to see what codeshot found
```

## Use

Put `codeshot --` in front of a command:

```bash
codeshot -- ls shim
```

```text
codeshot.bash	codeshot.zsh	shim_test.go
Stored the shot in ~/Codeshots/ls-shim.png
```

| Command | Takes a shot of |
| --- | --- |
| `codeshot [name] [flags] -- <command> [args...]` | a command codeshot runs under a pty (the wrapper) |
| `<command> \| codeshot [name] [flags]` | standard input (pipe mode) |
| `codeshot render <file.ansi> [name] [flags]` | a saved dump |
| `codeshot themes [--json]` | nothing; lists the themes that resolve |
| `codeshot doctor [--json]` | nothing; reports the config files, theme directories, font index, font, gallery, clipboard and terminal size |
| `codeshot version` | nothing; prints the version |

[USAGE.md](USAGE.md) has more examples and a troubleshooting table.

### The wrapper

- codeshot's flags go before `--`. Everything after `--` belongs to the command, `--help` included.
- The command runs once, under a pty. Its output passes through to standard output as it prints.
- Keystrokes reach the command, and window resizes follow it.
- The `Stored the shot in …` line goes to standard error.
- When standard output is a file or a pipe, it gets no escape codes, and `\n` in place of `\r\n`.
- codeshot checks the theme and the font before it runs the command. A theme or font that does not resolve exits 1, and the command does not run.
- codeshot exits with the command's status. See [Exit codes](#exit-codes).

Pagers wait for a key under the wrapper. Turn them off for the command:

```bash
codeshot -- git --no-pager log --oneline -5
```

```text
Stored the shot in ~/Codeshots/git-no-pager-log-oneline-5.png
```

### Name the shot

| `name` | Shot goes to | Existing file there |
| --- | --- | --- |
| left out | the gallery, named after the command: `ls -la` → `ls-la.png` | kept; the shot gets `-2`, `-3`, …: `ls-la-2.png` |
| a bare name: `shot` or `shot.png` | the gallery: `~/Codeshots/shot.png` | moved to the Trash first |
| a name with a `/`: `./shot.png`, `~/Desktop/shot.png` | that path | moved to the Trash first |

`--out <name>` is the same as `name`. A name without `.png` gets `.png` added. The Trash is `~/.Trash` on macOS and `${XDG_DATA_HOME:-~/.local/share}/Trash` on Linux.

```bash
codeshot shot.png -- ls shim
codeshot shot.png -- ls shim
```

```text
codeshot.bash	codeshot.zsh	shim_test.go
Stored the shot in ~/Codeshots/shot.png
codeshot.bash	codeshot.zsh	shim_test.go
codeshot: moved the old ~/Codeshots/shot.png to ~/.Trash/shot.png
Stored the shot in ~/Codeshots/shot.png
```

`--clip` copies the shot to the clipboard and writes no file. It uses `osascript` on macOS, and `wl-copy` or `xclip` on Linux.

`--stdout` writes the PNG to standard output. The command's output goes to standard error.

```bash
codeshot --stdout -- go version > version.png
```

```text
go version go1.27.4 darwin/arm64
```

### Pipe mode

codeshot reads standard input and passes it through. With no `--command`, no shim and no `name`, the shot is `codeshot.png`.

```bash
ls shim | codeshot --command "ls shim"
```

```text
codeshot.bash
codeshot.zsh
shim_test.go
Stored the shot in ~/Codeshots/ls-shim.png
```

- A pipe carries standard output only. Add `2>&1` for a command that writes to standard error: `go vet ./... 2>&1 | codeshot --command "go vet ./..."`.
- Most tools print no colour into a pipe. Pass the tool's own flag, such as `--color=always`.
- The prompt line shows no command without `--command` or a shim.
- An empty standard input with no `--command` exits 2 and stores nothing.

### Shims

`shim/codeshot.zsh` and `shim/codeshot.bash` define a `codeshot` shell function. Source one in `~/.zshrc` or `~/.bashrc`:

```bash
source ~/.local/share/codeshot/shim/codeshot.zsh
```

| Mode | What the shim does |
| --- | --- |
| pipe mode | passes the command line from the shell's history as `--command`, in an interactive shell, unless `--command` is given |
| wrapper | expands one level of alias. The prompt line shows the alias name. An alias with a pipe or a redirect in it runs through the shell |

A shell function is not expanded. Without a shim, `codeshot -- ll` fails with `command not found`.

```bash
ls shim | codeshot shot.png    # prompt line says "ls shim"
codeshot -- ll                 # prompt line says "ll"
```

### Render a dump

A dump is a file of the raw bytes a terminal received, written through a pty. `script` writes one:

```bash
script -q shim.ansi ls -G shim                      # macOS
script -qc "ls --color=always shim" shim.ansi       # Linux
codeshot render shim.ansi --command "ls -G shim"
```

```text
Stored the shot in ~/Codeshots/ls-g-shim.png
```

A dump holds no command. Pass `--command`, or `--no-prompt` to leave out the prompt. A dump with more than one line and no `\r` gets this warning, and its lines step to the right in the shot:

```text
codeshot: red.ansi has more than one line and no carriage returns. A plain `cmd > file` redirect never passes through a pty, so the terminal driver never turns \n into \r\n, and this will stair-step: the cursor moves down but never back to column one between lines. Capture through a pty instead, e.g. `codeshot -- cmd`.
```

## Flags

| Flag | Default | Effect |
| --- | --- | --- |
| `--out <name>` | | where the shot goes; the same as `name` |
| `--stdout` | | write the PNG to standard output, and the command's output to standard error |
| `--clip` | | copy the shot to the clipboard, writing no file |
| `--command <text>` | the command that ran; none for `render` and pipe mode | the command line after the prompt |
| `--cwd <path>` | the current directory | the directory in the prompt |
| `--cols <n>` | the terminal's width, read from standard error, else `100` | terminal width |
| `--max-cols <n>` | `132` | cap on the width; `0` disables |
| `--min-cols <n>` | `80` | floor for the content-fit width; `0` disables |
| `--rows <n>` | `0` (all lines) | crop to this many lines |
| `--tail` | | crop from the bottom instead of the top |
| `--no-prompt` | | leave out the prompt and command lines |
| `--theme <name\|path>` | Ghostty's `theme`, else `codeshot-dark` | a name `codeshot themes` lists, or a Ghostty theme file |
| `--title <text>` | the command | window title |
| `--no-title` | | draw no title |
| `--controls <style>` | `macos` | window buttons: `macos`, `linux` or `none` |
| `--scale <n>` | `2` | pixel scale, at least `1` |
| `--padding <n>` | Ghostty's padding, else `14` | pixels around the grid |
| `--margin <n>` | `64`; `0` with `--no-shadow` | pixels around the window |
| `--no-shadow` | | draw no drop shadow |
| `--background <hex>` | transparent | fill the margin: `#1e1e2e` or `#abc` |
| `--radius <n>` | `10` | corner radius in pixels |
| `--font <family>` | Ghostty's `font-family`, else the embedded JetBrains Mono NL | a family `codeshot doctor` lists |
| `--font-size <n>` | Ghostty's `font-size`, else `13` | points |
| `--line-height <f>` | `1.0` | multiple of the font's own height |
| `--prompt <text>` | `{cwd}` in cyan, then a green `❯` on the next line | the prompt; `{cwd}` is the directory |
| `--config <path>` | see [Configuration](#configuration) | the defaults file |
| `--gallery <dir>` | `~/Codeshots` | where bare names are stored |
| `--debug` | | report every escape sequence the emulator ignored |

```bash
codeshot --theme codeshot-light --cols 80 --rows 12 --tail --no-shadow -- go vet ./...
```

```text
Stored the shot in ~/Codeshots/go-vet.png
```

## Configuration

Precedence, highest first:

```text
flags > codeshot config > Ghostty config > built-in defaults
```

```toml
# ~/.config/codeshot/config
theme = codeshot-light
```

```bash
codeshot --theme codeshot-dark -- ls shim    # drawn with codeshot-dark
```

### codeshot config

codeshot reads the first of:

1. `--config <path>`
2. `$XDG_CONFIG_HOME/codeshot/config`, when `XDG_CONFIG_HOME` is set
3. `~/.config/codeshot/config`

- One `key = value` per line. A key is a flag name without its dashes.
- A bare key means `true`: `no-shadow` and `no-shadow = true` are the same.
- Booleans accept `true`, `yes`, `on`, `1` and `false`, `no`, `off`, `0`.
- Lines starting with `#` are comments.
- An unknown key prints `codeshot: <path>: no setting called "<key>"`, and codeshot takes the shot.

```toml
theme = Catppuccin Mocha
font-size = 13
padding = 20
no-shadow
```

### Ghostty config

codeshot reads the first Ghostty config that exists:

1. `$XDG_CONFIG_HOME/ghostty/config`, when `XDG_CONFIG_HOME` is set; else `~/.config/ghostty/config`
2. `~/Library/Application Support/com.mitchellh.ghostty/config`

It reads five keys and ignores the rest:

| Key | Used as |
| --- | --- |
| `theme` | `--theme`. From `dark:<name>,light:<name>` it takes the dark name |
| `font-family` | `--font`. A family that is not installed prints a warning, and the embedded font is used |
| `font-size` | `--font-size` |
| `window-padding-x` | horizontal padding. From a pair such as `8,16` it takes the larger number |
| `window-padding-y` | vertical padding, read the same way |

`codeshot doctor` lists the files it read and the keys it took from each.

## Themes

| `--theme` | Resolves to |
| --- | --- |
| `codeshot-dark`, `codeshot-light` | the themes embedded in the binary |
| any other name | a file of that name in the first of `/Applications/Ghostty.app/Contents/Resources/ghostty/themes`, `/opt/homebrew/share/ghostty/themes`, `/usr/local/share/ghostty/themes`, `/usr/share/ghostty/themes` |
| a path | that Ghostty theme file |

A theme file sets `background`, `foreground`, `cursor-color` and `palette`. It must set `background` and `foreground`:

```bash
codeshot --theme ~/.config/ghostty/config -- ls shim
```

```text
codeshot: point --theme at a Ghostty theme file, not a config: "config" has no background or foreground colour
```

```bash
codeshot themes | head -4
```

```text
codeshot-dark
codeshot-light
0x96f
12-bit Rainbow
```

## Fonts

- The embedded font is JetBrains Mono NL, in regular, bold, italic and bold-italic.
- `--font <family>` draws with a family installed on the machine, or with `JetBrains Mono NL`. `codeshot doctor` lists the installed ones.
- A family from `--font` or from `font` in the codeshot config that is not installed exits 1: `codeshot: pick a font family `codeshot doctor` lists; "Nosuch" is not installed`.
- A rune the family has no glyph for is drawn with the platform's fallback families. `codeshot doctor` lists them.
- A family with no italic is sheared 12°. A family with no bold is double-struck.
- Colour emoji are drawn from Apple Color Emoji on macOS. On Linux they draw as boxes.
- Nerd Font icons need a Nerd Font installed, such as `brew install --cask font-symbols-only-nerd-font`. codeshot finds it without `--font`.
- The font index is cached in `~/Library/Caches/codeshot/fonts.json` on macOS and `${XDG_CACHE_HOME:-~/.cache}/codeshot/fonts.json` on Linux.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | the shot was stored, or `themes`, `doctor` or `version` ran |
| the command's status | wrapper: the command exited non-zero. This wins over a failed shot |
| `1` | the shot failed: a theme, font or dump that does not exist, or a write that failed |
| `2` | the arguments are wrong: no command after `--`, an empty pipe, a bad flag value |

```bash
codeshot -- false; echo $?
```

```text
Stored the shot in ~/Codeshots/false.png
1
```

```bash
codeshot --rows x -- ls; echo $?
```

```text
codeshot: give --rows a whole number; "x" is not one
2
```

## Uninstall

```bash
rm ~/bin/codeshot                 # or ~/.local/bin/codeshot, or $PREFIX/bin/codeshot
rm -r ~/.local/share/codeshot     # the shims, if install.sh downloaded them
rm -r ~/Library/Caches/codeshot   # the font index on macOS; ~/.cache/codeshot on Linux
rm -r ~/.config/codeshot          # the codeshot config
rm -r ~/Codeshots                 # the gallery
```

Remove the `source …/codeshot.zsh` line from your shell rc.

## License

GPL-3.0. See [LICENSE](LICENSE). The embedded JetBrains Mono NL is under the SIL Open Font License, in [internal/adapters/fonts/assets/OFL.txt](internal/adapters/fonts/assets/OFL.txt).
