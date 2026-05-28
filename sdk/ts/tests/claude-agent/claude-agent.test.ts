import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { Credential } from "../../src/auth/index.js";
import { Client } from "../../src/client.js";
import { MarmotAgentTracker } from "../../src/integrations/claude-agent/index.js";
import { summarizeTranscript } from "../../src/integrations/claude-agent/transcript.js";

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

// All Stop hooks now POST /agents/runs, so every fixture needs a handler for it.
// Tests that care about the body capture it via the optional `sink`.
function makeFetch(opts: {
  lineageSink?: { body: unknown };
  runsSink?: unknown[];
  agentExists?: boolean;
  failIfLineage?: boolean;
}): typeof fetch {
  const { lineageSink, runsSink, agentExists = false, failIfLineage = false } = opts;
  return vi.fn(async (url: URL | string, init?: RequestInit) => {
    const u = url.toString();
    if (u.includes("/lookup/Agent/")) {
      return agentExists
        ? ok({ id: "agent-1", mrn: "marmot://claude/agent/explorer" })
        : ok(null, 404);
    }
    if (u.endsWith("/api/v1/assets") && init?.method === "POST") {
      return ok({ id: "agent-1", mrn: "marmot://claude/agent/explorer" });
    }
    if (u.endsWith("/api/v1/assets/agent-1") && init?.method === "PUT") {
      return ok({});
    }
    if (u.endsWith("/api/v1/lineage/batch")) {
      if (failIfLineage) throw new Error("lineage/batch should not be called");
      if (lineageSink) lineageSink.body = JSON.parse(init?.body as string);
      return ok([]);
    }
    if (u.endsWith("/api/v1/agents/runs")) {
      if (runsSink) runsSink.push(JSON.parse(init?.body as string));
      return ok({
        id: "run-1",
        agent_id: "agent-1",
        run_id: "x",
        started_at: "2026-01-01T00:00:00Z",
        status: "success",
        tokens_in: 0,
        tokens_out: 0,
        created_at: "2026-01-01T00:00:00Z",
      });
    }
    throw new Error(`unexpected: ${init?.method ?? "GET"} ${u}`);
  }) as unknown as typeof fetch;
}

describe("claude-agent integration", () => {
  test("hooks() returns the full set of lifecycle events with one callback each", () => {
    const client = makeClient(vi.fn() as unknown as typeof fetch);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const hooks = tracker.hooks();
    expect(Object.keys(hooks).sort()).toEqual([
      "PostToolUse",
      "PostToolUseFailure",
      "PreToolUse",
      "SessionStart",
      "Stop",
    ]);
    for (const event of [
      "SessionStart",
      "PreToolUse",
      "PostToolUse",
      "PostToolUseFailure",
      "Stop",
    ] as const) {
      expect(hooks[event]).toHaveLength(1);
      expect(hooks[event][0]!.hooks).toHaveLength(1);
    }
  });

  test("registers the agent asset on first hook and writes lineage on Stop", async () => {
    const lineageSink: { body: unknown } = { body: null };
    const fetchImpl = makeFetch({ lineageSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, {
      name: "explorer",
      model: "claude-sonnet-4-6",
      owner: "data-eng",
    });

    const hooks = tracker.hooks();
    const sessionStart = hooks.SessionStart[0]!.hooks[0]!;
    const postToolUse = hooks.PostToolUse[0]!.hooks[0]!;
    const stop = hooks.Stop[0]!.hooks[0]!;

    await sessionStart({ hook_event_name: "SessionStart", session_id: "s1" });
    expect(tracker.agentMrn).toBe("marmot://claude/agent/explorer");

    await postToolUse({
      hook_event_name: "PostToolUse",
      session_id: "s1",
      tool_name: "mcp__marmot__search_catalog",
      tool_response: {
        results: [
          { id: "x", mrn: "postgres://p/s/orders" },
          { id: "y", mrn: "kafka://c/orders.events" },
        ],
      },
    });

    await stop({ hook_event_name: "Stop", session_id: "s1" });

    const edges = lineageSink.body as { source: string; target: string }[];
    expect(edges.map((e) => e.source).sort()).toEqual([
      "kafka://c/orders.events",
      "postgres://p/s/orders",
    ]);
    expect(edges.every((e) => e.target === "marmot://claude/agent/explorer")).toBe(true);
  });

  test("PostToolUse also registers when SessionStart did not fire (Python parity)", async () => {
    const lineageSink: { body: unknown } = { body: null };
    const fetchImpl = makeFetch({ lineageSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { PostToolUse, Stop } = tracker.hooks();

    await PostToolUse[0]!.hooks[0]!({
      hook_event_name: "PostToolUse",
      session_id: "s2",
      tool_name: "mcp__marmot__lookup_asset",
      tool_response: { mrn: "postgres://p/s/orders" },
    });
    expect(tracker.agentMrn).toBe("marmot://claude/agent/explorer");

    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s2" });
    expect((lineageSink.body as { source: string }[])[0]!.source).toBe("postgres://p/s/orders");
  });

  test("recordSource lets a custom tool attribute a runtime MRN", async () => {
    const lineageSink: { body: unknown } = { body: null };
    const fetchImpl = makeFetch({ lineageSink, agentExists: true });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { SessionStart, Stop } = tracker.hooks();

    await SessionStart[0]!.hooks[0]!({ hook_event_name: "SessionStart", session_id: "s3" });
    tracker.recordSource("s3://bucket/key.parquet", "s3");
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s3" });

    expect((lineageSink.body as { source: string }[])[0]!.source).toBe("s3://bucket/key.parquet");
  });

  test("upserts (PUT) when the agent asset already exists", async () => {
    let putCalled = false;
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      if (u.includes("/lookup/Agent/")) {
        return ok({ id: "agent-1", mrn: "marmot://claude/agent/explorer" });
      }
      if (u.endsWith("/api/v1/assets/agent-1") && init?.method === "PUT") {
        putCalled = true;
        const body = JSON.parse(init?.body as string);
        expect(body.metadata.framework).toBe("ClaudeAgent");
        expect(body.metadata.model).toBe("claude-sonnet-4-6");
        return ok({});
      }
      throw new Error(`unexpected: ${init?.method} ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tracker = new MarmotAgentTracker(client, {
      name: "explorer",
      model: "claude-sonnet-4-6",
    });
    await tracker.register();
    expect(putCalled).toBe(true);
    expect(tracker.agentMrn).toBe("marmot://claude/agent/explorer");
  });

  test("concurrent registration only upserts once", async () => {
    let postCount = 0;
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const u = url.toString();
      if (u.includes("/lookup/Agent/")) return ok(null, 404);
      if (u.endsWith("/api/v1/assets") && init?.method === "POST") {
        postCount += 1;
        return ok({ id: "agent-1", mrn: "marmot://claude/agent/explorer" });
      }
      throw new Error(`unexpected: ${init?.method} ${u}`);
    });
    const client = makeClient(fetchImpl as unknown as typeof fetch);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    await Promise.all([tracker.register(), tracker.register(), tracker.register()]);
    expect(postCount).toBe(1);
  });

  test("Stop with no upstreams skips the lineage call", async () => {
    const fetchImpl = makeFetch({ failIfLineage: true });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { SessionStart, Stop } = tracker.hooks();
    await SessionStart[0]!.hooks[0]!({ hook_event_name: "SessionStart", session_id: "s4" });
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s4" });
  });

  test("captures MRNs from MCP content[].text markdown envelopes", async () => {
    const lineageSink: { body: unknown } = { body: null };
    const fetchImpl = makeFetch({ lineageSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { PostToolUse, Stop } = tracker.hooks();
    await PostToolUse[0]!.hooks[0]!({
      hook_event_name: "PostToolUse",
      session_id: "s6",
      tool_name: "mcp__marmot__discover_data",
      tool_response: {
        content: [
          {
            type: "text",
            text:
              "# Found 2 assets\n\n" +
              "- [orders-search](http://localhost:5173/discover/index/orders-search)" +
              " · `mrn://index/elasticsearch/orders-search` · elasticsearch\n" +
              "- [PARTNER_ORDERS](http://localhost:5173/discover/table/PARTNER_ORDERS)" +
              " · `mrn://table/snowflake/glacier.partner.partner_orders` · snowflake\n",
          },
        ],
      },
    });
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s6" });

    const sources = (lineageSink.body as { source: string }[]).map((e) => e.source).sort();
    expect(sources).toEqual([
      "mrn://index/elasticsearch/orders-search",
      "mrn://table/snowflake/glacier.partner.partner_orders",
    ]);
  });

  test("tool_output field is accepted as an alias for tool_response", async () => {
    const lineageSink: { body: unknown } = { body: null };
    const fetchImpl = makeFetch({ lineageSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { PostToolUse, Stop } = tracker.hooks();
    await PostToolUse[0]!.hooks[0]!({
      hook_event_name: "PostToolUse",
      session_id: "s5",
      tool_name: "mcp__marmot__get_asset",
      tool_output: { mrn: "redshift://w/sales/orders" },
    });
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s5" });
    expect((lineageSink.body as { source: string }[])[0]!.source).toBe("redshift://w/sales/orders");
  });

  test("Stop posts an agent_run with per-tool timing", async () => {
    const runsSink: unknown[] = [];
    const fetchImpl = makeFetch({ runsSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, {
      name: "explorer",
      model: "claude-sonnet-4-5",
    });
    const { PreToolUse, PostToolUse, Stop } = tracker.hooks();

    await PreToolUse[0]!.hooks[0]!(
      {
        hook_event_name: "PreToolUse",
        session_id: "s-run",
        tool_name: "mcp__marmot__discover_data",
      },
      "tool-call-1",
    );
    await PostToolUse[0]!.hooks[0]!(
      {
        hook_event_name: "PostToolUse",
        session_id: "s-run",
        tool_name: "mcp__marmot__discover_data",
        tool_response: { mrn: "postgres://p/s/orders" },
      },
      "tool-call-1",
    );
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s-run" });

    expect(runsSink).toHaveLength(1);
    const body = runsSink[0] as Record<string, unknown>;
    expect(body.agent_mrn).toBe("marmot://claude/agent/explorer");
    expect(body.run_id).toBe("s-run");
    expect(body.status).toBe("success");
    expect(body.model).toBe("claude-sonnet-4-5");
    expect(body.tokens_in).toBe(0);
    expect(body.tokens_out).toBe(0);
    const toolCalls = body.tool_calls as Record<string, unknown>[];
    expect(toolCalls).toHaveLength(1);
    expect(toolCalls[0]!.tool_name).toBe("mcp__marmot__discover_data");
    expect(toolCalls[0]!.status).toBe("success");
    expect(toolCalls[0]!.target_mrn).toBe("postgres://p/s/orders");
    expect(toolCalls[0]!.duration_ms).toBeGreaterThanOrEqual(0);
  });

  test("PostToolUseFailure marks the run as error", async () => {
    const runsSink: unknown[] = [];
    const fetchImpl = makeFetch({ runsSink });
    const client = makeClient(fetchImpl);
    const tracker = new MarmotAgentTracker(client, { name: "explorer" });
    const { PreToolUse, PostToolUseFailure, Stop } = tracker.hooks();

    await PreToolUse[0]!.hooks[0]!(
      { hook_event_name: "PreToolUse", session_id: "s-err", tool_name: "broken_tool" },
      "tc-err",
    );
    await PostToolUseFailure[0]!.hooks[0]!(
      {
        hook_event_name: "PostToolUseFailure",
        session_id: "s-err",
        tool_name: "broken_tool",
        error: "permission denied",
      },
      "tc-err",
    );
    await Stop[0]!.hooks[0]!({ hook_event_name: "Stop", session_id: "s-err" });

    const body = runsSink[0] as Record<string, unknown>;
    expect(body.status).toBe("error");
    expect(body.error).toBe("permission denied");
    const toolCalls = body.tool_calls as Record<string, unknown>[];
    expect(toolCalls[0]!.status).toBe("error");
  });

  describe("transcript-driven tokens", () => {
    let tmp: string;
    beforeEach(async () => {
      tmp = await mkdtemp(join(tmpdir(), "marmot-tx-"));
    });
    afterEach(async () => {
      await rm(tmp, { recursive: true, force: true });
    });

    test("Stop reads transcript_path and lands tokens in the run body", async () => {
      const transcript = join(tmp, "session.jsonl");
      await writeFile(
        transcript,
        [
          JSON.stringify({
            type: "assistant",
            timestamp: "2026-05-28T10:00:00.000Z",
            message: {
              usage: {
                input_tokens: 100,
                cache_creation_input_tokens: 200,
                cache_read_input_tokens: 50,
                output_tokens: 80,
              },
            },
          }),
          JSON.stringify({
            type: "assistant",
            timestamp: "2026-05-28T10:00:05.500Z",
            message: { usage: { input_tokens: 10, output_tokens: 30 } },
          }),
        ].join("\n"),
        "utf-8",
      );

      const runsSink: unknown[] = [];
      const fetchImpl = makeFetch({ runsSink });
      const client = makeClient(fetchImpl);
      const tracker = new MarmotAgentTracker(client, { name: "explorer" });
      const { PreToolUse, Stop } = tracker.hooks();

      await PreToolUse[0]!.hooks[0]!(
        {
          hook_event_name: "PreToolUse",
          session_id: "s-tx",
          tool_name: "noop",
          transcript_path: transcript,
        },
        "t1",
      );
      await Stop[0]!.hooks[0]!({
        hook_event_name: "Stop",
        session_id: "s-tx",
        transcript_path: transcript,
      });

      const body = runsSink[0] as Record<string, unknown>;
      expect(body.tokens_in).toBe(100 + 200 + 50 + 10);
      expect(body.tokens_out).toBe(80 + 30);
    });

    test("summarizeTranscript returns null for a missing file", async () => {
      expect(await summarizeTranscript(join(tmp, "nope.jsonl"))).toBeNull();
    });

    test("summarizeTranscript skips malformed lines", async () => {
      const p = join(tmp, "tx.jsonl");
      await writeFile(
        p,
        [
          "not json at all",
          JSON.stringify({ type: "user", timestamp: "2026-05-28T10:00:00Z" }),
          JSON.stringify({
            type: "assistant",
            timestamp: "2026-05-28T10:00:01Z",
            message: { usage: { input_tokens: 5, output_tokens: 7 } },
          }),
          "",
        ].join("\n"),
        "utf-8",
      );
      const summary = await summarizeTranscript(p);
      expect(summary).not.toBeNull();
      expect(summary!.tokensIn).toBe(5);
      expect(summary!.tokensOut).toBe(7);
      expect(summary!.startedAt).not.toBeNull();
      expect(summary!.endedAt).not.toBeNull();
    });
  });
});
