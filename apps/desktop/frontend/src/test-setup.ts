import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// `globals: false` (see vite.config.ts) keeps vitest's APIs explicit imports,
// which also means @testing-library/react cannot register its own afterEach
// cleanup hook. Without this, every rendered tree stays in the document and the
// next test sees duplicated elements: "Found multiple elements with the role".
afterEach(cleanup);
