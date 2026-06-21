import { describe, expect, test, vi } from "vitest";
import type { Credential } from "../src/auth/index.js";
import { Client } from "../src/client.js";
import {
  AuthError,
  NotFoundError,
  RateLimitError,
  ServerError,
  ValidationError,
  isNotFound,
  isRateLimit,
} from "../src/errors.js";
import type {
  Asset,
  DataProduct,
  DataProductListResult,
  GlossaryTerm,
  Tag,
  User,
} from "../src/index.js";

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

  test("assets.get returns a typed Asset", async () => {
    const fetchImpl = vi.fn(
      async () =>
        new Response(JSON.stringify({ id: "abc", name: "orders", type: "Table" }), {
          status: 200,
        }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const asset: Asset = await client.assets.get("abc");
    expect(asset.name).toBe("orders");
    expect(asset.type).toBe("Table");
  });

  test("assets.search forwards filters as query params", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/assets/search");
      expect(u).toContain("q=orders");
      expect(u).toContain("types=table");
      expect(u).toContain("services=postgres");
      expect(u).toContain("tags=pii");
      return new Response(JSON.stringify({ assets: [], total: 0 }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.assets.search({
      query: "orders",
      types: ["table"],
      providers: ["postgres"],
      tags: ["pii"],
    });
  });

  test("assets.summary returns AssetSummaryResponse", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("/api/v1/assets/summary");
      return new Response(JSON.stringify({ providers: { postgres: 5 } }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.assets.summary();
    expect(out.providers?.postgres).toBe(5);
  });

  test("assets.addTag POSTs to /assets/tags/{id}", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/tags/asset-1");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 201 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags = await client.assets.addTag("asset-1", "tag-1");
    expect(tags[0]?.name).toBe("pii");
  });

  test("assets.removeTag DELETEs with a body", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/tags/asset-1");
      expect(init?.method).toBe("DELETE");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(null, { status: 204 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.assets.removeTag("asset-1", "tag-1");
  });

  test("lineage.get attaches direction and depth", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/lineage/assets/abc");
      expect(u).toContain("direction=upstream");
      expect(u).toContain("depth=3");
      return new Response(JSON.stringify({ edges: [], nodes: [] }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.lineage.upstream("abc", { depth: 3 });
  });

  test("lineage.write attaches jobMrn when provided", async () => {
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      expect(JSON.parse(init?.body as string)).toEqual({
        source: "a",
        target: "b",
        type: "DIRECT",
        job_mrn: "job/123",
      });
      return new Response(JSON.stringify({ id: "e1" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.lineage.write({ source: "a", target: "b", jobMrn: "job/123" });
  });

  test("glossary.create posts typed body", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/glossary/");
      expect(JSON.parse(init?.body as string)).toEqual({
        name: "PII",
        definition: "Personally Identifiable Information",
        description: "Sensitive data",
      });
      return new Response(JSON.stringify({ id: "term-1", name: "PII" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const term: GlossaryTerm = await client.glossary.create({
      name: "PII",
      definition: "Personally Identifiable Information",
      description: "Sensitive data",
    });
    expect(term.id).toBe("term-1");
  });

  test("glossary.list passes pagination", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/glossary/list");
      expect(u).toContain("limit=50");
      expect(u).toContain("offset=10");
      return new Response(JSON.stringify({ terms: [], total: 0 }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.glossary.list({ limit: 50, offset: 10 });
  });

  test("users.me hits /users/me", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/users/me");
      return new Response(JSON.stringify({ id: "u1", username: "alice" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const me: User = await client.users.me();
    expect(me.username).toBe("alice");
  });

  test("teams.members hits /teams/{id}/members", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/teams/team-1/members");
      return new Response(JSON.stringify({ members: [{ id: "m1" }] }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.teams.members("team-1");
    expect(out.members?.[0]?.id).toBe("m1");
  });

  test("apiKeys.create posts CreateAPIKeyRequest", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/users/apikeys");
      expect(JSON.parse(init?.body as string)).toEqual({ name: "ci", expires_in_days: 30 });
      return new Response(JSON.stringify({ id: "k1", key: "raw-secret" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.apiKeys.create({ name: "ci", expiresInDays: 30 });
    expect(out.key).toBe("raw-secret");
  });

  test("metrics.topAssets passes start/end", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/metrics/top-assets");
      expect(u).toContain("start=2025-01-01");
      expect(u).toContain("end=2025-02-01");
      expect(u).toContain("limit=5");
      return new Response(JSON.stringify([{ asset_id: "a1", count: 10 }]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.metrics.topAssets({
      start: "2025-01-01T00:00:00Z",
      end: "2025-02-01T00:00:00Z",
      limit: 5,
    });
    expect(out[0]?.count).toBe(10);
  });

  test("owners.search hits /owners/search", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/owners/search");
      expect(u).toContain("q=alice");
      return new Response(JSON.stringify({ owners: [] }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.owners.search("alice");
  });

  test("runs.entities passes filters", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      const u = url.toString();
      expect(u).toContain("/api/v1/runs/run-1/entities");
      expect(u).toContain("entity_type=asset");
      expect(u).toContain("status=failed");
      return new Response(JSON.stringify({ entities: [], total: 0 }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.runs.entities("run-1", { entityType: "asset", status: "failed" });
  });

  test("admin.reindex POSTs to /admin/search/reindex", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/admin/search/reindex");
      expect(init?.method).toBe("POST");
      return new Response(JSON.stringify({ status: "accepted" }), { status: 202 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const out = await client.admin.reindex();
    expect(out.status).toBe("accepted");
  });

  test("agentRuns.create normalizes tool calls and timestamps", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/agents/runs");
      const body = JSON.parse(init?.body as string);
      expect(body).toMatchObject({
        agent_mrn: "marmot://lc/agent/explorer",
        run_id: "r-1",
        status: "completed",
        started_at: "2025-01-02T03:04:05.000Z",
        tokens_in: 100,
        tokens_out: 200,
      });
      expect(body.tool_calls).toEqual([
        {
          tool_name: "search",
          started_at: "2025-01-02T03:04:05.000Z",
          status: "success",
          target_mrn: "postgres://orders",
          duration_ms: 12,
        },
      ]);
      return new Response(JSON.stringify({ id: "agent-run-1" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const t = new Date("2025-01-02T03:04:05Z");
    await client.agentRuns.create({
      agentMrn: "marmot://lc/agent/explorer",
      runId: "r-1",
      startedAt: t,
      status: "completed",
      tokensIn: 100,
      tokensOut: 200,
      toolCalls: [
        { toolName: "search", startedAt: t, targetMrn: "postgres://orders", durationMs: 12 },
      ],
    });
  });

  test("400 throws ValidationError", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error: "bad" }), { status: 400 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(client.assets.get("x")).rejects.toBeInstanceOf(ValidationError);
  });

  test("429 throws RateLimitError and isRateLimit returns true", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error: "slow down" }), { status: 429 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    try {
      await client.assets.get("x");
      throw new Error("expected throw");
    } catch (e) {
      expect(isRateLimit(e)).toBe(true);
      expect(e).toBeInstanceOf(RateLimitError);
    }
  });

  test("isNotFound type guard", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ error: "nope" }), { status: 404 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    try {
      await client.assets.get("x");
      throw new Error("expected throw");
    } catch (e) {
      expect(isNotFound(e)).toBe(true);
    }
  });

  test("tags.list returns Tag array", async () => {
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 200 }),
    );
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags: Tag[] = await client.tags.list();
    expect(tags[0]?.id).toBe("tag-1");
    expect(tags[0]?.name).toBe("pii");
  });

  test("tags.get returns a Tag", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("/api/v1/tags/tag-1");
      return new Response(JSON.stringify({ id: "tag-1", name: "pii" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tag: Tag = await client.tags.get("tag-1");
    expect(tag.id).toBe("tag-1");
    expect(tag.name).toBe("pii");
  });

  test("tags.create POSTs with name and description", async () => {
    const fetchImpl = vi.fn(async (_url: URL | string, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({
        name: "pii",
        description: "sensitive data",
      });
      return new Response(JSON.stringify({ id: "tag-1", name: "pii" }), { status: 201 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tag = await client.tags.create({ name: "pii", description: "sensitive data" });
    expect(tag.id).toBe("tag-1");
  });

  test("tags.update PUTs updated fields", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toContain("/api/v1/tags/tag-1");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({ name: "pii-updated" });
      return new Response(JSON.stringify({ id: "tag-1", name: "pii-updated" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tag = await client.tags.update("tag-1", { name: "pii-updated" });
    expect(tag.name).toBe("pii-updated");
  });

  test("tags.delete sends DELETE", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toContain("/api/v1/tags/tag-1");
      expect(init?.method).toBe("DELETE");
      return new Response(null, { status: 204 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.tags.delete("tag-1");
  });

  test("assets.listTags GETs tag array", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/tags/asset-1");
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags: Tag[] = await client.assets.listTags("asset-1");
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("assets.setTags PUTs tag_ids", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/tags/asset-1");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_ids: ["tag-1"] });
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags = await client.assets.setTags("asset-1", ["tag-1"]);
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("assets.setColumnTags PUTs column path and tag_ids", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/column-tags/asset-1");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({
        column_path: "schema.table.col",
        tag_ids: ["tag-1"],
      });
      return new Response(null, { status: 204 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.assets.setColumnTags("asset-1", "schema.table.col", ["tag-1"]);
  });

  test("assets.removeColumnTag DELETEs with column path and tag_id", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/assets/column-tags/asset-1");
      expect(init?.method).toBe("DELETE");
      expect(JSON.parse(init?.body as string)).toEqual({
        column_path: "schema.table.col",
        tag_id: "tag-1",
      });
      return new Response(null, { status: 204 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.assets.removeColumnTag("asset-1", "schema.table.col", "tag-1");
  });

  test("glossary.listTermTags returns Tag array", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/glossary/tags/term-1");
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags: Tag[] = await client.glossary.listTermTags("term-1");
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("glossary.addTermTag POSTs tag_id", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/glossary/tags/term-1");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 201 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags = await client.glossary.addTermTag("term-1", "tag-1");
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("glossary.removeTermTag DELETEs with tag_id", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/glossary/tags/term-1");
      expect(init?.method).toBe("DELETE");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(JSON.stringify({ message: "tag removed" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.glossary.removeTermTag("term-1", "tag-1");
  });

  test("glossary.setTermTags PUTs tag_ids and returns term", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/glossary/tags/term-1");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_ids: ["tag-1"] });
      return new Response(JSON.stringify({ id: "term-1", name: "PII" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const result = await client.glossary.setTermTags("term-1", ["tag-1"]);
    expect(result.id).toBe("term-1");
  });

  test("dataProducts.list returns DataProductListResult", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("/api/v1/products/list");
      return new Response(
        JSON.stringify({ data_products: [{ id: "product-1", name: "Orders" }], total: 1 }),
        { status: 200 },
      );
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const result: DataProductListResult = await client.dataProducts.list();
    expect(result.total).toBe(1);
    expect(result.data_products?.[0]?.id).toBe("product-1");
  });

  test("dataProducts.list passes pagination params", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("limit=10");
      expect(url.toString()).toContain("offset=20");
      return new Response(JSON.stringify({ data_products: [], total: 0 }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.dataProducts.list({ limit: 10, offset: 20 });
  });

  test("dataProducts.get returns DataProduct", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("/api/v1/products/product-1");
      return new Response(JSON.stringify({ id: "product-1", name: "Orders" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const product: DataProduct = await client.dataProducts.get("product-1");
    expect(product.id).toBe("product-1");
    expect(product.name).toBe("Orders");
  });

  test("dataProducts.listTags returns Tag array", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toBe("http://m/api/v1/products/tags/product-1");
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags: Tag[] = await client.dataProducts.listTags("product-1");
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("dataProducts.addTag POSTs tag_id", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/products/tags/product-1");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(JSON.stringify([{ id: "tag-1", name: "pii" }]), { status: 201 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tags = await client.dataProducts.addTag("product-1", "tag-1");
    expect(tags[0]?.id).toBe("tag-1");
  });

  test("dataProducts.removeTag DELETEs with tag_id", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/products/tags/product-1");
      expect(init?.method).toBe("DELETE");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_id: "tag-1" });
      return new Response(JSON.stringify({ message: "tag removed" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    await client.dataProducts.removeTag("product-1", "tag-1");
  });

  test("dataProducts.setTags PUTs tag_ids and returns DataProduct", async () => {
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      expect(url.toString()).toBe("http://m/api/v1/products/tags/product-1");
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(init?.body as string)).toEqual({ tag_ids: ["tag-1"] });
      return new Response(JSON.stringify({ id: "product-1", name: "Orders" }), { status: 200 });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const product = await client.dataProducts.setTags("product-1", ["tag-1"]);
    expect(product.id).toBe("product-1");
  });
});
