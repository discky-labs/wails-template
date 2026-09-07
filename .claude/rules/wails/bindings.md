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
