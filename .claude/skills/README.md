# `.claude/skills/` — on-demand references

A skill loads **only when a task matches its `description`**, so it costs nothing until
it is needed. That is the whole difference from a rule.

|              | `.claude/rules/`                       | `.claude/skills/`                            |
| ------------ | -------------------------------------- | -------------------------------------------- |
| **Loaded**   | always, every session                  | on demand, when the description matches      |
| **Cost**     | every line taxes every conversation    | zero until invoked                           |
| **Content**  | hard constraints — "always X, never Y" | procedure and reference: code, recipes       |
| **Length**   | ≤ 200 lines, terse                     | long is fine                                 |
| **The test** | "would a session go wrong without it?" | "is this needed only while doing that task?" |

Practically: if it changes a **decision**, it is a rule. If it shows **how**, it is a
skill. A rule that carries a code sample or a CLI recipe is usually a skill with the
wrong filename.

Rules should point here rather than paraphrase — see
[`../rules/README.md`](../rules/README.md).

| Skill                                                   | Covers                                                                   |
| ------------------------------------------------------- | ------------------------------------------------------------------------ |
| [`wails`](wails/SKILL.md)                               | The Wails v3 API: services, bindings, events, dev and build model        |
| [`wails-best-practices`](wails-best-practices/SKILL.md) | API design, performance, security, panics, profiling and traces, testing |
