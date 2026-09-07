# Rule: Wails services

A service is a plain struct whose **exported methods are callable from the frontend**.
Registered in `application.Options.Services`, they live in `apps/desktop/services/`.

This rule owns the **shape and lifecycle** of a service. Its API surface is
`wails/api-design.md`; input validation and secrets are `go/security.md`.

## Thin adapters over Wails-free code

Domain logic lives in `libraries/` and **imports no Wails package**, so it is testable as
ordinary Go. A service validates input, calls into a library, and maps the result.
`libraries/core` + `apps/desktop/services/demo.go` are the worked example.

Enforced by Semgrep `go-wails-application-import-containment`:
`github.com/wailsapp/wails/v3/pkg/application` must not be imported outside
`apps/desktop/`.

- ❌ No domain logic in a service method body beyond validation and mapping.
- ❌ No `libraries/**` file importing `pkg/application`. If a library needs to reach the
  UI, return a value or an error and let the service translate it.

## Registration

- **Register pointers**: `application.NewService(&DemoService{})`. By value, methods run
  on a copy — accumulated state is silently discarded and pointer-receiver methods are not
  bound at all. Enforced by Semgrep `go-wails-service-registered-by-value`.
- Dependencies go in through plain constructors
  (`application.NewService(NewNoteService(db))`). **No DI framework** — the upstream
  architecture guide suggests Wire; this repo does not use it. "Inject dependencies" here
  means a constructor parameter, nothing more.
- **A service that needs data takes `repository.DataStore`**, never a `*sql.DB` and never a
  concrete adapter. It holds no SQL and opens no transaction of its own —
  `go/persistence.md` owns that boundary; `services/notes.go` is the example.
- **A service that emits events needs the app itself**, which cannot be referenced inside
  `application.Options`. Register it after construction:
  `app.RegisterService(application.NewService(NewClockService(app)))` —
  `apps/desktop/services/clock.go` is the example.

## Lifecycle

Both hooks are optional and duck-typed:

```go
func (s *S) ServiceStartup(ctx context.Context, o application.ServiceOptions) error
func (s *S) ServiceShutdown() error
```

- Startup runs in **registration order**; returning an error **aborts app startup**. Keep
  it fast — do the I/O here once rather than per call, but do not block on the network.
- Shutdown runs in **reverse order**; the app context is already canceled, and an error
  only logs a warning. Always release what startup acquired, and **wait** for anything it
  started.
- The `ctx` passed to startup is the app's lifetime. A goroutine that outlives a request
  should select on it — `go/performance-and-concurrency.md` owns goroutine ownership.

## Shared state

Services are **singletons shared across every window**, so concurrent calls are the
default assumption. `go/performance-and-concurrency.md` owns the locking rules; the short
version is guard the state, and never hold the lock across I/O.

## Go house rules that bite here

`.golangci.yml` runs ~60 linters; two catch newcomers in this package:

- **`wrapcheck`** — an error crossing a package boundary needs context. `go/errors.md`
  owns error style.
- **`forbidigo`** bans `fmt.Print*` and `log.*`. Use `slog`.
