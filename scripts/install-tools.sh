#!/usr/bin/env bash
# Install the tools that proto and `go tool` don't cover.
#
# Deliberately short. Most tooling is pinned elsewhere:
#   - go, node, pnpm, moon        -> .prototools (`proto install`)
#   - JS tooling (oxlint, oxfmt,
#     knip, syncpack, typos,
#     vitest, typescript)         -> package.json (`pnpm install`)
#   - wails3, deadcode,
#     govulncheck, air            -> `tool` directives in apps/desktop/go.mod,
#                                    invoked as `go tool <name>`
#   - gofumpt, goimports, gci,
#     golines                     -> bundled into golangci-lint's `fmt` chain
#                                    (see .golangci.yml formatters)
#
# Which leaves the two standalone binaries below, plus the OS-level webview
# libraries that only a package manager can provide.
set -euo pipefail

GOLANGCI_LINT_VERSION="v2.12.2"
LEFTHOOK_VERSION="v2.1.10"

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "installing golangci-lint $GOLANGCI_LINT_VERSION..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
    | sh -s -- -b "$(go env GOPATH)/bin" "$GOLANGCI_LINT_VERSION"
else
  echo "golangci-lint already installed"
fi

if ! command -v lefthook >/dev/null 2>&1; then
  echo "installing lefthook $LEFTHOOK_VERSION..."
  go install "github.com/evilmartians/lefthook/v2@$LEFTHOOK_VERSION"
else
  echo "lefthook already installed"
fi

pnpm install

if command -v lefthook >/dev/null 2>&1; then
  lefthook install
else
  echo "lefthook not on PATH; git hooks NOT wired (is \$(go env GOPATH)/bin on PATH?)" >&2
fi

# Wails needs GTK4 + WebKitGTK 6.0 on Linux. These are OS packages, so this
# script reports rather than installs — `wails3 doctor` prints the exact command
# for your distro.
if [ "$(uname -s)" = "Linux" ]; then
  missing=""
  for pkg in gtk4 webkitgtk-6.0; do
    pkg-config --exists "$pkg" 2>/dev/null || missing="$missing $pkg"
  done
  if [ -n "$missing" ]; then
    echo ""
    echo "MISSING webview libraries:$missing" >&2
    echo "  Debian/Ubuntu: sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev" >&2
    echo "  Fedora:        sudo dnf install gcc pkg-config gtk4-devel webkitgtk6.0-devel" >&2
    echo "  Arch:          sudo pacman -S base-devel gtk4 webkitgtk-6.0" >&2
    echo "Then run: go tool wails3 doctor" >&2
    exit 1
  fi
fi

echo "tools ready."
