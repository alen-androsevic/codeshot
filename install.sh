#!/usr/bin/env bash
# Put codeshot on your PATH, with the version stamped in so that `codeshot
# version` says something more useful than "dev".
#
#   curl -fsSL https://raw.githubusercontent.com/alen-androsevic/codeshot/main/install.sh | bash
#   ./install.sh                     # from a checkout: builds with Go if you have it
#   PREFIX=/usr/local ./install.sh   # into /usr/local/bin
#
# From a checkout with Go installed it builds what you have checked out.
# Anywhere else it downloads the latest release's binary, so neither Go nor a
# clone is needed. The shims go next to the binary's data, in
# ${XDG_DATA_HOME:-~/.local/share}/codeshot/shim, when downloaded.
set -euo pipefail

repo="alen-androsevic/codeshot"

if [[ -n "${PREFIX:-}" ]]; then
	bindir="$PREFIX/bin"
elif [[ -d "$HOME/bin" ]]; then
	bindir="$HOME/bin"
else
	bindir="$HOME/.local/bin"
fi
mkdir -p "$bindir"

# BASH_SOURCE is empty when the script arrives on a pipe, which is how the
# curl line runs it; then there is no checkout to build from.
here=""
if [[ -n "${BASH_SOURCE[0]:-}" && -f "$(dirname "${BASH_SOURCE[0]}")/go.mod" ]]; then
	here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

if [[ -n "$here" ]] && command -v go >/dev/null 2>&1; then
	cd "$here"
	# git describe is the version when this is a checkout with tags; a tarball
	# without them still builds, it just says so.
	version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
	echo "building codeshot $version" >&2
	go build -ldflags "-X codeshot/internal/cli.Version=$version" -o "$bindir/codeshot" ./cmd/codeshot
	shimdir="$here/shim"
else
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m)"
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*)
		echo "install.sh: there is no release binary for $arch; install Go and run ./install.sh from a clone" >&2
		exit 1
		;;
	esac
	case "$os" in
	darwin | linux) ;;
	*)
		echo "install.sh: there is no release binary for $os; install Go and run ./install.sh from a clone" >&2
		exit 1
		;;
	esac
	name="codeshot-$os-$arch"
	url="https://github.com/$repo/releases/latest/download/$name.tar.gz"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	echo "downloading $url" >&2
	if ! curl -fsSL "$url" -o "$tmp/$name.tar.gz"; then
		echo "install.sh: no release to download yet; install Go, clone https://github.com/$repo and run ./install.sh there" >&2
		exit 1
	fi
	tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
	install -m 0755 "$tmp/$name/codeshot" "$bindir/codeshot"
	shimdir="${XDG_DATA_HOME:-$HOME/.local/share}/codeshot/shim"
	mkdir -p "$shimdir"
	cp "$tmp/$name/shim/codeshot.zsh" "$tmp/$name/shim/codeshot.bash" "$shimdir/"
fi
echo "installed $bindir/codeshot" >&2

case ":$PATH:" in
*":$bindir:"*) ;;
*) echo "note: $bindir is not on your PATH" >&2 ;;
esac

# The shims are optional and only matter to people who pipe into codeshot,
# so they are pointed at rather than installed into anyone's shell.
echo >&2
echo "to make \`npm test | codeshot\` show the command in the shot, add to your shell rc:" >&2
echo "    source $shimdir/codeshot.zsh    # or codeshot.bash" >&2
echo >&2
echo "run \`codeshot -- ls -la\` to take your first shot, or \`codeshot doctor\` to see what codeshot found" >&2
