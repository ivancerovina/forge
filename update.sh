#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

echo "Pulling latest..."
git pull

echo "Building..."
go build -o forge

BIN_DIR="${HOME}/.local/bin"
LINK="${BIN_DIR}/forge"
TARGET="$(pwd)/forge"

mkdir -p "$BIN_DIR"

if [ -L "$LINK" ] && [ "$(readlink "$LINK")" = "$TARGET" ]; then
  echo "Symlink already in place: $LINK -> $TARGET"
else
  ln -sf "$TARGET" "$LINK"
  echo "Linked $LINK -> $TARGET"
fi

case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *) echo "Note: $BIN_DIR is not in your PATH" ;;
esac

echo "Done."
