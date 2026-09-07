# Rule: Wails services

A service is a plain struct whose **exported methods are callable from the
frontend**. Registered in `application.Options.Services`, they live in
`apps/desktop/services/`.

## Thin adapters over Wails-free code

Domain logic lives in `libraries/` and **imports no Wails package**, so it is
testable as ordinary Go. A service validates input, calls into a library, and
maps the result. `libraries/core` + `apps/desktop/services/demo.go` are the
worked example.

Enforced by Semgrep `go-wails-application-import-containment`:
`github.com/wailsapp/wails/v3/pkg/application` must not be imported outside
`apps/desktop/`.

- ❌ No domain logic in a service method body beyond validation and mapping.
- ❌ No `libraries/**` file importing `pkg/application`. If a library needs to
  reach the UI, return a value or an error and let the service translate it.

## Registration

- **Register pointers**: `application.NewService(&DemoService{})`. By value,
  methods run on a copy — accumulated state is silently discarded and
  pointer-receiver methods are not bound at all. Enforced by Semgrep
  `go-wails-service-registered-by-value`.
- Dependencies go in through plain constructors
  (`application.NewService(NewNoteService(db))`). **No DI framework.**
- Services are **singletons shared across every window**. Guard mutable state
  with a mutex.

## Lifecycle

Both hooks are optional and duck-typed:

```go
func (s *S) ServiceStartup(ctx context.Context, o application.ServiceOptions) error
func (s *S) ServiceShutdown() error
```

- Startup runs in **registration order**; returning an error **aborts app
  startup**. Keep it fast.
- Shutdown runs in **reverse order**; the app context is already cancelled, and
  an error only logs a warning. Always release what startup acquired.

## The frontend is untrusted input

Every exported method is a trust boundary — treat it exactly as an HTTP
handler taking a request body.

- **Validate arguments.** Bound length, reject empty, check ranges.
  `DemoService.Greet` bounds the name length for this reason.
- ❌ **Never build a shell command or filesystem path from a raw frontend
  string.** Semgrep `go-wails-exec-in-service` flags `exec` inside a service.
- **Return errors; do not panic and do not swallow.** A returned error becomes
  a rejected promise carrying its message, so the message is user-visible —
  write it accordingly, and don't leak internals into it.
- Secrets stay in Go. Anything reachable from the frontend bundle is readable
  by the user.
- Long work blocks that promise, not the UI. Emit progress events rather than
  making the frontend poll.

## Go house rules that bite here

`.golangci.yml` runs ~60 linters; two catch newcomers in this package:

- **`wrapcheck`** — an error crossing a package boundary needs context:
  `fmt.Errorf("divide %d by %d: %w", a, b, err)`.
- **`forbidigo`** bans `fmt.Print*` and `log.*`. Use `slog`.
