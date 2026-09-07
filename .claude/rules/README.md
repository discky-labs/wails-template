# `.claude/rules/` — slim, always-loaded rules

Every file here is **auto-loaded into every Claude session**, so every line is
a tax on every conversation regardless of what is being worked on. Keep each
file focused and **≤ 200 lines**.

Rules state hard constraints. The fuller reference — canonical `main.go`, the
manager list, the dev-mode contract — lives in
[`.claude/skills/wails/SKILL.md`](../skills/wails/SKILL.md), which loads
on-demand. Prefer a pointer here over a paraphrase.

Several rules are mechanically enforced by the Semgrep suite in
[`.semgrep/`](../../.semgrep/README.md); those name their rule id so a
violation is traceable from CI output back to the reason.

| File                     | Covers                                             |
| ------------------------ | -------------------------------------------------- |
| `wails/v3-only.md`       | v3 is the only target; the v2 constructs to reject |
| `wails/services.md`      | Service shape, registration, trust boundary        |
| `wails/bindings.md`      | Generated bindings and the Go↔TS boundary          |
| `wails/build-and-dev.md` | moon is the task runner; build and dev contracts   |
