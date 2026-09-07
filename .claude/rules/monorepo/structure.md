# Rule: Folder and file structure

Cross-cutting: this covers the monorepo shape, the Go side and the frontend. The aim is
that the layout at 200 files looks like the layout at 20 — you add folders, you do not
reorganise.

## The monorepo shape

```
apps/<app>/            a deployable. Owns wiring, owns nothing reusable.
libraries/<name>/      reusable, and IMPORTS NO WAILS. One concern per library.
scripts/               task bodies (moon tasks stay ≤2 lines)
.claude/ .semgrep/     conventions and their enforcement
```

**The dividing line:** if two things could ever both need it, it belongs in
`libraries/`. `apps/` holds composition and adapters. `libraries/core` (domain) and
`libraries/config` (persistence) are the worked examples, and
`go-wails-application-import-containment` enforces the no-Wails half.

**Promote to a library when** a second consumer appears, or the code has a test suite
that does not need the app. **Not** in anticipation — a library with one consumer and no
tests is a folder with extra steps.

**Adding a project is four steps, and skipping one fails CI:** create the folder with a
`moon.yml`; register it in `.moon/workspace.yml` `projects.sources`; add it to `go.work`
(Go) or `pnpm-workspace.yaml` (TS); for Go, add a `replace` in the consumer's `go.mod`
because `go mod tidy` cannot resolve an unpublished sibling. See `libraries/config` as
the most recent example.

## Go: one service per file, thin

```
apps/desktop/
├── main.go            entry point + wiring ONLY
├── services/          one file per service, named after it
│   ├── clock.go
│   ├── demo.go
│   ├── notes.go
│   └── preferences.go
├── internal/          app-private, not importable from outside
│   ├── entity/        stored shapes + their sentinel errors
│   └── repository/    ports, adapters, SQL and migrations
└── build/             wails3-owned; do not hand-edit generated parts
```

- **One file per service**, named for the service (`preferences.go` →
  `PreferencesService`). Why one service per concern is `wails/api-design.md`.
- **`main.go` wires, it does not implement.** If logic is accumulating there, it is a
  service. The `ClockService` extraction is precisely this move.
- **`main()` translates an error into an exit status; `run()` does the work.** `os.Exit`
  does not run deferred functions, so cleanup written next to an `os.Exit` never happens.
  Keeping the only `os.Exit` in `main` means every defer in `run` is honored.
- **`internal/` holds what only this app may import**, and grows a folder per layer rather
  than a file per concept. `go/persistence.md` owns its contents.
- **There is no `handler/` layer.** In a Wails app the bound service _is_ the handler — it
  validates, calls inward, and maps errors. A microservice separates the two because a
  transport exists to separate; here there is none.
- **Domain types live with the domain**, in the library that owns them — not in a shared
  `models/` bucket. A `models/` folder that every service imports becomes the coupling
  it was meant to prevent. (The architecture guide shows `models/`; at monorepo scale a
  per-library type is better.)
- **Split a service** when it grows a second responsibility. Length alone is not the
  trigger — a long file of closely related methods is fine, and unrelated method groups
  are not fine at any length.
- Tests sit beside the code as `<name>_test.go`, package `<pkg>_test` so they exercise
  only the exported surface.

## Frontend: feature folders, thin routes

Organise by **feature**, never by component type.

```
apps/desktop/frontend/src/
├── routes/            thin bindings only — createFileRoute + component
├── components/
│   ├── <feature>/     one folder per surface, once it earns one
│   └── {ui,layout}/   cross-cutting buckets, same depth as features
├── hooks/<domain>/    hooks used by more than one feature
├── styles/            THE palette (see frontend/design-tokens.md)
├── lib/               framework-free helpers
└── env.ts
```

- **A route file declares the binding and nothing else** — no JSX body, no
  sub-components, no helpers. `routes/index.tsx` is the reference: it composes
  components and owns no markup of its own beyond layout.
- **Flat until it earns a folder.** `components/greet-form.tsx` is correct at this size.
  Promote to `components/<feature>/` when a third related file appears, or when a
  feature grows its own hook or skeleton.
- **Filenames kebab-case, exported symbol PascalCase and matching** — `theme-select.tsx`
  → `ThemeSelect`, so "find file by name" works.
- **Hooks colocate with their only consumer**; move to `hooks/<domain>/` on the second.
  `use-go-clock.ts` sits in `hooks/` because it is about the Go event stream, not one
  component.
- **`index.ts` is for re-exports only.** Never a component in `index.tsx` — it destroys
  "find file by name".

### ❌ Anti-patterns

- **Type-first folders** — `components/forms/`, `components/cards/`. They scatter one
  feature across the tree.
- **Area umbrella folders** wrapping a group of features.
- **Cross-importing between sibling feature folders** — hoist the shared piece to
  `components/ui/` or `hooks/`.
- **A `utils/` or `helpers/` grab bag.** Name the concern.
- **Generated output edited by hand** — `wails/bindings.md` owns which files are
  generated and why they are still committed.
