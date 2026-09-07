# Rule: Bindings (the Go ↔ TypeScript boundary)

`wails3 generate bindings` turns every exported service method into a typed TS
function. Run it through moon:

```sh
moon run desktop-frontend:bindings
```

## Hard rules

- ❌ **Never hand-edit anything under `frontend/bindings/`.** Generation runs
  with `-clean`, which wipes the directory first.
- ✅ **Bindings are committed to git.** The frontend build cannot resolve
  `./bindings` without them, and CI fails on drift (`codegen-drift` job).
  Regenerate and commit in the same change as the Go edit.
- ✅ **Import through the `#bindings` alias**, never the raw generated path.
  The output mirrors Go import paths
  (`frontend/bindings/github.com/raphaeldiscky/wails-template/apps/desktop/services/`),
  so a package move would otherwise touch every import. The alias is declared
  in the frontend's `package.json` `imports` and `tsconfig.json` `paths`.
- ✅ **Write good Go doc comments** — they carry through into the generated
  TSDoc and are what the frontend author reads.

## A bound method takes `ctx context.Context` first

Wails **injects** it, and the generator **drops it from the TS signature**:

```go
func (s *NotesService) Add(ctx context.Context, title, body string) (entity.Note, error)
```

```ts
Add(title: string, body: string): $CancellablePromise<entity.Note>
```

- ✅ **Take a ctx on any method that does I/O.** It is the frontend promise's cancellation,
  so a query stops when the caller stops waiting — and it is what makes the generated
  return a `$CancellablePromise`. Free, and correct.
- ✅ It must be the **first** parameter. `bindings.go` only sets `needsContext` for
  parameter 0; anywhere else it becomes a real argument the frontend has to pass.
- ❌ Don't store a ctx on the service struct to avoid the parameter. `ServiceStartup`'s ctx
  is the app lifetime, not the call's.

## Go is the source of truth, so codegen runs one way only

The Go signature **is** the contract; TS is derived from it. That is the opposite of a
spec-first HTTP service, and it is why there is no OpenAPI document here and no
`oapi-codegen` in server mode — there is no HTTP API to serve. Adding one would mean
inventing a transport purely to have something to generate from.

`oapi-codegen` is the right tool in `generate: client` mode when this app calls a **remote**
service; see `go/persistence.md`.

## Ownership: the frontend project owns generation

The `bindings` task belongs to the **`desktop-frontend`** moon project, not the
Go one, and `scripts/wails-bindings.sh` cds into the Go module to run it.

This is not cosmetic. The Go binary embeds `frontend/dist` while the frontend
imports Go-generated bindings. If each project owned one half, moon's project
graph would be **cyclic** and refuse to build. The frontend owns everything
generated under `frontend/`; `apps/desktop` only consumes `dist/`.

## Error and value shapes

- A Go `(value, error)` return becomes a promise that **rejects** with the
  error's message. Handle it — `libraries/ts-shared`'s `attempt()` converts a
  rejection into a `Result` to branch on, which is why service calls in the UI
  go through it rather than a bare `await`.
- Registering an event's payload type (`application.RegisterEvent[string]("time")`)
  gives the generator a typed frontend listener.
- `Events.On` returns its own unsubscribe function. **Return it from
  `useEffect`**, or a listener stacks up on every hot reload.

## Lint and format exclusions

`frontend/bindings/**` and `**/*.gen.ts` (the TanStack route tree) are excluded
from oxlint, oxfmt and knip. Generated output is whatever the generator emits;
we do not reformat it and we do not make codegen fight the formatter.
