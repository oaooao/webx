#!/usr/bin/env bash
set -euo pipefail

# webx installer
# Usage: curl -fsSL https://raw.githubusercontent.com/oaooao/webx/master/scripts/install.sh | bash

REPO="oaooao/webx"
BINARY="webx"

# Detect OS
OS="$(uname -s)"
case "${OS}" in
  Linux*)   OS="linux" ;;
  Darwin*)  OS="darwin" ;;
  *)
    echo "Unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# Fetch latest release version
echo "Fetching latest release..."
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"

if [ -z "${VERSION}" ]; then
  echo "Failed to fetch latest version" >&2
  exit 1
fi

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})"

# Construct download URLs
ARCHIVE="${BINARY}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"

# Create temp directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# Download archive and checksums
echo "Downloading ${ARCHIVE_URL}..."
curl -fsSL -o "${TMP_DIR}/${ARCHIVE}" "${ARCHIVE_URL}"
curl -fsSL -o "${TMP_DIR}/checksums.txt" "${CHECKSUM_URL}"

# Verify checksum
echo "Verifying checksum..."
cd "${TMP_DIR}"
if command -v sha256sum > /dev/null 2>&1; then
  grep "${ARCHIVE}" checksums.txt | sha256sum -c -
elif command -v shasum > /dev/null 2>&1; then
  grep "${ARCHIVE}" checksums.txt | shasum -a 256 -c -
else
  echo "WARNING: no sha256 tool found, skipping checksum verification" >&2
fi
cd - > /dev/null

# Extract
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

# Determine install location
if [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
elif command -v sudo > /dev/null 2>&1; then
  INSTALL_DIR="/usr/local/bin"
  USE_SUDO=1
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "${INSTALL_DIR}"
fi

# Install binary
if [ "${USE_SUDO:-0}" = "1" ]; then
  sudo install -m 755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  install -m 755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo "${BINARY} ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"

# Check if install dir is in PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "NOTE: ${INSTALL_DIR} is not in your PATH."
    echo "Add the following to your shell profile:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac
