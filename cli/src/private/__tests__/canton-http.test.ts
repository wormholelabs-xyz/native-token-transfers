import { afterAll, describe, expect, test } from "bun:test";

import { fetchJsonBounded } from "../canton/http.js";
import { CantonLedgerClient } from "../canton/ledger-client.js";
import { parseCantonPartyId } from "../types.js";

let tokenRequestCount = 0;
let ledgerRequestCount = 0;
let ledgerStatuses: number[] = [];

const server = Bun.serve({
  port: 0,
  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname === "/hang") {
      return new Promise<Response>((resolve) =>
        setTimeout(() => resolve(new Response("late")), 1000)
      );
    }
    if (url.pathname === "/streamed-big") {
      const stream = new ReadableStream({
        start(controller) {
          controller.enqueue(new TextEncoder().encode("x".repeat(1024)));
          controller.enqueue(new TextEncoder().encode("x".repeat(1024)));
          controller.close();
        },
      });
      return new Response(stream);
    }
    if (url.pathname === "/declared-big") return new Response("x".repeat(2048));
    if (url.pathname === "/ok") return Response.json({ answer: 42 });
    if (url.pathname === "/token") {
      tokenRequestCount++;
      return Response.json({
        access_token: `token-${tokenRequestCount}`,
        expires_in: 300,
      });
    }
    if (url.pathname === "/v2/state/ledger-end") {
      ledgerRequestCount++;
      const status = ledgerStatuses.shift() ?? 200;
      return status === 200
        ? Response.json({ offset: 7 })
        : new Response("no", { status });
    }
    return new Response("not found", { status: 404 });
  },
});
const base = `http://localhost:${server.port}`;
afterAll(() => server.stop(true));

describe("fetchJsonBounded", () => {
  test("parses a small JSON body", async () => {
    const response = await fetchJsonBounded(
      `${base}/ok`,
      { method: "GET" },
      { timeoutMs: 2000, maxBytes: 1024 }
    );
    expect(response.status).toBe(200);
    expect(response.json).toEqual({ answer: 42 });
  });

  test("a server that never answers rejects within the timeout", async () => {
    const started = Date.now();
    await expect(
      fetchJsonBounded(
        `${base}/hang`,
        { method: "GET" },
        { timeoutMs: 200, maxBytes: 1024 }
      )
    ).rejects.toThrow(/timed out/);
    expect(Date.now() - started).toBeLessThan(2000);
  });

  test("a body of maxBytes + 1 rejects", async () => {
    await expect(
      fetchJsonBounded(
        `${base}/streamed-big`,
        { method: "GET" },
        { timeoutMs: 2000, maxBytes: 2047 }
      )
    ).rejects.toThrow(/larger than/);
  });

  test("a Content-Length above the cap rejects", async () => {
    await expect(
      fetchJsonBounded(
        `${base}/declared-big`,
        { method: "GET" },
        { timeoutMs: 2000, maxBytes: 1024 }
      )
    ).rejects.toThrow(/Content-Length/);
  });
});

function ledgerClient(): CantonLedgerClient {
  return new CantonLedgerClient({
    jsonApiUrl: new URL(base),
    oauthTokenUrl: new URL(`${base}/token`),
    oauthClientId: "client",
    oauthClientSecret: "secret",
    userPartyId: parseCantonPartyId(`alice::1220${"ab".repeat(32)}`),
  });
}

describe("CantonLedgerClient 401 handling", () => {
  test("401 then 200: 2 requests, 1 token refresh", async () => {
    tokenRequestCount = 0;
    ledgerRequestCount = 0;
    ledgerStatuses = [401, 200];
    expect(await ledgerClient().ledgerEndOffset()).toBe(7);
    expect(ledgerRequestCount).toBe(2);
    expect(tokenRequestCount).toBe(2);
  });

  test("401 twice: 2 requests, then an error", async () => {
    tokenRequestCount = 0;
    ledgerRequestCount = 0;
    ledgerStatuses = [401, 401];
    await expect(ledgerClient().ledgerEndOffset()).rejects.toThrow(/401/);
    expect(ledgerRequestCount).toBe(2);
  });
});
