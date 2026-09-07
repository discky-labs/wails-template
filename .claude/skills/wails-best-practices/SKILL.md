---
name: wails-best-practices
description: >
  Wails v3 best practice reference: bindings API design, performance patterns
  (batching, pagination, caching, event streaming), security patterns, error and
  panic handling, profiling with pprof and runtime/trace, architecture patterns,
  and testing. Use when designing or reviewing a service or bound method,
  deciding what crosses the Go/JS boundary, hunting a performance problem,
  profiling or tracing the app, handling panics, or hardening input validation,
  file paths and secrets. Complements the always-loaded rule at
  .claude/rules/go/performance-and-concurrency.md, which carries the hard
  constraints; this skill carries the patterns and recipes.
---

# Wails v3 Best Practices

Mirrors the structure of the official
[Bindings Best Practices](https://v3.wails.io/features/bindings/best-practices/),
[Performance](https://v3.wails.io/guides/performance/),
[Security](https://v3.wails.io/guides/security/),
[Panic Handling](https://v3.wails.io/guides/panic-handling/),
[Debugging](https://v3.wails.io/guides/dev/debugging/),
[Architecture](https://v3.wails.io/guides/architecture/) and
[Coding Standards](https://v3.wails.io/contributing/standards/) pages, so a claim here can
be checked against its source. **Repo adaptations are at the end** — a few doc examples
fail this repo's linters verbatim.

## 0. The shape of an app

Migrated here when the separate `wails` skill was retired for restating the rules.

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

Two places the upstream template differs from what this repo's linters allow:
`log.Fatal` is banned by `forbidigo` (use `slog` + `os.Exit(1)`), and `init()` is
banned by `gochecknoinits`, so `RegisterEvent` sits at the top of `main()` — the
binding generator finds it either way (verified).

`apps/desktop/main.go` additionally installs a `PanicHandler` and an opt-in
`runtime/trace` block; see sections 4 and 5.

**The manager inventory.** Everything hangs off the app — if a method seems
missing it moved under a manager, so check the changelog rather than inventing a
signature. The flat forms (`app.NewWebviewWindowWithOptions`, `app.OnEvent`) are
an alpha-era shape that `wails/v3-only.md` rejects and Semgrep enforces:

```go
app.Window      // New(), NewWithOptions(), GetByName(), GetAll(), Current()
app.Event       // Emit(), On(), OnApplicationEvent()
app.Dialog      // Info/Error/Question/OpenFile/SaveFile
app.Menu        app.SystemTray   app.Clipboard   app.Screen
app.KeyBinding  app.Browser      app.Logger      app.Updater
app.Env         app.ContextMenu
```

## 1. API design

**Single responsibility.** One service, one purpose. The god service is the named
anti-pattern:

```go
// ❌ Does everything
type AppService struct{}
func (a *AppService) SaveFile(path string, data []byte) error
func (a *AppService) GetUser(id int) (*User, error)
func (a *AppService) ProcessPayment(amount float64) error

// ✅ Focused
type FileService struct{}
type UserService struct{}
type PaymentService struct{}
```

**Clear, action-oriented names.** `SaveDocument`, `AuthenticateUser`, `CreateOrder` — not
`Do`, `Handle`, `Process`.

**Consistent returns.** Always `(T, error)`; never a bare `any` or a silent failure. The
error becomes a rejected promise carrying its message, so it is user-visible — write it
accordingly and don't leak internals.

**Validate on the Go side.** The frontend is untrusted input. See
`.claude/rules/wails/services.md`; `DemoService.Greet` bounds name length as the example.

**A service that emits events needs the app.** It cannot appear in `application.Options`
(the app does not exist yet), so register it after construction — the pattern the docs
prescribe and `apps/desktop/main.go` uses for `ClockService`:

```go
app := application.New(application.Options{ /* … */ })
app.RegisterService(application.NewService(services.NewClockService(app)))
```

## 2. Performance patterns

Ordered as the official page orders them. **This order is the point** — batching first,
parallelism nowhere on the list.

**Batch operations.** Reduce bridge crossings:

```go
// ✅ One crossing for N items
func (s *Service) ProcessItems(items []Item) ([]Result, error)
```

**Paginate — cursor first.** Never `FindAll()`. Default to a **cursor (keyset)** page,
not page numbers. The official example uses page/size; prefer this unless something
genuinely needs numbered pages:

```go
// ✅ The default: opaque cursor, no total count
type UserPageRequest struct {
    Cursor string `json:"cursor"` // empty for the first page
    Limit  int    `json:"limit"`
}

type UserPage struct {
    Items      []User `json:"items"`
    NextCursor string `json:"nextCursor"` // empty when exhausted
}
```

Three reasons cursor wins at scale:

- **Correct under concurrent writes.** `OFFSET` is computed against a moving set, so an
  insert or delete between requests silently duplicates or skips rows. A keyset cursor
  reads "everything after this exact record" and cannot drift.
- **Constant cost per page.** `WHERE (sort_key, id) > (?, ?) ORDER BY sort_key, id
LIMIT ?` walks the index. `OFFSET n` scans and discards n rows first, so page 500
  costs 500× page 1.
- **No `COUNT(*)`.** Producing `TotalPages` is often the most expensive part of the
  query, and it is the part nobody looks at.

Use numbered pages only when the product actually needs them: a visible "page 7 of 20",
jump-to-page, or a small bounded set where the count is cheap and meaningful. Then keep
`Page`/`PageSize` plus `TotalItems`/`TotalPages` as the docs show — and clamp both.

**Cache expensive reads.** An LRU with a TTL for hot lookups. In-repo example:
`PreferencesService` reads the file once in `ServiceStartup` rather than per call.

**Stream with events instead of polling:**

```go
// ❌ frontend: setInterval(() => GetProgress(), 100)
// ✅ Go: emit as you go
func (s *Service) ProcessLargeFile(path string) error {
    // … s.app.Event.Emit("progress", pct) …
}
```

**Frontend.** Bundle size (`manualChunks`, `terser` with `drop_console`), code splitting
via `lazy` + `Suspense`, compressed images. This repo already enables TanStack Router's
`autoCodeSplitting: true` in `apps/desktop/frontend/vite.config.ts`.

**Debounce frequent frontend events** (~300 ms) and **batch on the Go side** with a mutex
plus `time.AfterFunc` when many small updates arrive.

## 3. Security patterns

**Input.** Validate everything at the boundary — length, range, allowed set. Reject
rather than coerce.

**File paths.** Resolve, then confine:

```go
abs, err := filepath.Abs(filepath.Clean(path))
if err != nil { return err }
if !strings.HasPrefix(abs, allowedDir) { return errors.New("access denied") }
```

The docs also suggest rejecting any path containing `..`. Treat that as belt-and-braces,
**not** the control: it both misses encoded traversals and rejects legitimate names. The
resolved-prefix check is what actually holds.

**Secrets** go in the OS keychain (e.g. `github.com/zalando/go-keyring`), never in
`libraries/config` (plaintext JSON) and never in the frontend bundle.

**Passwords** with `bcrypt`. **Sensitive data at rest** with AES-GCM.

**Network.** HTTPS only, and pin a floor:
`&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}`.

**Rate-limit** anything a frontend loop could hammer. Don't log secrets, and don't let
error messages leak internals.

Escaping is the renderer's job, not the store's — React escapes by default, so
`html.EscapeString` on the way _into_ storage corrupts data without adding safety.

## 4. Errors and panics

**Errors:** wrap with `%w` for context (`wrapcheck` enforces it here), custom error types
where callers need to branch, early returns over deep nesting.

**The Wails panic model** — the part worth knowing:

- Wails **automatically recovers** panics in bound service methods and in its own
  runtime. A `recover` in a bound method is redundant.
- It does **not** cover goroutines you start. Those need their own `recover` —
  `apps/desktop/services/clock.go` shows it.
- Recovered panics are routed to `PanicHandler`, receiving
  `PanicDetails{StackTrace, Error, Time, FullStackTrace}`. `StackTrace` is trimmed to
  your code; `FullStackTrace` includes runtime frames.
- **The default handler prints a banner and quits.** `apps/desktop/main.go` installs one
  that logs structured detail and continues, because the panic is already contained; call
  `app.Quit()` there instead to fail hard.

## 5. Profiling and traces

**Execution traces** answer "where did the time actually go". This repo ships it opt-in —
`WAILS_TRACE=1` writes `trace.out` (see `startTrace` in `apps/desktop/main.go`):

```sh
WAILS_TRACE=1 ./apps/desktop/bin/desktop     # quit the app normally
go tool trace trace.out                       # opens the trace viewer
go tool trace -pprof=syscall trace.out > s.pprof   # or extract a profile
```

- **Quit the app normally.** A `SIGKILL`/`SIGTERM` skips the stop hook and truncates the
  trace.
- Some views need **Graphviz** — verify with `dot -V`
  (`brew install graphviz` / `apt-get install graphviz` / `pacman -S graphviz`).
- The **Syscall profile** view is the most useful starting point.
- The default trace is raw; add context with `trace.WithRegion`, `trace.NewTask` and
  `trace.Log`, and narrow what you record.

**pprof** for CPU and heap:

```go
pprof.StartCPUProfile(f); defer pprof.StopCPUProfile()
runtime.GC(); pprof.WriteHeapProfile(f)
```

```sh
go tool pprof -http=:8080 cpu.prof
```

## 6. Architecture

**Service layer** over Wails-free domain packages — `libraries/core` and
`libraries/config` here, adapted by `apps/desktop/services`. **Repository** interfaces
when persistence gets non-trivial.

On state: the upstream guide reaches for one centralised store guarded by
`sync.RWMutex`. In a monorepo prefer state owned by the library that defines it
(`structure.md`); a shared store is for values genuinely read across features, and then
`go/performance-and-concurrency.md`'s locking rules apply.

Do: separate concerns, keep services focused, pass dependencies through
constructors, handle errors. Don't: god objects, tight coupling, **ignore
concurrency**, over-engineer.

("Inject dependencies" in the upstream guide means Wire; here it means a constructor
parameter — see `wails/services.md`.)

## 7. Testing

Unit-test the Wails-free packages directly (`libraries/core`, `libraries/config` — no
webview needed). Test services through their exported methods with a temp dir, as
`preferences_test.go` does. Assert concurrency with `-race`
(`TestConcurrentAccessIsRaceFree`) and goroutine exit explicitly (`clock_test.go`). Mock
at the boundary the service depends on, not at Wails.

## Repo adaptations

Where a doc example fails verbatim here:

| The docs show                                      | This repo requires                                                                       |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `fmt.Printf` in `PanicHandler` and examples        | `forbidigo` bans `fmt.Print*` → use `slog`                                               |
| `gofmt` + `goimports`                              | `.golangci.yml` also runs gofumpt, gci and golines                                       |
| Wire for dependency injection                      | Plain constructors, no DI framework (`wails/services.md`)                                |
| An event bus doing `go handler(data)` per listener | Unbounded, no `recover`, no lifecycle — violates the performance guide's own leak advice |
| "the shipped Taskfile passes `-ldflags="-s -w"`"   | There is no Taskfile; `scripts/wails-build.sh` owns the build flags                      |
| `interface{}` in signatures                        | `any`                                                                                    |
| A package-level `var` for a lookup table           | `gochecknoglobals` — return it from a function                                           |
| `init()` for registration                          | `gochecknoinits` — do it at the top of `main()`                                          |
