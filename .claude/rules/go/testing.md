# Rule: Testing

Three tiers, and this repo already has one of each — copy the existing example rather than
inventing a shape.

Patterns: [`.claude/skills/wails-best-practices`](../../skills/wails-best-practices/SKILL.md)
section 7.

## Unit — the default, and where most tests belong

- ✅ **Test the Wails-free packages directly.** `libraries/core` and `libraries/config`
  need no webview, so they are fast and deterministic. This is the payoff for keeping
  domain code out of `apps/` (`structure.md`).
- ✅ **`package <pkg>_test`**, so a test can only reach the exported surface. If a test
  needs an unexported symbol, that is a signal the API is wrong, not that the test needs
  privileges.
- ✅ **`t.Parallel()`** in every test, and **`t.TempDir()`** for anything touching disk —
  never a shared fixture directory, which makes parallel tests flaky.
- ✅ **Table-driven** for input/output permutations; `libraries/core/math_test.go` is the
  example.

## Integration — through the exported service, with real dependencies

- ✅ Drive the service the way the frontend does: call its exported methods, back it with
  a real store in a temp dir. `apps/desktop/services/preferences_test.go` is the example —
  it even restarts a second service over the same file to prove persistence.
- The top tier is `moon run desktop:smoke`, which **launches the built binary** on all
  three platforms. A binary that compiles can still die on a missing webview library or a
  failed `ServiceStartup`, and only running it catches that.

## Concurrency is tested, not assumed

- ✅ **`-race` is the gate** — `.moon/tasks/go.yml` runs `go test -race -cover ./...`.
- ✅ **Assert the concurrent access you claim to support.**
  `TestConcurrentAccessIsRaceFree` hammers a service from two goroutines; without it the
  mutex is decoration.
- ✅ **Assert that goroutines exit.** `clock_test.go` fails if `ServiceShutdown` stops
  waiting for its ticker — a leak is otherwise invisible until production.

## Mock at the dependency, never at Wails

- ✅ Define the interface **where it is consumed**, and mock that: a repository, a clock,
  an HTTP client. On the frontend side, mock the `#bindings` module — the boundary — as
  `greet-form.test.tsx` does.
- ❌ **Never mock the Wails runtime itself.** If a test needs a fake `*application.App`,
  the logic is in the wrong place; move it into a library and test it there.
- ❌ No mock for a type with one real implementation and no branching — that tests the
  mock.
