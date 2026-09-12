#!/usr/bin/env sh
# DataTF installer for Linux and macOS.
#   curl -fsSL https://datatf.io/install.sh | sh
# Installs the latest release into ~/.local/bin or $DATATF_INSTALL_DIR.
# Set DATATF_VERSION, such as 1.0.0, to pin a release.
set -eu

repo="536tech/datatf"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
linux | darwin) ;;
*)
  echo "unsupported operating system: $os" >&2
  exit 1
  ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
  echo "unsupported architecture: $(uname -m)" >&2
  exit 1
  ;;
esac

version="${DATATF_VERSION:-}"
if [ -n "$version" ]; then
  base="https://github.com/$repo/releases/download/v${version#v}"
else
  base="https://github.com/$repo/releases/latest/download"
fi
asset="datatf_${os}_${arch}.tar.gz"
dest="${DATATF_INSTALL_DIR:-$HOME/.local/bin}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset..."
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o "$tmp/$asset" "$base/$asset"
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
expected="$(awk -v asset="$asset" '$2 == asset { print $1 }' "$tmp/checksums.txt")"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$asset" | cut -d' ' -f1)"
else
  actual="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"
fi
if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
  echo "checksum mismatch for $asset" >&2
  exit 1
fi
tar -xzf "$tmp/$asset" -C "$tmp" datatf
mkdir -p "$dest"
install -m 0755 "$tmp/datatf" "$dest/datatf"
echo "Installed $dest/datatf"
case ":$PATH:" in
*":$dest:"*) ;;
*) echo "Add $dest to your PATH." ;;
esac
"$dest/datatf" version
