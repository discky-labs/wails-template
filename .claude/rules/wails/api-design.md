# Rule: Bound API design

Every exported method on a registered service is public API: the frontend calls it, the
binding generator types it, and changing it breaks a caller you cannot see from Go. Design
it deliberately.

Patterns and code live in
[`.claude/skills/wails-best-practices`](../../skills/wails-best-practices/SKILL.md).

## One service, one purpose

❌ **No god service.** A single type carrying `SaveFile`, `GetUser` and `ProcessPayment`
is the anti-pattern the official page opens with. Split by responsibility —
`FileService`, `UserService`, `PaymentService` — one file each
(`monorepo/structure.md`).

**Split** when a service grows a second responsibility, not when it grows long.

## Name for the action

✅ `SaveDocument`, `AuthenticateUser`, `CreateOrder`.
❌ `Do`, `Handle`, `Process`, `Manage` — and anything whose meaning depends on knowing the
receiver.

## Always `(T, error)`

- ✅ **Return an error explicitly**, even when today it cannot fail. Adding one later is a
  breaking change to every call site.
- ❌ No bare `any`/`interface{}` returns. ❌ No silent failure — a method that swallows
  the reason gives the UI nothing to say.
- The error shape on the JS side is `wails/bindings.md`'s business; that rule owns it.

## Validate before you act

The frontend is untrusted input. `go/security.md` owns the requirement and the
techniques — this rule only says _where_: in the bound method, before anything else runs.

## Document methods

✅ **Write the doc comment.** It is copied into the generated TSDoc, so it is what the
frontend author reads at the call site. A bound method with no comment ships an untyped
mystery to the other side of the boundary.

## Plan for change

- ✅ **Version deliberately.** Adding a method or a struct field is safe; renaming or
  reordering parameters silently breaks the frontend at runtime, because the binding is
  regenerated and nothing type-checks the old call. When a signature must change
  incompatibly, add the new method and remove the old one in a separate step.
- ❌ **Don't expose internals.** Unexported types, `*sql.DB`, file handles and framework
  objects must not appear in a bound signature. Return the value the UI needs, not the
  machinery that produced it.
- ❌ **Don't hardcode** what belongs in configuration — see `libraries/config`.

## The summary, as the official page states it

✅ Single responsibility · clear naming · validate input · return errors · batch
operations · use events for streaming · sanitise input · test thoroughly · document
methods · version your API.

❌ No god objects · don't trust the frontend · don't return huge datasets · don't block ·
don't ignore errors · don't skip testing · don't hardcode · don't expose internals.

The performance half (batching, pagination, caching, events) is
`go/performance-and-concurrency.md`.
