# wails-template

A **Wails v3 + Go + React** template for multiplatform desktop apps
(Linux, macOS, Windows), wired to a moonrepo monorepo with proto-pinned
toolchains.

## What's inside

- **Wails v3** (`v3.0.0-beta.17`) — Go backend, native webview, no bundled
  Chromium. The `wails3` CLI is pinned as a `go tool` dependency so it can
  never drift from the runtime library.
- **React 19 + Vite 8 + TypeScript 7** with TanStack Router, TanStack Query
  and Tailwind v4.
- **moon** as the single task runner. There are **no Taskfiles** — see
  [Build system](#build-system).
- **proto** pins go / node / pnpm / moon in `.prototools`, so every machine
  and CI job runs the same versions.
- **pnpm catalog** — one place per dependency version.
- Quality gates: golangci-lint (~60 linters), oxlint + oxfmt, TypeScript
  project references, vitest, knip, syncpack, typos, and a
  [Semgrep suite](.semgrep/README.md) for Wails v3 invariants.
- CI builds **and runs** the app on all three platforms; a tag produces
  installers for each.

## Stack

| Concern           | Tool                                       |
| ----------------- | ------------------------------------------ |
| Desktop shell     | Wails v3 (WebKitGTK 6 / WebKit / WebView2) |
| Backend           | Go 1.26                                    |
| Frontend          | React 19, Vite 8, TypeScript 7             |
| Routing / data    | TanStack Router, TanStack Query            |
| Styling           | Tailwind v4 (CSS-first)                    |
| Task runner       | moon                                       |
| Toolchain pinning | proto (`.prototools`)                      |
| Package manager   | pnpm workspaces + catalog                  |
| Go lint / format  | golangci-lint                              |
| TS lint / format  | oxlint, oxfmt                              |
| Test              | vitest, `go test`                          |
| Dead code         | knip (TS), `go tool deadcode` (Go)         |
| Static analysis   | Semgrep (`.semgrep/`)                      |
| Git hooks         | lefthook + commitlint                      |

## Layout

```
.
├── apps/desktop/            # the Wails app (its own Go module)
│   ├── main.go              # application.New + window + services
│   ├── services/            # thin Wails adapters over libraries/
│   ├── build/               # wails3 build assets (icons, plists, nsis, nfpm)
│   ├── .air.toml            # Go watcher for dev mode
│   └── frontend/            # React app (own pnpm + moon project)
│       ├── src/routes/      # TanStack Router file routes
│       ├── src/styles/      # THE palette — every colour lives here
│       └── bindings/        # generated from Go — COMMITTED
├── libraries/
│   ├── core/                # Go domain logic, zero Wails imports
│   ├── config/              # preferences persisted to the OS config dir
│   └── ts-shared/           # @local/ts-shared
├── scripts/                 # task bodies (build, package, dev, bindings…)
├── .moon/                   # workspace + inherited task definitions
├── .semgrep/                # repo-specific static analysis
└── .claude/skills/wails/    # the Wails v3 skill
```

## Setup

Requires [proto](https://moonrepo.dev/proto) and Go on your PATH.

```sh
proto install                # go, node, pnpm, moon at the pinned versions
moon run root:install-tools  # golangci-lint, lefthook, deps, git hooks
go tool wails3 doctor        # verify the platform webview dependencies
```

Platform prerequisites (`wails3 doctor` prints the exact command):

| OS      | Needs                                                             |
| ------- | ----------------------------------------------------------------- |
| Linux   | `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`    |
| macOS   | `xcode-select --install`                                          |
| Windows | WebView2 runtime (preinstalled on Win 10/11); NSIS for installers |

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

## Build system

Upstream Wails v3 drives builds with go-task through a generated
`Taskfile.yml` plus one per platform. **This template deletes them** and moves
the same work into moon tasks and `/scripts`, so there is one task runner
rather than two.

That means `wails3 build`, `wails3 package` and `wails3 dev` — each just a
wrapper around `wails3 task <x>` — do not work here. Use the moon tasks above.
Every other subcommand still works, because those are compiled into the CLI:
`generate bindings|icons|syso|appimage`, `tool package|lipo`,
`update build-assets`, `doctor`, `sign`.

The per-OS build flags in `scripts/wails-build.sh` are ported **verbatim** from
the upstream Taskfiles. On a wails3 upgrade, scaffold a throwaway project
(`wails3 init -n scratch -t react`) and diff its Taskfiles against the script
before changing anything.

## Tasks

Three inheritance tiers, same as the tooling this borrows from:

1. **Workspace** — `moon.yml`: `typos`, `check-consistency`, `deadcode`,
   `audit`, `semgrep`, markdown `format`, `install-tools`, `sync-deps`,
   `upgrade`.
2. **Per-language** — `.moon/tasks/{go,go-library,typescript}.yml`: the
   `build` / `test` / `lint` / `format` / `typecheck` / `tidy` set every
   project of that language inherits.
3. **Per-project** — `<project>/moon.yml`: what only that project does.

`moon ci :<task>` matches the root-scoped and every per-project variant, so one
CI step covers Go and TypeScript together.

Keep task bodies to a line or two; real logic belongs in `scripts/*.sh`, with
the script listed as a task input so a script edit invalidates the cache.

## Conventions worth knowing

- **Generated files are committed**: `frontend/bindings/` and
  `src/routeTree.gen.ts`. CI fails if either is stale. Both are excluded from
  oxlint/oxfmt/knip.
- **`frontend/dist/.gitkeep` is tracked** because `//go:embed all:frontend/dist`
  fails to _compile_ when the directory is missing.
- **Domain logic imports no Wails.** `libraries/core` is plain Go;
  `apps/desktop/services` adapts it. Semgrep enforces this.
- **Register services as pointers** — by value, state is silently discarded.
- **Internal TS packages export source**, not `dist/`, so vite and vitest read
  through pnpm symlinks with no build step. Typechecking layers TS project
  references on top, and moon syncs the `references` from workspace deps.
- **Dependency versions live in the pnpm catalog** (`pnpm-workspace.yaml`).
  `@wailsio/runtime` must stay equal to the Go module version.
- **Every colour lives in `frontend/src/styles/`**, in two layers: `--swatch-*`
  appearance names hold the literals (each hex written once), and role tokens
  (`--background`, `--border`) alias them. Components name roles, never
  literals and never raw Tailwind palette utilities. `main.go`'s window
  background is checked against `--background`, because it is painted before
  the webview loads. See `.claude/rules/frontend/design-tokens.md`.

## Conventions index

The rules under [`.claude/rules/`](.claude/rules/README.md) are the short,
enforceable version of this README. They are auto-loaded by Claude Code, but
they are written for humans too — each states what not to do and why, and names
the check that enforces it.

| Rule                                                                   | Covers                                             | Enforced by                                                  |
| ---------------------------------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------------ |
| [`wails/v3-only.md`](.claude/rules/wails/v3-only.md)                   | v3 is the only target; the v2 constructs to reject | Semgrep `go-wails-v2-api-in-v3`, `go-wails-flat-manager-api` |
| [`wails/services.md`](.claude/rules/wails/services.md)                 | Service shape, registration, trust boundary        | Semgrep + golangci-lint                                      |
| [`wails/bindings.md`](.claude/rules/wails/bindings.md)                 | Generated bindings and the Go↔TS boundary          | CI `codegen-drift` job                                       |
| [`wails/build-and-dev.md`](.claude/rules/wails/build-and-dev.md)       | moon is the task runner; build and dev contracts   | `moon ci`, the 3-OS matrix                                   |
| [`frontend/design-tokens.md`](.claude/rules/frontend/design-tokens.md) | Every colour in one folder; the two-layer palette  | `moon run root:check-design-tokens`                          |

## More

- [`.semgrep/README.md`](.semgrep/README.md) — the static-analysis suite: six
  Wails v3 rules, each with a fixture
- [`.claude/skills/wails/SKILL.md`](.claude/skills/wails/SKILL.md) — the fuller
  Wails v3 reference, for humans and agents
- [Wails v3 docs](https://v3.wails.io)

## License

MIT
