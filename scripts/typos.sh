#!/usr/bin/env bash
# Spell-check the tree with `typos`.
#
# An OPTIONAL external tool, same tier as semgrep / golangci-lint / lefthook:
# warn and pass rather than blocking a local commit. CI installs the real binary
# on Linux, where the gate actually runs.
#
# Why not an npm devDependency? It was one (@ocular-d/typos-bin), and that
# package ships no Windows binary, so its postinstall threw and
# `pnpm install --frozen-lockfile` failed OUTRIGHT on the Windows runner:
#   Error: No binary found matching your system. It's probably not supported
# That broke the whole Windows build over a check Windows never runs. Making it
# external removes a platform-specific dependency from every developer's install.
set -euo pipefail

if ! command -v typos >/dev/null 2>&1; then
  echo "warning: typos not on PATH — skipping. Install: https://github.com/crate-ci/typos"
  exit 0
fi

exec typos "$@"
