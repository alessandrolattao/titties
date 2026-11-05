#!/bin/sh
# Installs titties from the latest GitHub release.
#   curl -fsSL https://raw.githubusercontent.com/alessandrolattao/titties/main/install.sh | sh
# TITTIES_VERSION=v0.1.0 picks a release, TITTIES_BIN_DIR changes where the command goes (default ~/.local/bin),
# TITTIES_APP_DIR where the binary and its libraries are unpacked.
set -eu

repo=alessandrolattao/titties
bin_dir=${TITTIES_BIN_DIR:-$HOME/.local/bin}
app_dir=${TITTIES_APP_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/titties/app}

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) echo "titties: unsupported system $(uname -s); on Windows download the zip from https://github.com/$repo/releases" >&2 && exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) echo "titties: unsupported architecture $(uname -m)" >&2 && exit 1 ;;
esac

version=${TITTIES_VERSION:-}
if [ -z "$version" ]; then
	# The latest release page redirects to .../releases/tag/<version>.
	version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest" | sed 's#.*/##')
fi
case "$version" in
v*) ;;
*) echo "titties: no release found" >&2 && exit 1 ;;
esac

name="titties-${version}-${os}-${arch}"
url="https://github.com/$repo/releases/download/${version}/${name}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading titties $version for $os-$arch..."
curl -fsSL "$url" -o "$tmp/titties.tar.gz"
tar -xzf "$tmp/titties.tar.gz" -C "$tmp"

mkdir -p "$(dirname "$app_dir")" "$bin_dir"
rm -rf "$app_dir"
mv "$tmp/$name" "$app_dir"
ln -sf "$app_dir/titties" "$bin_dir/titties"

echo "installed $("$bin_dir/titties" version) in $app_dir"
case ":$PATH:" in
*":$bin_dir:"*) ;;
*) echo "add $bin_dir to your PATH to run titties" ;;
esac
echo 'try: titties say "ciao, oggi è una bella giornata"'
