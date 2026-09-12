#!/usr/bin/env bash
# Build codeshot and put it somewhere on your PATH, with the version stamped
# in so that `codeshot version` says something more useful than "dev".
#
#   ./install.sh                  # into ~/bin, or ~/.local/bin if that is where you keep things
#   PREFIX=/usr/local ./install.sh   # into /usr/local/bin
#
# The only thing this does that `go build` does not is the version stamp and
# choosing a directory, so `go build -o codeshot ./cmd/codeshot` remains a
# perfectly good way to install codeshot by hand.
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
	echo "install.sh: go is not installed; codeshot is built from source" >&2
	exit 1
fi

# git describe is the version when this is a checkout with tags; a tarball
# without them still builds, it just says so.
version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

if [[ -n "${PREFIX:-}" ]]; then
	bindir="$PREFIX/bin"
elif [[ -d "$HOME/bin" ]]; then
	bindir="$HOME/bin"
else
	bindir="$HOME/.local/bin"
fi
mkdir -p "$bindir"

echo "building codeshot $version"
go build -ldflags "-X codeshot/internal/cli.Version=$version" -o "$bindir/codeshot" ./cmd/codeshot
echo "installed $bindir/codeshot"

case ":$PATH:" in
*":$bindir:"*) ;;
*) echo "note: $bindir is not on your PATH" >&2 ;;
esac

# The shims are optional and only matter to people who pipe into codeshot,
# so they are pointed at rather than installed into anyone's shell.
echo
echo "to make \`npm test | codeshot\` show the command in the picture, add to your shell rc:"
echo "    source $PWD/shim/codeshot.zsh    # or codeshot.bash"
echo
echo "run \`codeshot doctor\` to see what codeshot found on this machine"
