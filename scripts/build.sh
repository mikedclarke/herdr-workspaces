#!/bin/sh
# Build the plugin binary at install time. herdr runs this as the manifest's
# [[build]] step, in the plugin root, with no guaranteed Go toolchain. Prefer
# Go (an exact build of the source you have); without it, fall back to the
# prebuilt release binary. Either way the result is ./bin/herdr-workspaces,
# which the manifest's action and pane invoke.
set -e
cd "$(dirname "$0")/.."
mkdir -p bin
if command -v go >/dev/null 2>&1; then
  exec go build -o bin/herdr-workspaces .
fi
echo "herdr-workspaces: no Go toolchain found; downloading a prebuilt binary" >&2
INSTALL_DIR="$(pwd)/bin" sh scripts/install.sh
