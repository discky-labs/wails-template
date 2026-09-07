import path from "node:path";
import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

const here = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  test: {
    coverage: {
      provider: "v8",
      include: ["src/**/*.ts"],
      exclude: ["**/*.test.ts"],
      reporter: ["text", "html", "lcov"],
      // Anchored deliberately: invoked from the repo root, vitest would
      // otherwise dump coverage at the workspace root.
      reportsDirectory: path.resolve(here, "coverage"),
      thresholds: { lines: 80, branches: 80, functions: 80, statements: 80 },
    },
  },
});
