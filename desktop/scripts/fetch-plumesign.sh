#!/usr/bin/env bash
# Downloads the plumesign binaries of the release into the platform resource
# directories mygo packages into the app. Binaries already present are kept,
# so this is safe to run before every build. Missing assets warn and leave
# the app to fall back to the PATH plumesign.
set -euo pipefail

cd "$(dirname "$0")/../app"

VERSION="${PLUMESIGN_VERSION:-v2.7.1-patch.5}"
BASE="https://github.com/bitxeno/PlumeImpactor/releases/download/${VERSION}"

fetch() { # fetch <asset-name> <dest>
  local asset="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "keep  $dest"
    return 0
  fi
  echo "fetch $asset"
  if ! curl -fsSL "$BASE/$asset" -o "$dest"; then
    rm -f "$dest"
    echo "warn  $asset not published; place a local build at $dest" >&2
    return 1
  fi
}

# macOS: a tar.gz holding the universal binary. mygo build lipo's
# resources/darwin-arm64 and resources/darwin-amd64 into universal binaries,
# so the same archive serves both and the universal build needs either one.
mkdir -p resources/darwin-arm64/bin resources/darwin-amd64/bin
if fetch "plumesign-macos-universal.tar.gz" /tmp/plumesign-macos.tar.gz; then
  rm -rf /tmp/plumesign-extract && mkdir -p /tmp/plumesign-extract
  tar -xzf /tmp/plumesign-macos.tar.gz -C /tmp/plumesign-extract
  found=$(find /tmp/plumesign-extract -type f -name "plumesign*" | head -1)
  if [[ -n "$found" ]]; then
    cp "$found" resources/darwin-arm64/bin/plumesign
    cp "$found" resources/darwin-amd64/bin/plumesign
    chmod +x resources/darwin-arm64/bin/plumesign resources/darwin-amd64/bin/plumesign
  else
    echo "warn  no plumesign binary in the macOS archive" >&2
  fi
  rm -rf /tmp/plumesign-extract /tmp/plumesign-macos.tar.gz
fi

# Windows: a plain executable per architecture.
mkdir -p resources/windows-amd64/bin resources/windows-arm64/bin
fetch "plumesign-windows-x86_64.exe" resources/windows-amd64/bin/plumesign.exe || true
fetch "plumesign-windows-arm64.exe" resources/windows-arm64/bin/plumesign.exe || true

echo "plumesign resources ready (${VERSION}) — missing binaries warn and fall back to PATH"
