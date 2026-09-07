# Rule: Wails v3 only

This repo targets **Wails v3 exclusively** (`github.com/wailsapp/wails/v3`,
pinned `v3.0.0-beta.17`). v2 has an incompatible API. Blending the two is the
most common Wails failure mode, and it is especially common in generated code
because training data is saturated with v2.

**Write pure v3. If existing code mixes the two, that is the bug — flag it
before doing anything else.**

## ❌ Reject on sight

Enforced by Semgrep `go-wails-v2-api-in-v3`:

- **Any `github.com/wailsapp/wails/v2/...` import.** The module is not a
  dependency; any mention is a mistake.
- **`wails.Run(&options.App{...})`** → v3 is
  `application.New(application.Options{...})`.
- **`Bind: []interface{}{...}`** → v3 is
  `Services: []application.Service{application.NewService(&Svc{})}`.
- **`runtime.EventsEmit(ctx, ...)` / `runtime.EventsOn(ctx, ...)`** and the
  stored-`ctx` pattern → v3 threads no context; use `app.Event.Emit` /
  `app.Event.On`.
- **`pkg/runtime` imports** → the v3 equivalents are managers on `app`.
- **Frontend imports from `wailsjs/go/...`** → v3 emits `frontend/bindings/...`.

## ❌ Flat (alpha-era) methods

Enforced by Semgrep `go-wails-flat-manager-api`. v3 moved these onto managers;
the flat form reads plausible and is all over older tutorials:

| Don't                                  | Do                                  |
| -------------------------------------- | ----------------------------------- |
| `app.NewWebviewWindowWithOptions(...)` | `app.Window.NewWithOptions(...)`    |
| `app.NewWebviewWindow()`               | `app.Window.New()`                  |
| `app.OnEvent(...)`                     | `app.Event.On(...)`                 |
| `app.OnApplicationEvent(...)`          | `app.Event.OnApplicationEvent(...)` |
| `app.NewMenu()`                        | `app.Menu.New()`                    |
| `app.NewSystemTray()`                  | `app.SystemTray.New()`              |

If a method appears to be missing, it moved under a manager. Check the
[changelog](https://v3.wails.io/changelog) rather than inventing a signature.

## Beta discipline

- v3 is **beta**, not alpha — but releases are near-nightly off `master` and
  the API is "stable but correctable".
- **The version is pinned in three places that must move together:**
  `apps/desktop/go.mod` (`require` + the `wails3` `tool` directive) and the
  `@wailsio/runtime` entry in the pnpm catalog. The npm package and the Go
  module ship in lockstep.
- When a v3 call fails to compile and the fix is not obvious, **check the
  current docs before guessing** — training data lags a nightly beta by design.
- Linux targets **GTK4 + WebKitGTK 6.0**. The GTK3 / WebKit2GTK 4.1 stack is
  legacy (`-tags gtk3`) and is **removed in v3.1**; do not target it.
