#!/usr/bin/env bash
# Generate the frontend's TypeScript bindings from the Go services.
#
# Owned by the FRONTEND moon project, because everything it writes lands under
# apps/desktop/frontend/. That ownership is not cosmetic: the Go binary embeds
# frontend/dist while the frontend imports Go-generated bindings, so if the two
# projects each owned one half, moon's project graph would be cyclic. The
# frontend produces both; apps/desktop only consumes the bundle.
#
# The generator has to run inside the Go module, so resolve that from this
# script's own location rather than the caller's cwd.
set -euo pipefail

cd "$(dirname "$0")/../apps/desktop"
exec go tool wails3 generate bindings -clean=true -ts -i "$@"
