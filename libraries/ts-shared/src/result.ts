/**
 * A Result carries either a value or an error, without throwing.
 *
 * Wails binds Go methods as promises that reject with the Go error's message,
 * so every service call is a place where an error has to be handled rather than
 * assumed away. `attempt` turns such a call into a value the UI can branch on.
 */
export type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

export function ok<T>(value: T): Result<T, never> {
  return { ok: true, value };
}

export function err<E>(error: E): Result<never, E> {
  return { ok: false, error };
}

/** Runs a promise-returning function and captures a rejection as an error. */
export async function attempt<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    return ok(await fn());
  } catch (cause) {
    return err(cause instanceof Error ? cause : new Error(String(cause)));
  }
}
