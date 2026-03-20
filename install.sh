#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
BINARY="forge"

if ! command -v go &>/dev/null; then
  echo "Error: go is not installed" >&2
  exit 1
fi

echo "Building ${BINARY}..."
go build -o "${BINARY}" .

echo "Installing to ${INSTALL_DIR}/${BINARY}..."
sudo mkdir -p "${INSTALL_DIR}"
sudo install -m 755 "${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Cleaning up build artifact..."
rm -f "${BINARY}"

echo "Installed $(${BINARY} --version) to ${INSTALL_DIR}/${BINARY}"
