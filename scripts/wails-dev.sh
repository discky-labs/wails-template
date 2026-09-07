#!/usr/bin/env bash
# Run the app in development mode: vite dev server + auto-rebuilding Go binary.
#
# Replaces `wails3 dev` (which is `wails3 task dev`, and so went away with the
# Taskfiles). The contract it relies on is read from wails3's own dev command
# (v3/internal/commands/dev.go):
#
#   WAILS_VITE_PORT       port the frontend dev server must listen on (9245)
#   FRONTEND_DEVSERVER_URL  a NON-production build reads this and proxies assets
#                           to vite instead of serving the embedded FS
#                           (v3/internal/assetserver/build_dev.go)
#
# That is why scripts/wails-build.sh omits the `production` tag when DEV=true.
#
# Run from the app project root (apps/desktop) — moon does this for you.
set -euo pipefail

VITE_PORT="${WAILS_VITE_PORT:-9245}"
export WAILS_VITE_PORT="$VITE_PORT"
export FRONTEND_DEVSERVER_URL="http://127.0.0.1:$VITE_PORT"
export DEV=true

# The vite plugin resolves ./bindings at startup and fails the build if the
# directory is missing, so generate before starting the server.
go tool wails3 generate bindings -clean=true -ts -i

echo "starting vite on $FRONTEND_DEVSERVER_URL"
( cd frontend && pnpm dev --port "$VITE_PORT" --strictPort ) &
vite_pid=$!

cleanup() {
  # Kill the whole process group of the vite child: `pnpm dev` spawns vite as a
  # grandchild, so killing only $vite_pid leaves the port bound and the next
  # `moon run desktop:dev` dies on --strictPort.
  kill -- "-$(ps -o pgid= "$vite_pid" 2>/dev/null | tr -d ' ')" 2>/dev/null || kill "$vite_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Wait for vite to accept connections before launching the app, otherwise the
# webview loads a blank page and does not retry.
for _ in $(seq 1 60); do
  if (exec 3<>/dev/tcp/127.0.0.1/"$VITE_PORT") 2>/dev/null; then
    exec 3>&- 2>/dev/null || true
    break
  fi
  sleep 0.5
done

if ! (exec 3<>/dev/tcp/127.0.0.1/"$VITE_PORT") 2>/dev/null; then
  echo "error: vite did not start on port $VITE_PORT within 30s" >&2
  exit 1
fi
exec 3>&- 2>/dev/null || true

# air watches **/*.go, re-runs the build (which regenerates bindings, matching
# what `wails3 dev` did on a Go change) and relaunches the binary.
exec go tool air -c .air.toml
