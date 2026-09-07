---
name: wails
description: >
  Expert skill for building this Wails v3 desktop app in Go with a React
  frontend. Use whenever a Go desktop app, webview app, window, service,
  binding, or GUI is being built, reviewed, or debugged — any mention of Wails,
  wails3, a frontend calling into Go, or "desktop app in Go". This repo is v3
  ONLY: v2 has an incompatible API that LLMs constantly blend in, so reject
  every v2 construct listed below on sight.
---

# Wails v3: This Repo's Desktop App

Wails builds native desktop applications with a Go backend and a web frontend
rendered in the platform webview (WebKitGTK on Linux, WebKit on macOS, WebView2
on Windows). No bundled Chromium — binaries are megabytes, not hundreds.

**Adapted from [spf13/go-skills](https://github.com/spf13/go-skills) (`wails/`),
trimmed to v3 and to this repo's conventions.**

## Version facts (verified, keep current)

- This repo pins **`v3.0.0-beta.17`** — v3 is **beta**, not alpha. Releases are
  near-nightly off `master` and the API is "stable but correctable".
- **Go 1.25+** is required by the framework.
- The `wails3` CLI is pinned as a **`go tool` dependency** in
  `apps/desktop/go.mod`, so the CLI and the runtime library cannot drift.
  Invoke it as `go tool wails3 …`, never a globally installed `wails3`.
- The npm `@wailsio/runtime` version **must equal** the Go module version. Both
  ship in lockstep; the upstream template's floating `"latest"` was replaced
  with an exact pin in the pnpm catalog.
- Linux needs **GTK4 + WebKitGTK 6.0** (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`).
  The old GTK3 / WebKit2GTK 4.1 stack is legacy, reachable only via
  `-tags gtk3`, and is **removed in v3.1** — do not target it.
- `wails3 init -t react` is the **TypeScript** template. The bare framework name
  is TS; `-js` is the JavaScript variant. There is no `react-ts`.

## This repo has no Taskfiles — moon runs everything

Upstream v3 orchestrates builds with go-task via a generated `Taskfile.yml` plus
`build/<platform>/Taskfile.yml`. **Those were deleted here.** moon is the only
task runner, and the per-OS build/package logic lives in `/scripts`:

| Instead of        | Run                                  |
| ----------------- | ------------------------------------ |
| `wails3 dev`      | `moon run desktop:dev`               |
| `wails3 build`    | `moon run desktop:build`             |
| `wails3 package`  | `moon run desktop:package`           |
| `wails3 task …`   | — (no Taskfiles exist)               |
| generate bindings | `moon run desktop-frontend:bindings` |

`wails3 build`, `wails3 package` and `wails3 dev` are thin wrappers around
`wails3 task <x>`, so they **do not work here**. Every other subcommand does —
`generate bindings|icons|syso|appimage`, `tool package|lipo|has`,
`update build-assets`, `doctor`, `sign` — because those are compiled into the
CLI rather than implemented in the Taskfiles.

Build flags were ported verbatim from the upstream per-OS Taskfiles into
`scripts/wails-build.sh`. **On a wails3 upgrade, re-scaffold into a scratch dir
and diff those Taskfiles against the script** before changing anything.

## Canonical main.go

```go
//go:embed all:frontend/dist
var assets embed.FS

func main() {
    application.RegisterEvent[string]("time")

    app := application.New(application.Options{
        Name: appName,
        Services: []application.Service{
            application.NewService(&services.DemoService{}),
        },
        Assets: application.AssetOptions{
            Handler: application.AssetFileServerFS(assets),
        },
    })

    app.Window.NewWithOptions(application.WebviewWindowOptions{Title: appName})

    if err := app.Run(); err != nil { // blocks; must be the main goroutine
        slog.Error("application exited with an error", "error", err)
        os.Exit(1)
    }
}
```

Two repo-specific details the upstream template gets differently:

- **`log.Fatal` is banned** by `forbidigo`; use `slog` + `os.Exit(1)`.
- **`init()` is banned** by `gochecknoinits`. `RegisterEvent` moved to the top
  of `main()`; the binding generator still finds it (verified — bindings report
  the event either way).

The embed directive needs the **`all:`** prefix, or Go skips files beginning
with `.` or `_` — exactly what a bundler emits. The app then builds, runs, and
serves a blank window in production while working in dev. Enforced by
`go-wails-embed-missing-all-prefix`.

`frontend/dist/.gitkeep` is committed on purpose: `//go:embed` fails to
**compile** when the directory is absent, which would break `go build`,
golangci-lint and `go test` on a fresh clone.

## Use the Manager API

Managers hang off the app; the flat methods were an alpha-era shape that older
tutorials and LLM training data are full of:

```go
app.Window      // New(), NewWithOptions(), GetByName(), GetAll(), Current()
app.Event       // Emit(), On(), OnApplicationEvent()
app.Dialog      // Info/Error/Question/OpenFile/SaveFile
app.Menu  app.SystemTray  app.Clipboard  app.Screen
app.KeyBinding  app.Browser  app.Logger  app.Updater
```

Never `app.NewWebviewWindowWithOptions(...)` or `app.OnEvent(...)`. Enforced by
`go-wails-flat-manager-api`. If a method seems missing, it moved under a
manager — check the changelog rather than inventing it.

## Services: the v3 binding model

A service is a plain struct; exported methods are bound. Two optional
lifecycle hooks are duck-typed:

```go
func (s *S) ServiceStartup(ctx context.Context, o application.ServiceOptions) error
func (s *S) ServiceShutdown() error
```

- Startup runs in registration order; **returning an error aborts app startup**.
  Shutdown runs in reverse order.
- Services are **singletons shared across every window** — guard mutable state
  with a mutex.
- **Register pointers**, `application.NewService(&S{})`. By value, methods run
  on a copy and pointer-receiver methods are not bound at all. Enforced by
  `go-wails-service-registered-by-value`.
- A service can serve HTTP by implementing `http.Handler` and registering with
  `application.NewServiceWithOptions(svc, application.ServiceOptions{Route: "/files"})`.

**Services are thin adapters.** Domain logic lives in `libraries/` with zero
Wails imports, so it is testable as ordinary Go — `libraries/core` is the
worked example, and `apps/desktop/services/demo.go` is the adapter over it.
Enforced by `go-wails-application-import-containment`.

## Bindings

```sh
moon run desktop-frontend:bindings   # go tool wails3 generate bindings -clean=true -ts -i
```

- Output mirrors Go import paths:
  `frontend/bindings/github.com/raphaeldiscky/wails-template/apps/desktop/services/`.
  Import it through the `#bindings` alias (declared in the frontend's
  `package.json` `imports` and `tsconfig.json` `paths`) so a package move is a
  one-line change.
- **Bindings are committed to git.** The frontend build cannot resolve
  `./bindings` without them, and CI fails on drift.
- Owned by the **frontend** moon project, not the Go one. The binary embeds
  `frontend/dist` while the frontend imports Go-generated bindings; if each
  project owned one half, moon's project graph would be cyclic.
- **Never hand-edit** — `-clean` wipes the directory first.
- Go doc comments carry through into the generated TSDoc, so they are worth
  writing well.
- A Go `(value, error)` return becomes a promise that **rejects** with the
  error's message. Handle it — `libraries/ts-shared`'s `attempt()` turns a
  rejection into a `Result` to branch on.

## Events

```go
app.Event.Emit("time", value)                       // Go -> JS
```

```ts
const off = Events.On("time", (e) => setTime(e.data)); // payload is on e.data
return off; // unsubscribe on unmount
```

`application.RegisterEvent[T]("name")` gives the generator a typed frontend API.
Cancellable window hooks are the "confirm before close" pattern:
`window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel() })`.

## Dev mode

`moon run desktop:dev` replaces `wails3 dev`. The contract, read from wails3's
own `dev.go`:

- `WAILS_VITE_PORT` (9245) — vite must listen here with `strictPort: true`.
- **`FRONTEND_DEVSERVER_URL`** — a **non-production** build reads this and
  proxies assets to vite instead of serving the embedded FS. This is why
  `scripts/wails-build.sh` omits the `production` tag when `DEV=true`.

`scripts/wails-dev.sh` exports both, starts vite, waits for the port, then hands
over to `air` (`.air.toml`) for Go rebuilds. Browsing `localhost:9245` directly
renders the UI but **bindings do not work there** — by design.

## Reject on sight (v2 API in a v3 repo)

Enforced by `go-wails-v2-api-in-v3`. Blending versions is the most common Wails
failure mode, and training data is saturated with v2:

- any `github.com/wailsapp/wails/v2/...` import
- `wails.Run(&options.App{...})` — v3 is `application.New(application.Options{...})`
- `Bind: []interface{}{...}` — v3 is `Services: []application.Service{...}`
- `runtime.EventsEmit(ctx, ...)` / `runtime.EventsOn(ctx, ...)` and the stored-`ctx`
  pattern — v3 needs no context threading; use `app.Event.*`
- frontend imports from `wailsjs/go/...` — v3 is `frontend/bindings/...`
- `wails dev` / `wails build` — and in this repo, not `wails3 build` either

## Security

- **The frontend is untrusted input.** Validate arguments in bound methods
  exactly as you would an HTTP request body — `DemoService.Greet` bounds the
  name length for this reason. Never build a shell command or file path from a
  raw frontend string; `go-wails-exec-in-service` flags `exec` in a service.
- Secrets stay in Go. Anything in `frontend/dist` ships readable to the user,
  including every `VITE_*` value.
- Return errors from bound methods; don't panic and don't swallow.
- Long work in a bound method blocks that promise, not the UI — emit progress
  events rather than making the frontend poll.
