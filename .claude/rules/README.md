# `.claude/rules/` — slim, always-loaded rules

Every file here is **auto-loaded into every Claude session**, so every line is
a tax on every conversation regardless of what is being worked on. Keep each
file focused and **≤ 200 lines**.

Rules state hard constraints. The patterns, recipes and code — the canonical
`main.go`, the manager inventory, profiling and traces — live in
[`.claude/skills/wails-best-practices`](../skills/wails-best-practices/SKILL.md),
which loads on demand. Prefer a pointer here over a paraphrase.

Several rules are mechanically enforced — by the Semgrep suite in
[`.semgrep/`](../../.semgrep/README.md) or by a check script — and those name
the rule id or task so a CI failure is traceable back to the reason.

Rules are grouped by topic folder, so a second app or a backend concern lands
beside its peers rather than in a flat list — including `monorepo/`, for rules about the
repository itself rather than one language. For the rule-versus-skill split, see
[`../skills/README.md`](../skills/README.md): if it changes a decision it is a rule, if
it shows how it is a skill.

| File                                | Covers                                               |
| ----------------------------------- | ---------------------------------------------------- |
| `monorepo/structure.md`             | Folder and file layout; when code earns a library    |
| `wails/v3-only.md`                  | v3 is the only target; the v2 constructs to reject   |
| `wails/api-design.md`               | Bound API surface: naming, returns, versioning       |
| `wails/services.md`                 | Service shape, registration, lifecycle               |
| `wails/bindings.md`                 | Generated bindings and the Go↔TS boundary            |
| `wails/build-and-dev.md`            | moon is the task runner; build and dev contracts     |
| `go/performance-and-concurrency.md` | Measure first; bridge crossings; goroutine ownership |
| `go/errors.md`                      | Error wrapping, typed errors, retry, panics          |
| `go/security.md`                    | Trust boundary, paths, secrets, auth, rate limiting  |
| `go/testing.md`                     | Unit / integration / mock tiers; `-race` as the gate |
| `frontend/design-tokens.md`         | Every colour in one folder; the two-layer palette    |
