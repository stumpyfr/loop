#!/bin/sh
set -eu

REPO="${REPO:-stumpyfr/agentkit}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="${BINARY_NAME:-agentkit}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m | tr '[:upper:]' '[:lower:]')"
ext=""

case "$os" in
  darwin)
    os="darwin"
    ;;
  linux)
    os="linux"
    ;;
  mingw*|msys*|cygwin*)
    os="windows"
    ext=".exe"
    ;;
  *)
    echo "unsupported operating system: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64)
    arch="amd64"
    ;;
  arm64|aarch64)
    arch="arm64"
    ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

asset="${BINARY_NAME}_${os}_${arch}${ext}"
base_url="https://github.com/${REPO}/releases/latest/download"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

echo "Downloading ${asset} from ${REPO} latest release..."
download_asset="$asset"
curl -fsSL "${base_url}/${download_asset}" -o "${tmp_dir}/${download_asset}"
curl -fsSL "${base_url}/${download_asset}.sha256" -o "${tmp_dir}/${download_asset}.sha256"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp_dir" && sha256sum -c "${download_asset}.sha256")
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp_dir" && shasum -a 256 -c "${download_asset}.sha256")
else
  echo "warning: sha256sum or shasum not found; skipping checksum verification" >&2
fi

mkdir -p "$INSTALL_DIR"
chmod 0755 "${tmp_dir}/${download_asset}"

target="${INSTALL_DIR}/${BINARY_NAME}${ext}"
if ! mv -f "${tmp_dir}/${download_asset}" "$target" 2>/dev/null; then
  echo "Installing to ${target} requires elevated permissions; retrying with sudo." >&2
  sudo mv -f "${tmp_dir}/${download_asset}" "$target"
fi

echo "Installed ${target}"
