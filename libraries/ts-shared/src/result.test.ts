import { describe, expect, it } from "vitest";

import { attempt, err, ok } from "./result.ts";

describe("Result", () => {
  it("wraps a value as ok", () => {
    expect(ok(1)).toStrictEqual({ ok: true, value: 1 });
  });

  it("wraps an error as err", () => {
    const error = new Error("nope");
    expect(err(error)).toStrictEqual({ ok: false, error });
  });
});

describe("attempt", () => {
  it("captures a resolved value", async () => {
    await expect(attempt(() => Promise.resolve("hi"))).resolves.toStrictEqual({
      ok: true,
      value: "hi",
    });
  });

  it("captures a rejection as an Error", async () => {
    const result = await attempt(() => Promise.reject(new Error("boom")));

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.message).toBe("boom");
    }
  });

  // Go errors surface as strings often enough that a non-Error rejection must
  // not produce `error.message === undefined` downstream.
  it("normalises a non-Error rejection", async () => {
    const result = await attempt(() => Promise.reject("plain string"));

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toBeInstanceOf(Error);
      expect(result.error.message).toBe("plain string");
    }
  });
});
