import { describe, expect, test, vi } from "vitest";
import type { Credential } from "../src/auth/index.js";
import { Client } from "../src/client.js";
import { AuthError, NotFoundError, ServerError } from "../src/errors.js";

function makeClient(fetchImpl: typeof fetch, credential?: Credential): Client {
  return new Client({
    baseUrl: "http://m",
    credential: credential ?? { token: "test-key", scheme: "X-API-Key", source: "test" },
    fetchImpl,
  });
}

describe("resource modules", () => {
  test("search sends query and filters", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toContain("/api/v1/search");
      expect(url.toString()).toContain("query=orders");
      expect(url.toString()).toContain("asset_types=table");
      expect(url.toString()).toContain("limit=10");
      const headers = init?.headers as Record<string, string>;
      expect(headers["X-API-Key"]).toBe("test-key");
      return new Response(JSON.stringify({ results: [{ id: "a1" }] }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = (await client.search("orders", { types: ["table"], limit: 10 })) as {
      results: Array<{ id: string }>;
    };
    expect(out.results[0]?.id).toBe("a1");
  });

  test("assets.get", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/abc");
      return new Response(JSON.stringify({ id: "abc", name: "orders" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const a = (await client.assets.get("abc")) as { name: string };
    expect(a.name).toBe("orders");
  });

  test("assets.find returns null on 404", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error: "nf" }), { status: 404 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.assets.find({ type: "T", service: "s", name: "n" });
    expect(out).toBeNull();
  });

  test("assets.lookup raises on 404", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error: "nf" }), { status: 404 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(client.assets.lookup({ type: "T", service: "s", name: "n" })).rejects.toThrow(
      NotFoundError,
    );
  });

  test("lineage.write defaults type", async () => {
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      const body = JSON.parse(init?.body as string);
      expect(body).toEqual({ source: "a", target: "b", type: "DIRECT" });
      return new Response(JSON.stringify({ id: "edge-1" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.lineage.write({ source: "a", target: "b" });
  });

  test("lineage.batch normalizes tuples", async () => {
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      const body = JSON.parse(init?.body as string);
      expect(body).toEqual([
        { source: "a", target: "b", type: "DIRECT" },
        { source: "c", target: "d", type: "writes" },
      ]);
      return new Response(JSON.stringify([]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.lineage.batch([
      ["a", "b"],
      ["c", "d", "writes"],
    ]);
  });

  test("401 throws AuthError when no refresh", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error_description: "expired" }), { status: 401 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(client.assets.get("x")).rejects.toThrow(/expired/);
  });

  test("5xx throws ServerError with status", async () => {
    const fetchImpl = vi.fn(async () => new Response("oops", { status: 503 }));
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    try {
      await client.assets.get("x");
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBeInstanceOf(ServerError);
      expect((e as ServerError).statusCode).toBe(503);
    }
  });

  test("Bearer credential uses Authorization header", async () => {
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      const headers = init?.headers as Record<string, string>;
      expect(headers.Authorization).toBe("Bearer jwt");
      return new Response(JSON.stringify({ results: [] }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch, {
      token: "jwt",
      scheme: "Bearer",
      source: "test",
    });
    await client.search("x");
  });

  test("refresh on 401 retries with new token", async () => {
    let calls = 0;
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      calls++;
      const auth = (init?.headers as Record<string, string>).Authorization;
      if (calls === 1) {
        expect(auth).toBe("Bearer old-jwt");
        return new Response("", { status: 401 });
      }
      expect(auth).toBe("Bearer new-jwt");
      return new Response(JSON.stringify({ results: [] }), { status: 200 });
    });
    const refresh = vi.fn(
      async (): Promise<Credential> => ({
        token: "new-jwt",
        scheme: "Bearer",
        refresh,
        source: "r",
      }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch, {
      token: "old-jwt",
      scheme: "Bearer",
      refresh,
      source: "r",
    });
    await client.search("x");
    expect(refresh).toHaveBeenCalledOnce();
  });

  test("AuthError vs ServerError type guards", () => {
    expect(new AuthError("x")).toBeInstanceOf(AuthError);
    expect(new ServerError("x", 500).statusCode).toBe(500);
  });
});
