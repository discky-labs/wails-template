# Rule: Performance and concurrency

Constraints only. The recipes — pprof, `runtime/trace`, pagination and caching
shapes — live in [`.claude/skills/wails-best-practices`](../../skills/wails-best-practices/SKILL.md),
which loads on demand.

## Measure before you change anything

The Wails performance guide's first ✅ Do is **"Profile before optimising"** and its
first ❌ Don't is **"Don't optimise prematurely."** Follow that order: a change with no
measurement behind it is a guess, and a guess that adds concurrency is a guess that also
adds a race.

## Reduce bridge crossings first — that is the lever, not goroutines

The official bindings best-practices page lists its performance patterns in order, and
**#1 is "Batch Operations — reduce bridge calls"**, then pagination, then caching, then
events. Parallelism is not on the list.

- ✅ **Batch.** One call carrying N items beats N calls carrying one.
  `for (const i of items) await Process(i)` is N crossings; `await ProcessItems(items)`
  is one.
- ✅ **Paginate, cursor-first.** Never return `FindAll()`. Default to an opaque
  cursor + limit, not page numbers: offset pagination skips or duplicates rows when
  the set shifts under it, and its cost grows with the offset. Numbered pages only
  when the UI genuinely shows "page 7 of 20".
- ✅ **Stream with events, never poll.** A frontend `setInterval` calling `GetProgress()`
  is a hot loop across the boundary; emit progress instead.
- ❌ Don't return huge datasets. ❌ Don't block the main thread.

Adding a goroutine inside a bound method is a **semantics change, not a speedup**: the
call stops awaiting a result and becomes fire-and-forget, which then obliges you to emit
progress and completion events. Do it when the work is genuinely long or I/O-bound, not
to "make it faster".

## Every goroutine needs an owner and an exit

A bare `go func() { for { ... } }()` is the shape the performance guide names as a memory
leak. In this repo the owner is a service: start in `ServiceStartup` bound to its `ctx`,
stop in `ServiceShutdown`, and have shutdown **wait** for the goroutine so teardown is
deterministic.

`apps/desktop/services/clock.go` is the reference implementation — a `time.Ticker` with
`defer ticker.Stop()`, a `select` on `ctx.Done()` and an explicit stop channel, and
`clock_test.go` fails if the goroutine ever stops exiting.

- ❌ No `time.NewTicker` without `Stop`. ❌ No goroutine started from a bound method
  without a completion or progress event — the frontend cannot tell success from silence.
- ✅ **Bounded fan-out.** `errgroup.SetLimit` or a semaphore channel; one `go func` per
  item is how a file picker DoSes the disk.

## A panic in a background goroutine is NOT recovered by Wails

Wails recovers panics in **bound service methods** and in its own runtime, then routes
them to `PanicHandler` (configured in `apps/desktop/main.go`). It does **not** cover a
goroutine you started. Every long-lived goroutine carries its own
`defer func() { if r := recover(); r != nil { … } }()`, or one bad iteration takes the
whole process down. This is the least obvious rule here.

## Shared state: assume concurrent access, and never lock across I/O

- **Never assume bound calls are serialised.** A service is a singleton shared across
  every window, so guard mutable state with a mutex regardless of whether you added
  concurrency yourself. `go test -race` is the gate (`.moon/tasks/go.yml`).
- ✅ **`sync.RWMutex` for state read more than written**, and a **second, plain mutex to
  serialise writers** when a write involves I/O. That is two locks doing two jobs, not
  belt-and-braces.
- ❌ **Never hold either lock across a syscall.** A reader must not queue behind a file
  write. `PreferencesService` is the reference: `writeMu` spans the read-modify-write
  while `mu` is released across the disk write, so `Get()` never waits on I/O.
- ✅ `sync.Pool` only under **measured** allocation pressure, never as a default.

## Secrets

`go/security.md` owns this. The one performance-shaped consequence: caching a decrypted
secret in a service field keeps it in memory for the life of the app — fetch it from the
keychain when needed instead.
