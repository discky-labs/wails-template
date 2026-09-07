# `.claude/rules/` — slim, always-loaded rules

Every file here is **auto-loaded into every Claude session**, so every line is
a tax on every conversation regardless of what is being worked on. Keep each
file focused and **≤ 200 lines**.

Rules state hard constraints. The fuller reference — canonical `main.go`, the
manager list, the dev-mode contract — lives in
[`.claude/skills/wails/SKILL.md`](../skills/wails/SKILL.md), which loads
on-demand. Prefer a pointer here over a paraphrase.

Several rules are mechanically enforced — by the Semgrep suite in
[`.semgrep/`](../../.semgrep/README.md) or by a check script — and those name
the rule id or task so a CI failure is traceable back to the reason.

Rules are grouped by topic folder, so a second app or a backend concern lands
beside its peers rather than in a flat list. A rule that spans every language —
`structure.md` — sits at the top level instead. For the rule-versus-skill split, see
[`../skills/README.md`](../skills/README.md): if it changes a decision it is a rule, if
it shows how it is a skill.

| File                                | Covers                                               |
| ----------------------------------- | ---------------------------------------------------- |
| `wails/v3-only.md`                  | v3 is the only target; the v2 constructs to reject   |
| `wails/services.md`                 | Service shape, registration, trust boundary          |
| `wails/bindings.md`                 | Generated bindings and the Go↔TS boundary            |
| `wails/build-and-dev.md`            | moon is the task runner; build and dev contracts     |
| `frontend/design-tokens.md`         | Every colour in one folder; the two-layer palette    |
| `go/performance-and-concurrency.md` | Measure first; bridge crossings; goroutine ownership |
| `structure.md`                      | Folder and file layout; when to promote to a library |
