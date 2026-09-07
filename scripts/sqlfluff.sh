#!/usr/bin/env bash
# Lint (or fix) every .sql file in the tree.
#
# The only SQL here is apps/desktop/internal/repository/migrations/*.up.sql,
# embedded by //go:embed and applied when the app opens its database. That is
# why it earns a linter: a malformed migration is NOT a compile error, it is a
# runtime failure on a user's machine the first time the app launches, with no
# operator there to fix it.
#
# An OPTIONAL external tool, same tier as semgrep / typos: warn and pass rather
# than blocking a local commit. CI installs a pinned version, so the gate holds
# where it counts.
set -euo pipefail

if ! command -v sqlfluff >/dev/null 2>&1; then
  echo "warning: sqlfluff not on PATH — skipping. Install: pip install sqlfluff"
  exit 0
fi

MODE="${1:-lint}"

case "$MODE" in
lint | fix) ;;
*)
  echo "usage: $0 [lint|fix]" >&2
  exit 2
  ;;
esac

# No --force on `fix`: it is deprecated from sqlfluff 4.x, where applying the
# fix IS the default and the flag only prints a warning.
#
# `xargs -r` so a tree with no .sql files is a no-op rather than sqlfluff
# erroring on an empty path list.
find . -name '*.sql' \
  -not -path '*/node_modules/*' \
  -not -path '*/.moon/cache/*' \
  -not -path '*/dist/*' \
  -print0 | xargs -0 -r sqlfluff "$MODE" --config .sqlfluff
