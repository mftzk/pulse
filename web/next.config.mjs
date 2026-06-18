import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const nextConfig = {
  // self-contained server bundle for a small Docker runtime image
  output: "standalone",
  // pin the tracing root to THIS app so standalone output is flat
  outputFileTracingRoot: __dirname,
  // NOTE: the /api/* reverse-proxy lives in app/api/[...path]/route.ts (runtime),
  // not a rewrite here — rewrite destinations are baked at build time and would
  // ignore the runtime API_URL env var.
};

export default nextConfig;
