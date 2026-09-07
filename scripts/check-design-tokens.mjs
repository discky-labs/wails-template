#!/usr/bin/env node
// Guards the one-source-of-truth rule: every colour in this repo is declared in
// the frontend's styles folder and reached through a CSS custom property or the
// Tailwind utility the bridge generates from it. A colour literal anywhere else
// is drift — it silently stops tracking the palette the moment the palette
// moves.
//
//   moon run root:check-design-tokens
//
// Three checks, each of which exists because the failure it catches is SILENT:
//   1. no colour literal outside the styles folder,
//   2. no hex written twice inside tokens.css (a palette drifting against
//      itself — change one and the other stays behind),
//   3. main.go's native window background equals `--background` (a mismatch
//      flashes the wrong colour for one frame on every launch).
//
// Exceptions are ALLOWLISTED BY FILE WITH A REASON below, so the escape hatch
// stays visible and small. Adding to it should feel like a decision.
//
// Adapted from my-hub/scripts/check-design-tokens.mjs.

import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(fileURLToPath(new URL("..", import.meta.url)));

/** The folder that is ALLOWED to hold literals — it is the source of truth. */
const SOURCE_OF_TRUTH = "apps/desktop/frontend/src/styles/";

const TOKENS_CSS = `${SOURCE_OF_TRUTH}tokens.css`;
const MAIN_GO = "apps/desktop/main.go";

/**
 * Files that may keep a literal, each with the reason. A Map, not a regex, so
 * an entry cannot rot silently: delete the file and this stops matching.
 */
const ALLOWED = new Map();

/** Colour literals: #rgb/#rgba/#rrggbb/#rrggbbaa, and the CSS colour functions. */
const LITERAL =
  /#[0-9a-fA-F]{3,8}\b|\b(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color-mix)\s*\(/;

/** Strip comments so a documented hex is not a violation. */
function stripComments(source, isCss) {
  let out = source.replaceAll(/\/\*[\s\S]*?\*\//g, "");
  if (!isCss) out = out.replaceAll(/(^|[^:])\/\/.*$/gm, "$1");

  return out;
}

// Every tracked file, filtered in JS. Deliberately NOT a git pathspec: a
// wildcard pathspec silently matches almost nothing, which makes the check pass
// vacuously — the worst possible failure mode for a guard.
function listFiles() {
  return execFileSync("git", ["ls-files", "-z"], {
    cwd: ROOT,
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
  })
    .split("\0")
    .filter(Boolean);
}

/** Where a colour could plausibly be written. Keeps docs and configs out. */
const SCANNED = /^(?:apps\/[^/]+\/frontend|libraries\/[^/]+)\/src\//;

// Generated output and test fixtures. A hex in a test is sample DATA standing in
// for a value, not a design decision — nothing renders from it.
const IGNORED = /\.gen\.ts$|^apps\/[^/]+\/frontend\/bindings\/|\.test\.tsx?$/;

const violations = [];
for (const file of listFiles()) {
  if (!SCANNED.test(file) || file.startsWith(SOURCE_OF_TRUTH)) continue;
  if (IGNORED.test(file) || ALLOWED.has(file)) continue;
  if (!/\.(css|ts|tsx|html)$/.test(file)) continue;

  const isCss = file.endsWith(".css");
  const lines = stripComments(readFileSync(resolve(ROOT, file), "utf8"), isCss).split("\n");
  for (const [index, line] of lines.entries()) {
    // A colour function wrapping a token — `color-mix(in oklab, var(--x) …)`,
    // `oklch(from var(--y) …)` — derives from the palette rather than
    // restating a colour. That is the fix, not the problem.
    if (!LITERAL.test(line)) continue;
    if (/\b(?:rgb|rgba|hsl|hsla|oklch|oklab|color-mix)\s*\(\s*(?:(?:in|from)\s+[\w-]+\s*,?\s*)?var\(/.test(line))
      continue;
    violations.push({ file, line: index + 1, text: line.trim().slice(0, 110) });
  }
}

// Comments are stripped BEFORE block extraction: tokens.css documents how to
// add a `[data-theme="light"] { … }` block, and a bare indexOf would find that
// prose and slice a phantom block out of the middle of :root — reporting every
// finding twice.
const tokensCss = stripComments(readFileSync(resolve(ROOT, TOKENS_CSS), "utf8"), true);

/** A `{ … }` block of tokens.css by its opening selector. */
function ruleBlock(selector) {
  const from = tokensCss.indexOf(selector);

  return from < 0 ? "" : tokensCss.slice(from, tokensCss.indexOf("\n}", from));
}

// ---------------------------------------------------------------------------
// Check 2: inside tokens.css a hex is written EXACTLY ONCE. If two tokens want
// the same colour, it earns a `--swatch-*` appearance name and both alias it.
// Two literals of one colour is how a palette drifts against itself.
// ---------------------------------------------------------------------------

const repeated = [];
for (const selector of [":root {", '[data-theme="light"] {', '[data-theme="dark"] {']) {
  const seen = new Map();
  for (const [, name, hex] of ruleBlock(selector).matchAll(
    /^\s*--([a-z0-9-]+):\s*(#[0-9a-fA-F]{3,8})\s*;/gm,
  )) {
    const key = hex.toLowerCase();
    seen.set(key, [...(seen.get(key) ?? []), name]);
  }

  for (const [hex, names] of seen) {
    if (names.length > 1) {
      repeated.push(
        `${selector.trim()}  ${hex} written ${String(names.length)}×: ${names
          .map((n) => `--${n}`)
          .join(", ")}`,
      );
    }
  }
}

// ---------------------------------------------------------------------------
// Check 3: the NATIVE window background is painted by Go before the webview
// exists, so it is a second copy of `--background` in another language. When
// they disagree the app flashes the wrong colour on every launch — visible,
// annoying, and invisible to every other check in the repo.
// ---------------------------------------------------------------------------

/** Follows `var(--x)` aliases to the literal behind a role token. */
function tokenValue(block, name, hops = 0) {
  const raw = new RegExp(`^\\s*--${name}:\\s*([^;]+);`, "m").exec(block)?.[1]?.trim();
  const alias = raw === undefined ? null : /^var\(\s*--([a-z0-9-]+)\s*\)$/.exec(raw);
  if (alias !== null && hops < 4) return tokenValue(block, alias[1], hops + 1);

  return raw;
}

function hexToRgb(hex) {
  const short = hex.length === 4;
  const part = (i) =>
    short
      ? Number.parseInt(hex[1 + i].repeat(2), 16)
      : Number.parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16);

  return [part(0), part(1), part(2)];
}

const windowMismatch = [];
const background = tokenValue(ruleBlock(":root {"), "background");
const mainGo = readFileSync(resolve(ROOT, MAIN_GO), "utf8");
const rgbCall = /application\.NewRGB\(\s*(\d+),\s*(\d+),\s*(\d+)\s*\)/.exec(mainGo);

if (background === undefined) {
  windowMismatch.push(`${TOKENS_CSS} declares no --background token`);
} else if (rgbCall === null) {
  windowMismatch.push(`${MAIN_GO} has no application.NewRGB(...) call to compare`);
} else if (!/^#[0-9a-fA-F]{3,8}$/.test(background)) {
  windowMismatch.push(`--background resolves to ${background}, which is not a hex`);
} else {
  const want = hexToRgb(background);
  const got = rgbCall.slice(1, 4).map(Number);
  if (want.join(",") !== got.join(",")) {
    windowMismatch.push(
      `${MAIN_GO} paints NewRGB(${got.join(", ")}) but --background is ` +
        `${background} = NewRGB(${want.join(", ")})`,
    );
  }
}

// ---------------------------------------------------------------------------

let failed = false;

if (repeated.length > 0) {
  failed = true;
  console.error(`\n${TOKENS_CSS} writes the same hex more than once:\n`);
  for (const line of repeated) console.error(`  ${line}`);
  console.error(
    "\nGive the colour a --swatch-* appearance name and have both tokens" +
      "\nalias it, so there is one place to change it.\n",
  );
}

if (windowMismatch.length > 0) {
  failed = true;
  console.error("\nThe native window background has drifted from the palette:\n");
  for (const line of windowMismatch) console.error(`  ${line}`);
  console.error(
    "\nThe Go window is painted before the webview loads, so it must equal" +
      "\n--background or the app flashes the wrong colour on launch.\n",
  );
}

if (violations.length > 0) {
  failed = true;
  console.error(`\n${String(violations.length)} colour literal(s) outside ${SOURCE_OF_TRUTH}:\n`);
  for (const v of violations) console.error(`  ${v.file}:${String(v.line)}\n      ${v.text}`);
  console.error(
    `\nEvery colour lives in ${SOURCE_OF_TRUTH}. Reach for it as:` +
      "\n  CSS       var(--muted-foreground)" +
      "\n  Tailwind  bg-background, text-muted-foreground, border-border" +
      "\n  derived   color-mix(in oklab, var(--brand) 30%, transparent)" +
      "\nIf a value genuinely cannot be reached, add the FILE and a reason to" +
      "\nALLOWED in scripts/check-design-tokens.mjs.\n",
  );
}

if (failed) process.exit(1);

console.log(
  `Colour literals confined to ${SOURCE_OF_TRUTH} ` +
    `(${String(ALLOWED.size)} allowlisted file(s)); ` +
    `no repeated hex; window background matches --background.`,
);
