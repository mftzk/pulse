import { NextRequest } from "next/server";

// Runtime reverse-proxy for the Go API. We do this in a route handler (not a
// next.config rewrite) because rewrite destinations are baked at BUILD time,
// whereas a handler reads process.env.API_URL on every request — so the same
// image works in any environment (local compose, cluster) by setting API_URL.
export const dynamic = "force-dynamic";

const HOP_BY_HOP = ["transfer-encoding", "connection", "content-encoding", "content-length"];

async function proxy(req: NextRequest) {
  const apiUrl = process.env.API_URL || "http://localhost:8080";
  const incoming = new URL(req.url);
  const target = `${apiUrl}${incoming.pathname}${incoming.search}`;

  const headers = new Headers(req.headers);
  headers.delete("host");
  headers.delete("connection");

  const init: RequestInit = { method: req.method, headers, redirect: "manual" };
  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }

  let upstream: Response;
  try {
    upstream = await fetch(target, init);
  } catch {
    return new Response(JSON.stringify({ error: "upstream unreachable" }), {
      status: 502,
      headers: { "content-type": "application/json" },
    });
  }

  const respHeaders = new Headers();
  upstream.headers.forEach((value, key) => {
    if (!HOP_BY_HOP.includes(key.toLowerCase())) respHeaders.set(key, value);
  });
  // forward Set-Cookie (undici folds them; getSetCookie returns the array)
  const setCookies =
    (upstream.headers as unknown as { getSetCookie?: () => string[] }).getSetCookie?.() ?? [];
  if (setCookies.length) {
    respHeaders.delete("set-cookie");
    for (const c of setCookies) respHeaders.append("set-cookie", c);
  }

  const body = await upstream.arrayBuffer();
  return new Response(body, { status: upstream.status, headers: respHeaders });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const HEAD = proxy;
export const OPTIONS = proxy;
