#!/bin/sh
# Package a release: a cross-compiled tarball per supported platform plus
# SHA256SUMS, all under dist/. Run scripts/check.sh first; this script only
# builds and packages. Publish with:
#
#   gh release create v<version> dist/*.tar.gz dist/SHA256SUMS
set -eu
cd "$(dirname "$0")/.."

version="$(sed -n 's/^const version = "\(.*\)"$/\1/p' main.go)"
[ -n "$version" ] || { echo "release: could not read version from main.go" >&2; exit 1; }

rm -rf dist
mkdir -p dist

for target in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
  os="${target%_*}"
  arch="${target#*_}"
  echo "==> $os/$arch" >&2
  mkdir -p "dist/$target"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w" \
    -o "dist/$target/herdr-workspaces" .
  tar -czf "dist/herdr-workspaces_${version}_${os}_${arch}.tar.gz" \
    -C "dist/$target" herdr-workspaces
done

cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -- *.tar.gz > SHA256SUMS
else
  shasum -a 256 -- *.tar.gz > SHA256SUMS
fi
echo "dist/ ready for v${version}" >&2
