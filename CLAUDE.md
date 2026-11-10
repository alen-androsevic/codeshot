# codeshot

A Go CLI that turns a command and what it printed into a PNG of a terminal
window. Read `README.md` for what it does, `docs/design.md` for why, and
`docs/adr/` for decisions already settled. `plan.md` is what is still owed.

## Working here

- `./Taskfile test` runs vet and every test with `-race`; the CLI tests take a
  few minutes under the race detector. `go test ./...` is quicker while
  iterating.
- `./Taskfile golden` rewrites the reference images after a deliberate change
  to rendering. Diff old against new first: only what you meant to change
  should move.
- `./Taskfile build` builds `./codeshot`; `./install.sh` installs it.
- Layout: `internal/domain` is the pure core (grid, theme, frame), `internal/app`
  the one pipeline, `internal/adapters/*` everything that touches the machine,
  and `internal/cli` the only place that knows flags exist and decides exit
  codes.
- Tests never touch the person's real gallery, clipboard, Ghostty config or
  font cache: `HOME` is moved aside, and those are variables a test replaces.
- Comments say why, in full sentences. Match that when you add code.
- Messages go to stderr and start with `codeshot: `; data goes to stdout.
  An error leads with what to do.
- Pushing a `v*` tag runs `.github/workflows/release.yml`, which publishes
  binaries. Add the version's section to `CHANGELOG.md` before tagging.

## Glossary

- **shot:** the PNG codeshot makes. Not "picture", "image" or "codeshot".
- **gallery:** where a bare name is stored, `~/Codeshots` by default.
- **wrapper:** `codeshot -- <command>`, which runs the command under a pty.
- **pipe mode:** `<command> | codeshot`, which reads standard input.
- **dump:** a saved file of raw terminal bytes, for `codeshot render`.
- **shim:** `shim/codeshot.zsh` or `.bash`, sourced into a shell.

## MIG

Skin: product (tokens in `internal/adapters/theme/assets/codeshot-dark.conf` and `codeshot-light.conf`)

Exceptions:
