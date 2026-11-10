#!/bin/sh
# enza-seal installer for macOS and Linux (no admin rights needed).
#   curl -fsSL https://raw.githubusercontent.com/kirollos-rafik/enza-seal/main/install.sh | sh
#
# Downloads the release for this computer, checks its SHA-256 against
# checksums.txt, and installs enza-seal to ~/.local/bin.
#
# Optional settings (environment variables):
#   ENZA_SEAL_VERSION      install a specific version, e.g. 1.0.0 (default: latest)
#   ENZA_SEAL_INSTALL_DIR  install somewhere else
#   ENZA_SEAL_BASE_URL     download from another location (a URL or a folder)
#   ENZA_SEAL_NO_PATH=1    do not edit your shell profile

set -eu

repo="kirollos-rafik/enza-seal"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "Unsupported processor: $(uname -m)" >&2; exit 1 ;;
esac

if [ -n "${ENZA_SEAL_BASE_URL:-}" ]; then
  base="$ENZA_SEAL_BASE_URL"
elif [ -n "${ENZA_SEAL_VERSION:-}" ]; then
  v="${ENZA_SEAL_VERSION#v}"
  base="https://github.com/$repo/releases/download/v$v"
else
  base="https://github.com/$repo/releases/latest/download"
fi
install_dir="${ENZA_SEAL_INSTALL_DIR:-$HOME/.local/bin}"
asset="enza-seal_${os}_${arch}.tar.gz"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

fetch() {
  case "$base" in
    http://*|https://*) curl -fsSL "$base/$1" -o "$work/$1" ;;
    *) cp "$base/$1" "$work/$1" ;;
  esac
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  else shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

echo "Downloading enza-seal for $os/$arch ..."
fetch "$asset"
fetch checksums.txt

expected="$(grep " $asset\$" "$work/checksums.txt" | head -n 1 | cut -d' ' -f1)"
[ -n "$expected" ] || { echo "No checksum listed for $asset. Stopping." >&2; exit 1; }
actual="$(sha256_of "$work/$asset")"
if [ "$expected" != "$actual" ]; then
  echo "Checksum does not match. The download may be damaged or tampered with. Stopping." >&2
  exit 1
fi
echo "Checksum OK"

tar -xzf "$work/$asset" -C "$work" enza-seal
mkdir -p "$install_dir"
mv -f "$work/enza-seal" "$install_dir/enza-seal"
chmod +x "$install_dir/enza-seal"
echo "Installed to $install_dir"

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    if [ "${ENZA_SEAL_NO_PATH:-}" != "1" ]; then
      case "${SHELL:-}" in
        */zsh)  profile="$HOME/.zprofile" ;;
        */bash) profile="$HOME/.bash_profile"; [ -f "$HOME/.bash_profile" ] || profile="$HOME/.profile" ;;
        *)      profile="$HOME/.profile" ;;
      esac
      printf '\nexport PATH="%s:$PATH"\n' "$install_dir" >> "$profile"
      echo "Added $install_dir to your PATH in $profile. Open a new terminal window to use it."
    fi
    ;;
esac

echo
"$install_dir/enza-seal" --version
echo "enza-seal installed. Run: enza-seal init"
