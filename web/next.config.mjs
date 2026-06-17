import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const API_URL = process.env.API_URL || "http://localhost:8080";

const nextConfig = {
  // produce a self-contained server bundle for a small Docker runtime image
  output: "standalone",
  // pin the tracing root to THIS app so the standalone output is flat
  // (.next/standalone/server.js) instead of nested under the detected monorepo root
  outputFileTracingRoot: __dirname,
  // Proxy all /api calls to the Go API so the browser stays same-origin and the
  // httpOnly session cookie just works (no CORS, no websockets).
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_URL}/api/:path*` }];
  },
};

export default nextConfig;
