#!/usr/bin/env bash
# Package the built binary into platform installers.
#
# Replaces the packaging half of the wails3-generated per-OS Taskfiles. The
# commands are ported VERBATIM from v3.0.0-beta.17 — note they mostly delegate
# to subcommands compiled into the wails3 binary (`generate appimage`,
# `generate .desktop`, `generate webview2bootstrapper`, `tool package`), so the
# NSIS / nfpm / AppImage / dmg logic still lives upstream, not here.
#
# Assumes `scripts/wails-build.sh` already ran (moon declares it as a dep).
# Run from the app project root (apps/desktop) — moon does this for you.
#
# Env:
#   TARGET_OS  target GOOS (default: host)
#   APP_NAME   binary name (default: desktop)
#   FORMATS    space-separated subset, e.g. "deb rpm". Default: all for the OS.
set -euo pipefail

APP_NAME="${APP_NAME:-desktop}"
BIN_DIR="${BIN_DIR:-bin}"
TARGET_OS="${TARGET_OS:-$(go env GOHOSTOS)}"
TARGET_ARCH="${TARGET_ARCH:-$(go env GOHOSTARCH)}"
ROOT="$PWD"

die() { echo "error: $*" >&2; exit 1; }

package_linux() {
  local formats="${FORMATS:-appimage deb rpm}"

  # Every Linux format needs the .desktop entry.
  mkdir -p "$ROOT/build/linux/appimage"
  go tool wails3 generate .desktop \
    -name "$APP_NAME" \
    -exec "$APP_NAME" \
    -icon "$APP_NAME" \
    -outputfile "$ROOT/build/linux/$APP_NAME.desktop" \
    -categories "Development;"

  for fmt in $formats; do
    case "$fmt" in
      appimage)
        # wails3 generate appimage expects the binary and icon beside it, named
        # after the app, and resolves the other paths relative to that dir. Those
        # copies are dead once the AppImage exists, so clean them up rather than
        # leaving a ~10MB binary in build/ for someone to commit by accident.
        ( cd "$ROOT/build/linux/appimage"
          trap 'rm -rf "./$APP_NAME" "./$APP_NAME.png" ./build' EXIT
          cp "$ROOT/$BIN_DIR/$APP_NAME" "./$APP_NAME"
          cp "$ROOT/build/appicon.png" "./$APP_NAME.png"
          go tool wails3 generate appimage \
            -binary "$APP_NAME" \
            -icon "$APP_NAME.png" \
            -desktopfile "../$APP_NAME.desktop" \
            -outputdir "$ROOT/$BIN_DIR" \
            -builddir "$ROOT/build/linux/appimage/build" )
        ;;
      deb|rpm|archlinux)
        go tool wails3 tool package \
          -name "$APP_NAME" \
          -format "$fmt" \
          -config ./build/linux/nfpm/nfpm.yaml \
          -out "$ROOT/$BIN_DIR"
        ;;
      *) die "unknown linux format '$fmt'" ;;
    esac
  done
}

package_darwin() {
  local formats="${FORMATS:-app dmg}"
  local app="$ROOT/$BIN_DIR/$APP_NAME.app"

  # The .app bundle is a plain directory layout — build it, then ad-hoc sign so
  # it launches locally. A real Developer ID signature is out of scope here.
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  cp build/darwin/icons.icns "$app/Contents/Resources/"
  [ -f build/darwin/Assets.car ] && cp build/darwin/Assets.car "$app/Contents/Resources/"
  cp "$ROOT/$BIN_DIR/$APP_NAME" "$app/Contents/MacOS/"
  cp build/darwin/Info.plist "$app/Contents/"

  if [ "$(go env GOHOSTOS)" = "darwin" ]; then
    codesign --force --deep --sign - "$app"
  else
    echo "skipping codesign (not on macOS) — sign the .app on macOS before distribution"
  fi

  case " $formats " in
    *" dmg "*)
      # hdiutil is macOS-only, so a DMG cannot be cross-built (the .app can).
      [ "$(go env GOHOSTOS)" = "darwin" ] || { echo "skipping dmg (macOS only)"; return 0; }
      go tool wails3 tool package --format dmg --name "$APP_NAME" --out "$BIN_DIR" \
        --background "build/darwin/dmg-background.png" \
        --volume-icon "build/darwin/icons.icns" \
        --file-icon "build/darwin/dmg-file-icon.icns" \
        --window-width 540 --window-height 380
      ;;
  esac
}

package_windows() {
  local scope="${INSTALL_SCOPE:-machine}"
  local arg_flag="ARM64"
  [ "$TARGET_ARCH" = "amd64" ] && arg_flag="AMD64"

  command -v makensis >/dev/null 2>&1 \
    || die "makensis not found. Install NSIS (winget install NSIS.NSIS) to build the installer."

  go tool wails3 generate webview2bootstrapper -dir "$ROOT/build/windows/nsis"

  local scope_flags=()
  if [ "$scope" = "user" ]; then
    scope_flags=(-DWAILS_INSTALL_SCOPE=user -DREQUEST_EXECUTION_LEVEL=user)
  fi

  ( cd "$ROOT/build/windows/nsis"
    makensis "${scope_flags[@]}" \
      "-DARG_WAILS_${arg_flag}_BINARY=$ROOT/$BIN_DIR/$APP_NAME.exe" \
      project.nsi )
}

case "$TARGET_OS" in
  linux)   package_linux ;;
  darwin)  package_darwin ;;
  windows) package_windows ;;
  *) die "unsupported target OS '$TARGET_OS'" ;;
esac

echo "packaged into $BIN_DIR:"
ls -1 "$BIN_DIR"
