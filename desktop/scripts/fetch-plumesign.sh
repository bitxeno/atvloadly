#!/usr/bin/env bash
# Downloads the plumesign binaries of the release into the platform resource
# directories mygo packages into the app. Binaries already present for the
# current version are kept, so this is safe to run before every build; a
# version change re-downloads them. Missing assets warn and leave the app to
# fall back to the PATH plumesign.
set -euo pipefail

# Resolve the repo root before changing directory, so the Dockerfile can be read
# from the script's location regardless of the caller's working directory.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DOCKERFILE="$REPO_ROOT/Dockerfile"

cd "$SCRIPT_DIR/../app"

# The server image (Dockerfile) is the single source of truth for the
# plumesign version, so the desktop bundle never drifts from it. Read the tag
# from its download URL; PLUMESIGN_VERSION overrides it for a one-off build.
dockerfile_version() {
  [[ -f "$DOCKERFILE" ]] || return 0
  grep -oE 'PlumeImpactor/releases/download/v[^/]+/' "$DOCKERFILE" 2>/dev/null \
    | head -n1 | sed -E 's#.*/(v[^/]+)/#\1#' || true
}
VERSION="${PLUMESIGN_VERSION:-$(dockerfile_version)}"
if [[ -z "$VERSION" ]]; then
  echo "error could not determine the plumesign version from $DOCKERFILE" >&2
  exit 1
fi
BASE="https://github.com/bitxeno/PlumeImpactor/releases/download/${VERSION}"

# The resources/bin directories are gitignored and reused across builds, so a
# downloaded binary can outlive the version it came from. Stamp each directory
# with the version it holds: on a version change the stale binaries are dropped
# and fetched again, so bumping the Dockerfile actually updates the bundle
# instead of silently keeping the old plumesign.
VERSION_STAMP="resources/.plumesign-version"
if [[ ! -f "$VERSION_STAMP" || "$(cat "$VERSION_STAMP")" != "$VERSION" ]]; then
  if [[ -f "$VERSION_STAMP" ]]; then
    echo "note  plumesign version changed ($(cat "$VERSION_STAMP") -> $VERSION); refetching"
  fi
  rm -f resources/darwin-arm64/bin/plumesign resources/darwin-amd64/bin/plumesign \
        resources/windows-amd64/bin/plumesign.exe resources/windows-arm64/bin/plumesign.exe
fi

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
if [[ -f resources/darwin-arm64/bin/plumesign && -f resources/darwin-amd64/bin/plumesign ]]; then
  echo "keep  resources/darwin-*/bin/plumesign"
elif fetch "plumesign-macos-universal.tar.gz" /tmp/plumesign-macos.tar.gz; then
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

# Windows: a plain executable per architecture. The upstream release has no
# windows-arm64 asset; when it is missing, fall back to the x86_64 binary —
# Windows on ARM runs it through emulation.
mkdir -p resources/windows-amd64/bin resources/windows-arm64/bin
if fetch "plumesign-windows-x86_64.exe" resources/windows-amd64/bin/plumesign.exe; then
  chmod +x resources/windows-amd64/bin/plumesign.exe
  if [[ ! -f resources/windows-arm64/bin/plumesign.exe ]]; then
    echo "note  plumesign-windows-arm64.exe not published; using the x86_64 binary (emulated on ARM)"
    cp resources/windows-amd64/bin/plumesign.exe resources/windows-arm64/bin/plumesign.exe
  else
    echo "keep  resources/windows-arm64/bin/plumesign.exe"
  fi
else
  echo "warn  windows plumesign missing; place a local build at resources/windows-amd64/bin/plumesign.exe" >&2
  # An empty bin directory still gets packaged and breaks the NSIS "File /r"
  # step ("no files found"); drop the directories so mygo skips them.
  rmdir resources/windows-amd64/bin resources/windows-arm64/bin 2>/dev/null || true
fi

printf '%s\n' "$VERSION" > "$VERSION_STAMP"

echo "plumesign resources ready (${VERSION}) — missing binaries warn and fall back to PATH"
