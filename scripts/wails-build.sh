#!/usr/bin/env bash
# Build the Wails desktop binary.
#
# Replaces the wails3-generated build/{linux,darwin,windows}/Taskfile.yml. Every
# flag below is ported VERBATIM from those files at v3.0.0-beta.17, so a moon
# build produces what `wails3 build` produced. If you upgrade wails3, re-read
# them (`wails3 init` into a scratch dir) and diff before changing anything here.
#
# The frontend bundle, bindings and icons are moon task DEPS (see
# apps/desktop/moon.yml), not steps here: moon owns ordering and caching.
#
# Env:
#   TARGET_OS    target GOOS   (default: host)
#   TARGET_ARCH  target GOARCH (default: host)
#   DEV          "true" for a debug build (default: false)
#   EXTRA_TAGS   extra comma-separated build tags
#   APP_NAME     binary name   (default: desktop)
#   OUTPUT       explicit output path (default: bin/$APP_NAME[.exe])
set -euo pipefail

APP_NAME="${APP_NAME:-desktop}"
BIN_DIR="${BIN_DIR:-bin}"
DEV="${DEV:-false}"
EXTRA_TAGS="${EXTRA_TAGS:-}"
TARGET_OS="${TARGET_OS:-$(go env GOHOSTOS)}"
TARGET_ARCH="${TARGET_ARCH:-$(go env GOHOSTARCH)}"

# Cross-OS builds need CGO toolchains we deliberately don't ship (the upstream
# answer is an ~800MB `wails-cross` Docker image; ours is a 3-OS CI matrix).
# Fail loudly rather than emit a subtly broken binary.
if [ "$TARGET_OS" != "$(go env GOHOSTOS)" ] && [ "$TARGET_OS" != "windows" ]; then
  echo "error: cross-compiling to '$TARGET_OS' from '$(go env GOHOSTOS)' needs a CGO" >&2
  echo "       cross toolchain. Build on a '$TARGET_OS' runner instead (see" >&2
  echo "       .github/workflows/release.yml)." >&2
  exit 1
fi

# Tag and flag sets, per platform Taskfile.
tags="production"
build_flags=(-trimpath -buildvcs=false)
ldflags="-w -s"

if [ "$DEV" = "true" ]; then
  # Dev builds carry no `production` tag: that is what makes the asset server
  # read FRONTEND_DEVSERVER_URL and proxy to vite instead of the embedded FS.
  tags=""
  build_flags=(-buildvcs=false '-gcflags=all=-l')
  ldflags=""
fi

if [ -n "$EXTRA_TAGS" ]; then
  tags="${tags:+$tags,}$EXTRA_TAGS"
fi
[ -n "$tags" ] && build_flags+=(-tags "$tags")

export GOOS="$TARGET_OS"
export GOARCH="$TARGET_ARCH"

ext=""
case "$TARGET_OS" in
  linux)
    export CGO_ENABLED=1
    ;;
  darwin)
    export CGO_ENABLED=1
    export CGO_CFLAGS="-mmacosx-version-min=12.0"
    export CGO_LDFLAGS="-mmacosx-version-min=12.0"
    export MACOSX_DEPLOYMENT_TARGET="12.0"
    ;;
  windows)
    export CGO_ENABLED=0
    ext=".exe"
    # -H windowsgui suppresses the console window for a GUI app.
    [ -n "$ldflags" ] && ldflags="$ldflags -H windowsgui"
    ;;
  *)
    echo "error: unsupported target OS '$TARGET_OS'" >&2
    exit 1
    ;;
esac

[ -n "$ldflags" ] && build_flags+=("-ldflags=$ldflags")

OUTPUT="${OUTPUT:-$BIN_DIR/$APP_NAME$ext}"

# Windows embeds icon + manifest + version info through a generated .syso that
# `go build` picks up implicitly from the package dir, so it must exist first
# and be removed after (it is GOARCH-specific and would poison another target).
syso=""
if [ "$TARGET_OS" = "windows" ]; then
  syso="wails_windows_${TARGET_ARCH}.syso"
  go tool wails3 generate syso \
    -arch "$TARGET_ARCH" \
    -icon build/windows/icon.ico \
    -manifest build/windows/wails.exe.manifest \
    -info build/windows/info.json \
    -out "$syso"
  trap 'rm -f "$syso"' EXIT
fi

mkdir -p "$BIN_DIR"
echo "building $OUTPUT (os=$TARGET_OS arch=$TARGET_ARCH dev=$DEV tags='${tags:-none}')"
go build "${build_flags[@]}" -o "$OUTPUT"

echo "built $OUTPUT"
