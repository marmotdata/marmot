import { describe, expect, test, vi } from "vitest";
import type { Credential } from "../src/auth/index.js";
import { Client } from "../src/client.js";
import {
  MarmotCallbackHandler,
  catalogTools,
  marmotTool,
} from "../src/integrations/langchain/index.js";

function makeClient(fetchImpl: typeof fetch, credential?: Credential): Client {
  return new Client({
    baseUrl: "http://m",
    credential: credential ?? { token: "test-key", scheme: "X-API-Key", source: "test" },
    fetchImpl,
  });
}

function ok(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("langchain integration", () => {
  test("catalogTools exposes the expected tool names", () => {
    const client = makeClient(vi.fn() as unknown as typeof fetch);
    const tools = catalogTools(client);
    expect(tools.map((t) => t.name).sort()).toEqual(
      ["get_asset", "get_upstream_lineage", "lookup_asset", "search_catalog"].sort(),
    );
  });

  test("search_catalog tool calls /search", async () => {
    const fetchImpl = vi.fn(async (url: URL | string) => {
      expect(url.toString()).toContain("/api/v1/search");
      expect(url.toString()).toContain("query=orders");
      return ok({
        results: [{ id: "a1", metadata: { mrn: "postgres://p/s/orders", type: "Table" } }],
      });
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tools = catalogTools(client);
    const search = tools.find((t) => t.name === "search_catalog") as
      | { invoke: (input: unknown) => Promise<unknown> }
      | undefined;
    if (!search) throw new Error("missing search_catalog");
    const result = await search.invoke({ query: "orders", limit: 5 });
    expect(result).toMatchObject({ results: [{ mrn: "postgres://p/s/orders" }] });
  });

  test("registers a new agent on first chain start", async () => {
    const calls: { method?: string; url: string; body: unknown }[] = [];
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      calls.push({
        method: init?.method,
        url: u,
        body: init?.body ? JSON.parse(init.body as string) : undefined,
      });
      if (u.includes("/lookup/Agent/LangChain/explorer")) {
        return ok({ error: "nf" }, 404);
      }
      if (u.endsWith("/api/v1/assets") && init?.method === "POST") {
        return ok({ id: "agent-1", mrn: "marmot://langchain/agent/explorer" });
      }
      throw new Error(`unexpected request: ${init?.method} ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    const handler = new MarmotCallbackHandler(client, {
      name: "explorer",
      model: "gpt-4o",
      owner: "data-eng",
    });
    await handler.handleChainStart({} as never, {}, "run-1");

    expect(handler.agentMrn).toBe("marmot://langchain/agent/explorer");
    const post = calls.find((c) => c.method === "POST");
    expect(post?.body).toMatchObject({
      name: "explorer",
      type: "Agent",
      providers: ["LangChain"],
      metadata: { framework: "LangChain", model: "gpt-4o", owner: "data-eng" },
    });
  });

  test("updates an existing agent on subsequent runs", async () => {
    const calls: { method?: string; url: string }[] = [];
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      calls.push({ method: init?.method, url: u });
      if (u.includes("/lookup/Agent/LangChain/explorer")) {
        return ok({ id: "agent-1", mrn: "marmot://langchain/agent/explorer" });
      }
      if (u.endsWith("/api/v1/assets/agent-1") && init?.method === "PUT") {
        return ok({});
      }
      throw new Error(`unexpected: ${init?.method} ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    const handler = new MarmotCallbackHandler(client, { name: "explorer" });
    await handler.handleChainStart({} as never, {}, "run-1");

    expect(handler.agentMrn).toBe("marmot://langchain/agent/explorer");
    expect(calls.some((c) => c.method === "PUT")).toBe(true);
  });

  test("captures lineage from tool metadata and flushes on chain end", async () => {
    let batchBody: unknown = null;
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      if (u.includes("/lookup/Agent/LangChain/explorer")) {
        return ok({ id: "agent-1", mrn: "marmot://langchain/agent/explorer" });
      }
      if (u.endsWith("/api/v1/assets/agent-1") && init?.method === "PUT") return ok({});
      if (u.endsWith("/api/v1/lineage/batch")) {
        batchBody = JSON.parse(init?.body as string);
        return ok([]);
      }
      throw new Error(`unexpected: ${init?.method} ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    const handler = new MarmotCallbackHandler(client, { name: "explorer" });
    await handler.handleChainStart({} as never, {}, "root");
    await handler.handleToolStart(
      { name: "query_orders" } as never,
      "select * from orders",
      "tool-1",
      "root",
      undefined,
      { marmot_asset_mrn: "postgres://prod/sales/orders" },
    );
    await handler.handleToolEnd("ok", "tool-1", "root");
    await handler.handleChainEnd({}, "root");

    expect(batchBody).toEqual([
      {
        source: "postgres://prod/sales/orders",
        target: "marmot://langchain/agent/explorer",
        type: "DIRECT",
      },
    ]);
  });

  test("extracts MRNs from tool output dicts", async () => {
    let batchBody: unknown = null;
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      if (u.includes("/lookup/Agent/")) {
        return ok({ id: "agent-1", mrn: "marmot://langchain/agent/explorer" });
      }
      if (init?.method === "PUT") return ok({});
      if (u.endsWith("/lineage/batch")) {
        batchBody = JSON.parse(init?.body as string);
        return ok([]);
      }
      throw new Error(`unexpected: ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);

    const handler = new MarmotCallbackHandler(client, { name: "explorer" });
    await handler.handleChainStart({} as never, {}, "root");
    await handler.handleToolEnd(
      { results: [{ id: "x", mrn: "kafka://c/orders.events" }] },
      "tool-1",
      "root",
    );
    await handler.handleChainEnd({}, "root");

    const sources = (batchBody as { source: string }[]).map((e) => e.source);
    expect(sources).toEqual(["kafka://c/orders.events"]);
  });

  test("marmotTool stamps the asset MRN into tool metadata", () => {
    const t = marmotTool({
      name: "query_orders",
      description: "Run a SQL query.",
      assetMrn: "postgres://p/s/orders",
      schema: {
        type: "object",
        properties: { sql: { type: "string" } },
        required: ["sql"],
      },
      func: async (_input: { sql: string }) => "ok",
    });
    expect(t.name).toBe("query_orders");
    expect(t.metadata).toMatchObject({ marmot_asset_mrn: "postgres://p/s/orders" });
  });
});
