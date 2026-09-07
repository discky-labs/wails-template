# Rule: Errors and panics

An error returned from a bound method becomes a **rejected promise whose message the user
may see**. That makes error text part of the UI, and error _type_ part of the API.

Code and patterns: [`.claude/skills/wails-best-practices`](../../skills/wails-best-practices/SKILL.md)
section 4.

## Wrap with context

- ✅ `fmt.Errorf("load %s: %w", path, err)` — say what was being attempted. Bare
  `return os.ReadFile(path)` gives the UI "no such file or directory" with no subject.
- `%w`, never `%v`, so `errors.Is`/`errors.As` still work up the stack. `wrapcheck`
  enforces wrapping across package boundaries.
- ❌ Don't leak internals into the message: no SQL, no absolute paths from the developer's
  machine, no secrets. It may be rendered verbatim.

## Type the errors callers branch on

- ✅ **Sentinel** for a condition with no payload:
  `var ErrEmptyName = errors.New("name must not be empty")`, matched with `errors.Is`.
  `libraries/config`'s `ErrUnknownTheme` is the in-repo example.
- ✅ **Struct type** when the caller needs the detail — a `ValidationError{Field}` or
  `NotFoundError{Resource, ID}` — matched with `errors.As`.
- ❌ **Never branch on the message string.** That is not an API, it is a coincidence.

## Recover deliberately, and know what not to retry

- ✅ Retry only what is **transient** — I/O, network, a lock contention — with a backoff.
- ❌ **Never retry a validation error.** The input will not become valid on the second
  attempt; retrying turns a clear rejection into a slow one.
- ✅ Degrade rather than die where the user still has something usable:
  `PreferencesService.ServiceStartup` logs a corrupt file and starts with defaults instead
  of aborting the app.

## Panics

- Wails **recovers panics in bound methods and in its own runtime** and routes them to the
  `PanicHandler` in `apps/desktop/main.go`, so a `recover` in a bound method is redundant.
- It does **not** cover goroutines you start — `go/performance-and-concurrency.md` owns
  that requirement.
- **The default handler prints a banner and quits.** This repo's handler logs structured
  detail via `slog` and _continues_, because the panic is already contained and killing the
  window would discard the user's unsaved state. If a panic should be fatal in your app,
  call `app.Quit()` from the handler — that is a deliberate choice, not a default.
- ❌ Don't panic for a runtime condition. Bad input, a missing file and a failed network
  call are `error` values. `panic` is for programming-error invariants only.
