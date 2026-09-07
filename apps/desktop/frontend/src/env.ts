import { createEnv } from "@t3-oss/env-core";
import { z } from "zod";

/**
 * Validated build-time configuration.
 *
 * A desktop bundle ships to the user's machine, so anything here is readable by
 * them — secrets belong in Go, behind a service method.
 */
export const env = createEnv({
  clientPrefix: "VITE_",
  client: {
    VITE_APP_TITLE: z.string().min(1).default("Wails + React"),
  },
  runtimeEnv: import.meta.env,
  emptyStringAsUndefined: true,
});
