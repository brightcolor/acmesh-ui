#!/bin/sh
# acmesh-ui installer.
#
#   curl -fsSL https://raw.githubusercontent.com/brightcolor/acmesh-ui/main/install.sh | sh
#
# Options (environment variables):
#   VERSION=v1.2.3   install a specific release (default: latest)
#   BINDIR=/path     install directory (default: /usr/local/bin)
set -eu

REPO="brightcolor/acmesh-ui"
BINDIR="${BINDIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

info() { printf '\033[1;34m==>\033[0m %s\n' "$1"; }
err()  { printf '\033[1;31mError:\033[0m %s\n' "$1" >&2; exit 1; }

# --- platform detection ---
os="$(uname -s)"
[ "$os" = "Linux" ] || err "acmesh-ui only ships Linux binaries (detected: $os). Build from source instead."

case "$(uname -m)" in
  x86_64|amd64)        arch="amd64" ;;
  aarch64|arm64)       arch="arm64" ;;
  *) err "unsupported architecture: $(uname -m)" ;;
esac

asset="acmesh-ui-linux-$arch"
if [ "$VERSION" = "latest" ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

# --- download tool ---
if command -v curl >/dev/null 2>&1; then dl() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then dl() { wget -qO "$2" "$1"; }
else err "need curl or wget"; fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

info "Downloading $asset ($VERSION)"
dl "$base/$asset" "$tmp/acmesh-ui" || err "download failed: $base/$asset"

# --- checksum verification (best effort: skip if no sha tool / no sums file) ---
if dl "$base/SHA256SUMS" "$tmp/SHA256SUMS" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then
    want="$(grep " $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')"
    got="$(sha256sum "$tmp/acmesh-ui" | awk '{print $1}')"
    [ "$want" = "$got" ] || err "checksum mismatch (want $want, got $got)"
    info "Checksum OK"
  fi
fi

chmod 0755 "$tmp/acmesh-ui"

# --- install (use sudo only if needed) ---
SUDO=""
if [ ! -w "$BINDIR" ]; then
  if command -v sudo >/dev/null 2>&1; then SUDO="sudo"; else err "$BINDIR is not writable and sudo is unavailable"; fi
fi
info "Installing to $BINDIR/acmesh-ui"
$SUDO install -m 0755 "$tmp/acmesh-ui" "$BINDIR/acmesh-ui"

info "Installed: $("$BINDIR/acmesh-ui" version)"
printf '\nNext: acmesh-ui init && acmesh-ui serve   (see https://github.com/%s)\n' "$REPO"
