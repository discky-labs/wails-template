# Rule: Colour lives in one folder

Every colour in this repo is declared in
**`apps/desktop/frontend/src/styles/`** and nowhere else. A literal anywhere
else silently stops tracking the palette the moment the palette moves.

Enforced by `moon run root:check-design-tokens` (CI + pre-push).

## The two layers

`tokens.css` holds the palette in two layers, and the split is load-bearing:

- **`--swatch-*`** — APPEARANCE names (`--swatch-ink`, `--swatch-fog`). These
  hold the **literals**, and each hex is written **exactly once**.
- **role tokens** — what a colour is FOR (`--background`, `--muted-foreground`,
  `--border`, `--brand`). Each one **aliases a swatch**; none holds a literal.

Why: a role can be repointed at a different swatch without the swatch name
becoming a lie, and two roles that happen to share a colour stay in sync
because they alias the same swatch instead of repeating its hex.

`tailwind.css` is the v4 bridge — `@theme inline` re-exports each role as
`--color-*` so `bg-background` / `text-muted-foreground` / `border-border`
compile to `var(--role)`. `inline` is required: without it Tailwind snapshots
the value at build time and a future `[data-theme]` override could never reach
the utility.

## ❌ Don't

- ❌ **No colour literal outside that folder** — no `#hex`, no `rgb()`,
  `hsl()`, `oklch()` with raw numbers, in `.ts`, `.tsx`, `.css` or `.html`.
- ❌ **No raw Tailwind palette utility for a design decision** —
  `bg-neutral-950`, `text-zinc-400`. They are literals with friendlier names,
  and they bypass the palette. Name a role: `bg-background`.
- ❌ **No hex written twice in `tokens.css`.** If two tokens want one colour,
  it earns a `--swatch-*` name and both alias it.
- ❌ **No component naming a swatch.** Components name roles only.

## ✅ Do

| Need                  | Reach for                                            |
| --------------------- | ---------------------------------------------------- |
| a colour in CSS       | `var(--muted-foreground)`                            |
| a colour in markup    | `bg-background`, `text-foreground`, `border-border`  |
| a translucent variant | `color-mix(in oklab, var(--brand) 30%, transparent)` |
| a new colour          | add a `--swatch-*`, then a role that aliases it      |

A colour function that wraps a token (`color-mix(in oklab, var(--x) …)`)
derives from the palette rather than restating it — that is the fix, not a
violation, and the checker allows it.

## The Go window background

`main.go` paints the native window with `application.NewRGB(...)` **before the
webview exists**, so it is a second copy of `--background` in another language.
When the two disagree the app flashes the wrong colour for a frame on every
launch — visible to users and invisible to every other check.

**Change one, change the other.** The checker compares them and fails if they
drift.

## Adding an exception

There is currently none, and that is the point. If a value genuinely cannot
reach a token, add the **file and a reason** to `ALLOWED` in
`scripts/check-design-tokens.mjs`. It is a `Map`, not a regex, so an entry
cannot rot silently — and adding one should feel like a decision.
