#!/bin/sh
# Build the plugin binary at install time. herdr runs this as the manifest's
# [[build]] step, in the plugin root. Requires a Go toolchain; prebuilt release
# binaries are a future item.
set -e
cd "$(dirname "$0")/.."
if ! command -v go >/dev/null 2>&1; then
  echo "herdr-workspaces: Go toolchain not found; install Go 1.24+ to build" >&2
  exit 1
fi
mkdir -p bin
exec go build -o bin/herdr-workspaces .
