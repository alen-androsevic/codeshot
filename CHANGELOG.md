# Changelog

codeshot follows semantic versioning. Newest first. Each breaking change has a
line saying what to do instead.

## 0.1.1 (2026-11-10)

- Windows is no longer a build target. It never had a pty, so codeshot did
  not build there; its font-directory and clipboard files are gone too.
- The release workflow uses `actions/checkout@v7` and `actions/setup-go@v7`.

## 0.1.0 (2026-11-10)

The first release.

- `codeshot -- <command>` runs a command under a pty, passes its output
  through, and stores a shot of it in `~/Codeshots`. codeshot exits with the
  command's own status.
- `<command> | codeshot` takes a shot of a pipe, and `codeshot render` takes
  one of a saved dump. `--clip` and `--stdout` send the shot somewhere other
  than a file.
- Shots look like your terminal: Ghostty's theme, font size and padding are
  read, any installed font can be used, and colour emoji are drawn on macOS.
- `codeshot doctor` and `codeshot themes` report what codeshot found, both
  with `--json`.
- An explicit name moves the file it replaces to the Trash first.
- A stdout that is a file or a pipe gets no escape codes, the same as from
  the command on its own.
- An empty pipe with no command known exits 2 and stores nothing.
- The embedded themes meet 4.5:1 contrast for text, the window title too.
- `install.sh` downloads a release binary when there is no Go to build with.
