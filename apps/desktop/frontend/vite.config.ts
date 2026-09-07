import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
  server: {
    // Wails' dev mode requires the frontend server on WAILS_VITE_PORT with a
    // strict port: the Go side is told exactly this URL via
    // FRONTEND_DEVSERVER_URL and does not probe for alternatives.
    host: "127.0.0.1",
    port: Number(process.env["WAILS_VITE_PORT"]) || 9245,
    strictPort: true,
  },
  plugins: [
    // MUST come before the react plugin — TanStack's plugin rewrites route
    // files that the react plugin then transforms.
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    // Generates typed custom-event definitions from the Go bindings. The path
    // is the bindings dir emitted by `wails3 generate bindings`.
    wails("./bindings"),
  ],
  test: {
    environment: "jsdom",
    // Explicit imports of describe/it/expect rather than ambient globals.
    globals: false,
    // Registers the testing-library cleanup that `globals: false` prevents
    // @testing-library/react from installing itself.
    setupFiles: ["./src/test-setup.ts"],
  },
});
