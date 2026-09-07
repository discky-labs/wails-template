#!/usr/bin/env bash
# Run the repo's own Semgrep rules over the Go tree.
#
# These encode Wails v3 semantics that golangci-lint cannot express — see
# .semgrep/README.md for the division of labour. Every rule passes on the
# current tree, so any finding is a real violation or a false positive to fix;
# there is no accepted backlog.
#
# `--error` makes a finding exit non-zero. `--metrics=off` keeps the run
# offline. Scope is apps/ + libraries/ so the rules' own fixtures under
# .semgrep/rules/** are never scanned by the rules they test.
set -euo pipefail

# Optional local tool, same tier as the other non-Go binaries: warn and pass
# rather than blocking a local commit. CI installs it, so the gate still holds
# where it counts.
if ! command -v semgrep >/dev/null 2>&1; then
  echo "warning: semgrep not on PATH — skipping. Install: pip install semgrep"
  exit 0
fi

MODE="${1:-scan}"

case "$MODE" in
scan)
  semgrep --config .semgrep/rules --error --metrics=off apps libraries
  ;;
test)
  # The rules' own regression suite. Each <name>.yaml is paired with a
  # <name>.go carrying `ruleid:` (must match) and `ok:` (must not) cases.
  semgrep --test --config .semgrep/rules .semgrep/rules
  ;;
*)
  echo "usage: $0 [scan|test]" >&2
  exit 2
  ;;
esac
