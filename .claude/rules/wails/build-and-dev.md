# Rule: Build and dev (moon is the only task runner)

Upstream Wails v3 orchestrates builds with go-task through a generated
`Taskfile.yml` plus one per platform. **This repo deleted them.** moon runs
everything, and the per-OS logic lives in `/scripts`.

## ❌ Do not use these

`wails3 build`, `wails3 package` and `wails3 dev` are thin wrappers around
`wails3 task <x>`. With no Taskfiles they **do not work here**:

| Instead of       | Run                        |
| ---------------- | -------------------------- |
| `wails3 dev`     | `moon run desktop:dev`     |
| `wails3 build`   | `moon run desktop:build`   |
| `wails3 package` | `moon run desktop:package` |
| `wails3 task …`  | — no Taskfiles exist       |

❌ **Never re-add a Taskfile**, and never introduce a second task runner.

Every _other_ subcommand works, because those are compiled into the CLI:
`generate bindings|icons|syso|appimage`, `tool package|lipo|has`,
`update build-assets`, `doctor`, `sign`. Invoke them as **`go tool wails3 …`**
— the CLI is a `go tool` dependency pinned to the runtime version, not a
globally installed binary.

## Build flags are ported, not invented

`scripts/wails-build.sh` reproduces the upstream per-OS Taskfiles verbatim:

| Target  | Env                                          | Production flags                                              |
| ------- | -------------------------------------------- | ------------------------------------------------------------- |
| linux   | `CGO_ENABLED=1`                              | `-tags production -trimpath -buildvcs=false -ldflags="-w -s"` |
| darwin  | `CGO_ENABLED=1`, `-mmacosx-version-min=12.0` | same                                                          |
| windows | `CGO_ENABLED=0`                              | same **+ `-H windowsgui`**                                    |

- Windows needs `wails3 generate syso` **before** `go build` and `rm -f *.syso`
  after — it is GOARCH-specific and would poison another target.
- **On a wails3 upgrade**, scaffold a throwaway project
  (`wails3 init -n scratch -t react`) and diff its Taskfiles against the script
  before changing anything. Do not edit these flags from memory.
- CGO rules out cross-OS builds; each platform builds natively in a CI matrix.
  A DMG cannot be built anywhere but macOS.

## Dev mode contract

`moon run desktop:dev` replaces the `refresh`-based watcher `wails3 dev` used.
The contract comes from wails3's own `dev.go`:

- **`WAILS_VITE_PORT`** (9245) — vite must listen here with `strictPort: true`.
  ❌ Do not change the port on one side only.
- **`FRONTEND_DEVSERVER_URL`** — a **non-production** build reads this and
  proxies assets to vite instead of serving the embedded FS.

This is why `scripts/wails-build.sh` omits the `production` tag when
`DEV=true`. ❌ Adding `production` to a dev build silently serves the stale
embedded bundle instead of vite.

`scripts/wails-dev.sh` exports both vars, starts vite, waits for the port, then
hands over to `air` (`.air.toml`) for Go rebuilds. Browsing `localhost:9245`
directly renders the UI but **bindings do not work there** — by design.

## The embed directive

```go
//go:embed all:frontend/dist
```

- The **`all:`** prefix is required, or Go skips files beginning with `.` or
  `_` — exactly what a bundler emits. The app then builds, runs and serves a
  blank window in production while working in dev. Enforced by Semgrep
  `go-wails-embed-missing-all-prefix`.
- **`frontend/dist/.gitkeep` is tracked on purpose**: `//go:embed` fails to
  _compile_ when the directory is absent, which would break `go build`,
  golangci-lint and `go test` on a fresh clone. ❌ Do not gitignore it.

## Task hygiene

- Task bodies stay ≤ ~2 lines; real logic goes in `scripts/*.sh` with
  `set -euo pipefail`, and **the script is listed as a task input** so editing
  it invalidates the cache.
- `apps/desktop` overrides the inherited `build` task, because the shared
  `.moon/tasks/go.yml` definition (`go build ./...`) cannot produce a Wails
  binary.
- CI verifies the app **runs**, not just compiles: `moon run desktop:smoke`
  launches it on all three platforms (headless via Xvfb on Linux). A binary
  that compiles can still die instantly on a missing webview library or a
  failed `ServiceStartup`.
