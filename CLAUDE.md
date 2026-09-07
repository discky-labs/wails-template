# CLAUDE.md

A **Wails v3 + Go + React** multiplatform desktop template on a moonrepo
monorepo with proto-pinned toolchains.

## Repo shape

| Path                     | What                                                          |
| ------------------------ | ------------------------------------------------------------- |
| `apps/desktop/`          | the Wails app — own Go module, `main.go`, services            |
| `apps/desktop/services/` | thin Wails adapters; every method is frontend-callable        |
| `apps/desktop/frontend/` | React 19 + Vite 8 + TanStack Router (own pnpm + moon project) |
| `apps/desktop/build/`    | wails3 build assets (icons, plists, nsis, nfpm)               |
| `libraries/core/`        | Go domain logic, **zero Wails imports**                       |
| `libraries/ts-shared/`   | `@local/ts-shared`                                            |
| `scripts/`               | task bodies (build, package, dev, bindings, smoke)            |
| `.moon/`                 | workspace config + inherited task definitions                 |
| `.semgrep/`              | repo-specific static analysis                                 |

## Important rules — consult BEFORE doing common tasks

`.claude/rules/` is auto-loaded; these are the load-bearing ones:

| Doing this                     | Read this                              |
| ------------------------------ | -------------------------------------- |
| Anything with the Wails API    | `.claude/rules/wails/v3-only.md`       |
| Adding or changing a service   | `.claude/rules/wails/services.md`      |
| Touching generated bindings    | `.claude/rules/wails/bindings.md`      |
| Build, dev mode, packaging, CI | `.claude/rules/wails/build-and-dev.md` |
| Needing the full v3 reference  | `.claude/skills/wails/SKILL.md`        |
| Adding a Semgrep rule          | `.semgrep/README.md`                   |

## Hard don'ts

- ❌ **No Wails v2 API.** This repo is v3 only. No `wails/v2` import, no
  `wails.Run`, no `Bind:`, no `runtime.EventsEmit(ctx, …)`.
- ❌ **No flat window/event methods.** `app.Window.NewWithOptions(...)`, not
  `app.NewWebviewWindowWithOptions(...)`.
- ❌ **No Taskfile, and no second task runner.** moon runs everything;
  `wails3 build|package|dev` do not work here.
- ❌ **No `pkg/application` import under `libraries/`.** Domain code stays
  Wails-free.
- ❌ **No service registered by value** — `NewService(&Svc{})`.
- ❌ **No hand-edits to `frontend/bindings/` or `*.gen.ts`.** Regenerate.
- ❌ **No unvalidated frontend input**, and never into `exec` or a file path.
- ❌ **No `fmt.Print*` / `log.*`** (forbidigo) and no `init()` (gochecknoinits)
  — use `slog`.
- ❌ **No colour literal** outside `apps/desktop/frontend/src/styles/`, and no
  raw Tailwind palette utility (`bg-neutral-950`) for a design decision — name
  a role (`bg-background`).
- ❌ **No `npm`/`yarn`** — pnpm only, and dependency versions live in the
  **pnpm catalog**, not in a package.json.

## Daily commands

```sh
moon run desktop:dev            # vite + auto-rebuilding Go binary
moon run desktop:build          # production binary -> apps/desktop/bin/
moon run desktop:smoke          # launch it and assert it stays up
moon run desktop:package        # AppImage + deb + rpm | .app/dmg | NSIS

moon run :format                # auto-fix formatting everywhere
moon run :lint :typecheck :test # the triage trio
moon ci                         # the full CI pipeline, locally

moon run desktop-frontend:bindings  # regenerate Go -> TS bindings
moon run root:semgrep               # Wails-specific static analysis
moon run root:check-design-tokens   # one source of truth for colour
moon query projects                 # what moon can see
```

Single test escape hatches:
`pnpm --filter @local/ts-shared exec vitest run src/result.test.ts`,
`go test ./libraries/core/ -run TestDivide`.

## Task inheritance

1. **Workspace** — `moon.yml`: `typos`, `check-consistency`, `deadcode`,
   `audit`, `semgrep`, markdown `format`, `install-tools`, `sync-deps`, `upgrade`.
2. **Per-language** — `.moon/tasks/{go,go-library,typescript}.yml`.
3. **Per-project** — `<project>/moon.yml`.

`moon ci :<task>` matches the root-scoped and every per-project variant, so one
CI step covers Go and TypeScript. Keep task bodies ≤ ~2 lines; logic goes in
`scripts/*.sh`, listed as a task input.

## Two structural facts that look odd until explained

- **`apps/desktop` and `apps/desktop/frontend` are separate moon projects**,
  declared explicitly in `.moon/workspace.yml` `projects.sources`. moon
  inherits quality tasks per-project by toolchain/language, so one project
  cannot pick up both the Go and the TypeScript task sets.
- **The frontend project owns bindings generation**, not the Go one. The binary
  embeds `frontend/dist` while the frontend imports Go-generated bindings; if
  each owned one half, moon's project graph would be cyclic.

## Generated files that ARE committed

`apps/desktop/frontend/bindings/**`, `src/routeTree.gen.ts`, and
`frontend/dist/.gitkeep`. CI fails on drift for the first two. All are excluded
from oxlint/oxfmt/knip.

## TypeScript

`tsconfig.base.json` (extends `@tsconfig/strictest`) →
`tsconfig.bundler.json` → per-project. Consequences worth knowing:
`import type` required, **no `enum`** or value namespaces
(`erasableSyntaxOnly`), `process.env["FOO"]` bracket access,
`noUncheckedIndexedAccess`, and never pass an explicit `undefined`
(`exactOptionalPropertyTypes`).

Internal packages export **source**, not `dist/`, so vite and vitest read
through pnpm symlinks with no build step. Project references are moon-synced
from `workspace:*` deps — run `moon sync` after changing a workspace dep, then
restore any tsconfig comments moon stripped.

## House rules

- Conventional commits, **lowercase subject** (`feat: add x`, never
  `feat: Add x`).
- Pin versions. `@wailsio/runtime` must equal the Go module's wails version.
- A finding from lint or Semgrep is a thing to fix, not a backlog item.
