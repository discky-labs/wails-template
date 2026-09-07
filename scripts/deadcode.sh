#!/bin/bash
# Run `deadcode -test ./...` and FAIL on any finding.
#
# `deadcode` itself is report-only by design (exit 0 even when it
# prints findings), which is the wrong default for CI: a newly-
# orphaned func would slip in silently. This wrapper inverts that —
# any output line becomes a non-zero exit so the moon task fails and
# the PR is blocked.
#
# Library-only modules (proto/, libraries/go-shared/) neutralize this
# task at the moon.yml level (command: "true") because they have no
# main package to seed reachability analysis. Their callers (the
# service binaries) run their own deadcode pass — that's where unused
# library exports get caught.
set -euo pipefail

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

go tool deadcode -test ./... | tee "$tmp"

if [ -s "$tmp" ]; then
  echo "" >&2
  echo "deadcode: findings above are unreachable from any entrypoint." >&2
  echo "Either wire the function into a call path or delete it." >&2
  exit 1
fi
