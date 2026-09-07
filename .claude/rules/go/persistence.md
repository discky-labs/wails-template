# Rule: Persistence

How data reaches a disk, and where the code that does it lives. The worked example is
`apps/desktop/internal/` — `entity/` plus `repository/` — with `services/notes.go` as its
only caller.

Concurrency is `go/performance-and-concurrency.md`; error style is `go/errors.md`; input
validation is `go/security.md`. This rule owns **layout, ports, SQL, transactions and
migrations**.

## A port needs a second implementation, or a transaction

> An interface earns its keep when there is a real second implementation with a real
> caller. Until then, **the concrete type is the boundary.**

- ✅ **`NoteRepository` is a port** — not because a second database is coming, but because
  `SQLite.Atomic` has to hand the caller a transaction-scoped store, and a test has to
  substitute a fake. Those are real callers today.
- ❌ **Preferences have no port.** `PreferencesService` holds a concrete `*config.Store`,
  and that is correct: one JSON file, one caller, no second implementation. Wrapping it in
  a `Repository` interface would be the speculative abstraction this rule exists to stop.
- ❌ Don't abstract a **provider swap**. Moving SQLite between paths, or Postgres between
  hosts, is a connection-string change. The driver handle IS the boundary.

## Layout, and the names

```
apps/desktop/
├── internal/
│   ├── entity/<entity>.go        the stored shape + its sentinel errors. No driver.
│   └── repository/
│       ├── doc.go               the layer's import contract
│       ├── datastore.go         DataStore, Querier, Open, Atomic, migrate
│       ├── <entity>.go          the PORT: <Entity>Repository
│       ├── <entity>_sqlite.go   the ADAPTER: every SQL statement, as consts
│       └── migrations/          NNNN_<slug>.up.sql, one table per file
└── services/<entity>.go         the Wails-bound caller
```

- **The suffix names the backend.** `<entity>_sqlite.go`, and a second store would be
  `<entity>_pg.go` beside it under the same port. That is what the suffix is for.
- **The adapter type is unexported** (`sqliteNoteRepository`) and reached only through
  `DataStore.Notes()`, so no call site does transaction plumbing.
- **A `models/` bucket is not the answer.** Domain types live with the domain — see
  `monorepo/structure.md`.
- **Persistence belongs to the app, not `libraries/`,** until a second consumer exists.
  One schema with one consumer is app code.

## SQL lives in the adapter, and nowhere else

- ✅ Statements as **package consts** in `*_sqlite.go`, named `<verb><Entity><By>SQL`, with
  a shared `<entity>Columns`. The whole query surface of a table is then readable in one
  place, and a grep can prove nothing escaped.
- ❌ **No SQL literal in a service, a handler or a test.** If a service needs a new query,
  the port gains a method — the service does not gain a query.
- ❌ No ORM. Hand-written SQL through `database/sql`.
- The driver import is contained to this package. `modernc.org/sqlite` is imported for its
  side effect in `datastore.go` **only** — it is pure Go, which is load-bearing: a CGO
  driver would force `CGO_ENABLED=1` on Windows and drag in a cross-compile toolchain
  `scripts/wails-build.sh` deliberately avoids.

## Translate driver errors at the adapter boundary

`sql.ErrNoRows` must not leave `repository/`. It becomes `entity.ErrNoteNotFound` inside
the adapter, so a caller writes `errors.Is(err, entity.ErrNoteNotFound)` and never learns
which database is underneath. Same for a constraint violation.

Wrap the scan error with `%w` rather than replacing it, or the sentinel check upstream
stops working.

## `Atomic` is the only transaction API

```go
err := store.Atomic(ctx, func(tx repository.DataStore) error { ... })
```

- **`Querier` is the seam.** It is satisfied by both `*sql.DB` and `*sql.Tx`, so one
  adapter serves a plain call and a call inside a transaction. No method is written twice.
- ❌ **No exported `Begin`/`Commit`/`Rollback`** alongside it. Two ways to manage a
  transaction is one too many.
- ❌ **Nesting is rejected**, not folded into the outer transaction — that would make the
  inner "commit" a lie. `ErrNestedTransaction`.
- **Validate before opening it.** A rollback is cheap; holding the write lock while
  checking input is self-inflicted contention.
- Roll back from a `defer`, ignoring `sql.ErrTxDone` — that is the expected outcome once
  the commit has happened.

## Migrations ship inside the binary

- `//go:embed migrations/*.up.sql` + `PRAGMA user_version`. That is the entire version
  table: an integer in the DB header, transactional, no dependency.
- **`//go:embed` cannot reach a parent directory**, which is why the files sit inside
  `repository/` rather than a top-level `db/`.
- **Migrate on open.** A desktop app has no operator to run a migrate command, so it
  either migrates at launch or cannot open its own data.
- **Up-only, deliberately.** There is nobody on an end user's laptop to run a rollback.
- One migration per transaction, so a failure leaves neither a half-built schema nor a
  version claiming work that did not happen.
- **Lint the SQL.** `moon run root:sql-lint` (sqlfluff, `dialect = sqlite`) gates every
  `.sql` file, because a malformed migration is not a compile error — it fails at launch on
  a user's machine, with nobody there to fix it. `root:sql-fix` auto-formats.
  `.sqlfluff` has no rule overrides, and should stay that way.
- **`.sql` is a build input.** `//go:embed` puts migrations in the binary, so
  `**/*.sql` is in the `go-sources` file group and in `desktop:build`'s inputs. Without
  that, editing a migration hits a stale cache and the binary keeps the old schema.

## SQLite: two traps worth knowing

- **A stored timestamp must be fixed width.** SQLite compares TEXT byte by byte.
  `time.RFC3339Nano` trims trailing zeros, so `…:00.5Z` sorts _before_ `…:00Z` and
  silently reverses chronological order. Use nine explicit fractional digits, always UTC
  (`timeLayout` in `note_sqlite.go`), or store an integer.
- **Set the pragmas on the DSN**, not once after connecting: they are per-connection and
  the pool opens more than one. `journal_mode(WAL)`, `busy_timeout`, `foreign_keys(ON)`,
  and `_txlock=immediate` — without the last, a transaction that reads before it writes
  can fail on the lock upgrade with `SQLITE_BUSY`, which `busy_timeout` does **not** retry.

## Pagination: cursor-first, and clamp in exactly one place

`go/performance-and-concurrency.md` requires an opaque cursor over page numbers. Two
things this layer has to get right:

- **The keyset predicate carries the whole sort key.** `WHERE (created_at, id) < (?, ?)`
  with a matching index. A timestamp alone is not unique — a coarse clock stamps several
  rows identically — and the tie-breaker is what stops a row repeating across pages.
- **The ceiling is clamped at the service, the floor at the repository.** The service asks
  for `limit+1` as an "is there more" probe; a second clamp in the repository swallows that
  probe row at exactly the maximum page size and ends pagination one page early. This was
  a real bug here — `TestListPaginatesAtTheMaximumPageSize` is its regression test.

## Remote data is an API client, not a database handle

A desktop binary runs on the user's machine, so **an in-process Postgres or Redis client
ships the credentials to every user.** No amount of layering fixes that.

- ✅ Remote shared state goes behind an **HTTP client adapter** — `libraries/<domain>/
apiclient/` — talking to a service that owns the database, with a token obtained at
  runtime. Same port, different failure modes: offline, retry, auth.
- ✅ **That** is where `oapi-codegen` belongs, in `generate: client` mode against the
  remote service's published spec.
- ❌ Not `echo5-server`/`strict-server` mode. This app has no HTTP API to serve: the
  frontend boundary is generated by `wails3 generate bindings` in the opposite direction —
  Go signatures are the source of truth and TS is derived. See `wails/bindings.md`.
- ❌ Never a connection pool to a shared database from the desktop app.

## The composition root owns the handle

`Open` returns the concrete `*SQLite` so that only `run()` in `main.go` can `Close` it —
`DataStore` has no `Close` at all. Acquire, defer the release immediately, and let the
defers unwind LIFO. `os.Exit` skips defers, so it appears in `main` and nowhere else.
