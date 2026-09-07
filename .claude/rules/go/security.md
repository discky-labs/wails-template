# Rule: Security

**Every exported method on a registered service is reachable from the frontend**, which is
untrusted input — the same trust boundary as an HTTP handler taking a request body. This
rule owns that requirement; `wails/api-design.md` says where validation goes.

Techniques and code: [`.claude/skills/wails-best-practices`](../../skills/wails-best-practices/SKILL.md)
section 3.

## Validate and bound every argument

- ✅ **Reject, don't coerce.** Check length, range and allowed set, and return an error.
  `DemoService.Greet` bounds the name length; `config.Preferences.Normalise` clamps
  out-of-range values rather than failing the whole file.
- ✅ Trim and normalise _before_ validating, so `"  "` fails the empty check.
- ❌ **Don't escape on the way into storage.** `html.EscapeString` on a stored value
  corrupts the data and buys nothing: React escapes when it renders. Escaping is the
  renderer's job.

## Filesystem paths

✅ **Resolve, then confine.** Take the absolute cleaned path and require it to sit under an
allowed root:

```go
abs, err := filepath.Abs(filepath.Clean(path))
// then require strings.HasPrefix(abs, allowedRoot)
```

❌ A `strings.Contains(path, "..")` check is **not** the control. It misses encoded
traversal and rejects legitimate names; the resolved-prefix check is what holds. Reject
symlinks that escape the root if the root is security-relevant.

❌ **Never build a path or a shell command from a raw frontend string.** Semgrep
`go-wails-exec-in-service` flags `exec` inside a service.

## Secrets

- ❌ **Never in `libraries/config`** — it is plaintext JSON in the user's config dir.
- ❌ **Never in the frontend bundle.** Everything reachable there, including every
  `VITE_*` value, ships readable to the user. Secrets stay in Go.
- ✅ Use the **OS keychain** for anything that must persist.
- ❌ Don't log secrets, tokens or whole request payloads.

## Authenticating a bound call

A desktop app has no cookie or header to lean on, so if operations need protection:

- ✅ **Authenticate in the service**, not the frontend. A UI that hides a button has not
  protected anything — the method is still callable from the devtools console.
- ✅ Hash passwords with **bcrypt**; store sessions with an **expiry** and check it on
  every protected call, not just at login.
- ✅ Encrypt sensitive data at rest (AES-GCM), with the key from the keychain.
- ❌ Don't return "wrong password" vs "no such user" — one message for both.

## Rate limiting and outbound calls

- ✅ **Rate-limit anything a frontend loop could hammer** — and remember a bound method is
  trivially callable in a `for` loop. Bound the limiter's own memory; a map keyed by
  caller with no eviction is itself the leak.
- ✅ **HTTPS only**, with `&tls.Config{MinVersion: tls.VersionTLS12}` and a client
  `Timeout` set — Go's default client waits forever.
- ❌ Never skip certificate verification, including "just for dev".
