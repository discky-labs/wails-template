#!/usr/bin/env bash
# Launch the built app and assert it stays up.
#
# A Wails binary can compile perfectly and still die instantly at runtime — a
# missing webview library, a bad asset embed, a service whose ServiceStartup
# returns an error. `go build` catches none of those, so CI runs this on all
# three platforms after building.
#
# Linux CI runners are headless, so the app is launched under Xvfb when no
# DISPLAY is present. macOS and Windows runners already have a window server.
#
# Env:
#   APP_NAME   binary name (default: desktop)
#   HOLD_SECS  how long it must stay alive (default: 5)
set -euo pipefail

APP_NAME="${APP_NAME:-desktop}"
BIN_DIR="${BIN_DIR:-bin}"
HOLD_SECS="${HOLD_SECS:-5}"

ext=""
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) ext=".exe" ;;
esac

bin="$BIN_DIR/$APP_NAME$ext"
[ -x "$bin" ] || { echo "error: $bin not found or not executable" >&2; exit 1; }

# NOT an array. macOS ships bash 3.2, where expanding an EMPTY array under
# `set -u` is an error ("launcher[@]: unbound variable") — bash 5 on Linux
# tolerates it. Since the launcher is only ever the two fixed words `xvfb-run
# -a`, a plain string with word splitting at the call site is portable and
# needs no array at all.
launcher=""
if [ "$(uname -s)" = "Linux" ] && [ -z "${DISPLAY:-}" ]; then
  command -v xvfb-run >/dev/null 2>&1 \
    || { echo "error: headless Linux needs xvfb-run (apt install xvfb)" >&2; exit 1; }
  # -a picks a free display number; the app also needs a writable runtime dir.
  launcher="xvfb-run -a"
  export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-$(mktemp -d)}"

  # CI-ONLY CONCESSION, and only on a headless Linux runner.
  #
  # WebKitGTK runs its web process inside bubblewrap, which needs
  # network-namespace privileges a CI container does not grant:
  #   bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted
  # and there is no GPU/DRI3 behind Xvfb, so the dmabuf renderer fails too.
  #
  # Disabling the WebKit sandbox is a REAL security downgrade. It is acceptable
  # here because this is a throwaway container running our own binary for five
  # seconds. Never ship or recommend these for a user-facing run.
  export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1
  export WEBKIT_DISABLE_DMABUF_RENDERER=1
fi

log="$(mktemp)"
trap 'rm -f "$log"' EXIT

echo "launching $bin (must survive ${HOLD_SECS}s)"
# Unquoted on purpose: $launcher must word-split into `xvfb-run -a`, or vanish
# entirely when empty.
# shellcheck disable=SC2086
$launcher "./$bin" >"$log" 2>&1 &
pid=$!

alive=true
for _ in $(seq 1 "$HOLD_SECS"); do
  sleep 1
  kill -0 "$pid" 2>/dev/null || { alive=false; break; }
done

if [ "$alive" = false ]; then
  # `wait` reaps it and yields the real exit status.
  set +e
  wait "$pid"
  code=$?
  set -e
  # Status 0 here means the process ended without failing, which for a GUI app
  # that should still be running almost always means it never really started —
  # a launcher or environment problem, not a crash. Say which.
  if [ "$code" -eq 0 ]; then
    echo "FAIL: $bin did not stay running, and exited cleanly (status 0)." >&2
    echo "      That usually means it never launched — check the launcher and" >&2
    echo "      the display/environment rather than the app itself." >&2
  else
    echo "FAIL: $bin exited after less than ${HOLD_SECS}s with status $code" >&2
  fi
  echo "--- output ---" >&2
  cat "$log" >&2
  exit 1
fi

echo "PASS: still running after ${HOLD_SECS}s; shutting down"
kill "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true

if [ -s "$log" ]; then
  echo "--- app output ---"
  cat "$log"
fi
