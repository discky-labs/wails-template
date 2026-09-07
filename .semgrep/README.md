# `.semgrep/` — repo-specific static analysis

Semgrep covers what golangci-lint **cannot express**: this repo's own
semantics. `.golangci.yml` already runs ~60 linters over the generic Go
concerns, so a rule only earns a place here if no linter can state it.

Every rule here encodes a **Wails v3** invariant. This repo targets v3 only.

Before adding a rule, check it isn't already covered:

| Tempting rule                    | Already enforced by                  |
| -------------------------------- | ------------------------------------ |
| `math/rand` for secrets          | `gosec` G404                         |
| unwrapped / mis-wrapped errors   | `errorlint` + `wrapcheck` + `revive` |
| `fmt.Print*` / `log.Fatal*`      | `forbidigo`                          |
| missing `case` on an enum switch | `exhaustive`                         |
| an `init()` function             | `gochecknoinits`                     |
| a repeated string literal        | `goconst`                            |
| unused exported func             | `:deadcode` (`go tool deadcode`)     |
| unused TS export / dependency    | `:deadcode` (knip)                   |

## Layout

```
.semgrep/
├── README.md                 # this file
├── .semgrepignore            # keep fixtures out of scans run from here
└── rules/
    └── go/                   # language first, then category
        ├── security/         # exploitable flaws at the frontend trust boundary
        ├── architecture/     # v3 API shape + layering invariants
        └── quality/          # idioms whose failure mode is silent
```

This mirrors the official
[`semgrep/semgrep-rules`](https://github.com/semgrep/semgrep-rules) registry:
**language first, category second, no per-rule folder, files named after the
rule.** One rule per YAML file.

The naming is load-bearing: Semgrep pairs a rule with its test by filename
stem, so `path/to/foo.yaml` looks for `path/to/foo.go`. Adding TypeScript
rules later means `rules/ts/…`, no reshuffle.

## The rules

**security** — the frontend is untrusted input

| Rule id                    | Catches                                  |
| -------------------------- | ---------------------------------------- |
| `go-wails-exec-in-service` | process execution inside a bound service |

Every exported method on a registered service is callable from the frontend —
the same trust boundary as an HTTP handler. `exec` there is a
command-injection sink the moment a method parameter reaches it.

**architecture** — v3 API shape and layering

| Rule id                                   | Catches                                                    |
| ----------------------------------------- | ---------------------------------------------------------- |
| `go-wails-v2-api-in-v3`                   | v2 imports, `runtime.EventsEmit`, `wails.Run`, `Bind:`     |
| `go-wails-flat-manager-api`               | `app.NewWebviewWindowWithOptions` etc. instead of managers |
| `go-wails-application-import-containment` | `pkg/application` imported outside the desktop app         |

`go-wails-v2-api-in-v3` is the one that matters most. v2 and v3 have
incompatible APIs, and blending them is the most common Wails failure mode —
especially for LLM-generated code, since training data is full of v2.

`go-wails-flat-manager-api` exists because the flat methods were real during
the alphas: `app.NewWebviewWindowWithOptions(...)` reads perfectly plausible
and is all over older tutorials.

`go-wails-application-import-containment` is the layering rule this template is
built around — domain logic lives in `libraries/` with no Wails import, and
`apps/desktop/services/` holds thin adapters over it. That is what keeps the
domain testable as ordinary Go.

**quality** — idioms whose failure mode is silent

| Rule id                                | Catches                                          |
| -------------------------------------- | ------------------------------------------------ |
| `go-wails-embed-missing-all-prefix`    | `//go:embed frontend/dist` without `all:`        |
| `go-wails-service-registered-by-value` | `NewService(Svc{})` instead of `&Svc{}`          |
| `go-ticker-without-stop`               | `time.NewTicker` with no `Stop` in that function |

`go-ticker-without-stop` catches a ticker that holds a runtime timer for the life of the
process. `apps/desktop/services/clock.go` is the compliant shape (`defer ticker.Stop()`),
and the rule was proven both ways: zero findings on the real tree, and it fires on a
violation planted at a real path.

The other two ship broken apps that pass every other check. Without `all:`, Go skips
files starting with `.` or `_` — exactly what a bundler emits — so the app
builds, runs, and serves a blank window in production while working in dev.
A service registered by value has its methods called on a copy, so state is
silently discarded and pointer-receiver methods are not bound at all.

## What is deliberately NOT here

- **"Validate every bound-method parameter."** OSS taint mode is
  intra-procedural, so proving a parameter reaches a sink only works when both
  sit in one function. A syntactic approximation would flag every service
  method that touches its own arguments.
- **"Don't call a manager before `app.Run()`."** Ordering, not shape.
- **Dev-vs-production asset paths.** Decided by build tags, which the build
  script owns.

## Adding a rule

1. Pick `rules/go/<category>/<name>` — `<name>` short and readable; the rule
   **id** inside the YAML stays long and globally unique (`go-wails-<what>`),
   since ids are what appear in CI output and `nosemgrep` comments.
2. Write `<name>.go` **first** — `// ruleid: <id>` above each violation,
   `// ok: <id>` above each compliant case. Draw ok-cases from real code in
   this repo, not from imagination; that is what catches over-matching.
3. Write `<name>.yaml` (one rule, `languages: [go]`, `paths.exclude` for
   sanctioned sites).
4. Iterate on that rule alone until green:

   ```sh
   cd .semgrep/rules/go/<category> && semgrep --test --config <name>.yaml <name>.go
   ```

5. `moon run root:semgrep-test && moon run root:semgrep` — the full suite must
   pass and the repo must report **zero** findings. A finding on real code is a
   false positive to fix before merging, not a backlog item.

## Traps this suite already hit

- **`semgrep --test` ignores `paths:` entirely.** A rule scoped to
  `**/services/**` still matches a fixture living nowhere near that path.
  Convenient — but a glob matching _nothing_ passes its tests just as happily
  while finding nothing in a real scan. Globs resolve against the **scan root**
  (`apps libraries`), so a leading `apps/` matches nothing. Both path-scoped
  rules here were proven by planting a violation at a real path and running a
  real scan; `--test` cannot tell you.
- **A real scan is limited to files tracked by git.** A rule verified against
  uncommitted files has not been verified. Use `--no-git-ignore` when probing.
- **Semgrep reports a grouped `import (...)` at its opening line**, so an AST
  import pattern cannot point at the offending spec and its `// ruleid:`
  annotation never lines up. Import rules use `pattern-regex` on the quoted
  path instead.
- **`pattern-regex` has no lookahead.** `go-wails-embed-missing-all-prefix`
  matches whitespace followed immediately by the path rather than asserting the
  absence of `all:`.
- **A pattern containing `: ` breaks the YAML scanner.** Quote it:
  `- pattern: "application.Options{..., Bind: $B, ...}"`.
- **OSS taint mode is intra-procedural.** Source and sink must be in the same
  function; interprocedural analysis needs Semgrep Pro.

## Running

```sh
moon run root:semgrep        # scan apps/ + libraries/ (CI + pre-push)
moon run root:semgrep-test   # rule regression suite (CI)
```

Semgrep is an optional local tool (`pip install semgrep`);
`scripts/semgrep.sh` skips with a warning when it is absent, so it never
blocks a local commit. CI installs it, so the gate holds where it counts.
