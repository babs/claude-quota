#!/bin/bash
set -euo pipefail

MODULE=$(grep module go.mod | cut -d\  -f2)
BINBASE=${MODULE##*/}
VERSION=${VERSION:-${GITHUB_REF_NAME:-}}
VERSION=${VERSION:-0.0.0}
COMMIT_HASH="$(git rev-parse --short HEAD 2>/dev/null)"
COMMIT_HASH=${COMMIT_HASH:-00000000}
DIRTY=$(git diff --quiet 2>/dev/null || echo '-dirty')
BUILD_TIMESTAMP=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILDER=$(go version)

[ -d dist ] && rm -rf dist
mkdir dist

LDFLAGS=(
  "-X 'main.Version=${VERSION}'"
  "-X 'main.CommitHash=${COMMIT_HASH}${DIRTY}'"
  "-X 'main.BuildTimestamp=${BUILD_TIMESTAMP}'"
  "-X 'main.Builder=${BUILDER}'"
)

# Determine which targets to build based on the host OS.
# macOS targets require CGO (fyne.io/systray uses Objective-C),
# so they must be built on a macOS host.
HOST_OS=$(uname -s | tr '[:upper:]' '[:lower:]')

if [ "$HOST_OS" = "darwin" ]; then
  TARGETS="darwin/amd64 darwin/arm64"
  CGO=1
else
  TARGETS="linux/amd64 linux/arm64 windows/amd64 windows/arm64"
  CGO=0
fi

for DIST in $TARGETS; do
  GOOS=${DIST%/*}
  GOARCH=${DIST#*/}
  SUFFIX=""
  [ "$GOOS" = "windows" ] && SUFFIX=".exe"
  TARGET=${BINBASE}-${GOOS}-${GOARCH}
  echo "Building ${TARGET}..."
  env CGO_ENABLED=$CGO GOOS=$GOOS GOARCH=$GOARCH go build \
    -ldflags="${LDFLAGS[*]}" \
    -o dist/${TARGET}${SUFFIX}
  (cd dist; sha256sum ${TARGET}${SUFFIX} 2>/dev/null || shasum -a 256 ${TARGET}${SUFFIX}) | tee -a ${BINBASE}.sha256sum
  if [ -z "${NOCOMPRESS:-}" ]; then
    if [ "$GOOS" = "windows" ]; then
      xz --keep dist/${TARGET}${SUFFIX}
      (cd dist; zip -qm9 ${TARGET}.zip ${TARGET}${SUFFIX})
    else
      xz dist/${TARGET}
    fi
  fi
done

(cd dist; sha256sum * 2>/dev/null || shasum -a 256 *) | tee -a ${BINBASE}.sha256sum
mv ${BINBASE}.sha256sum dist/
echo "Done."
