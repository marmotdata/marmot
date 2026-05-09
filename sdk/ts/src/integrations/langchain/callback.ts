import { BaseCallbackHandler } from "@langchain/core/callbacks/base";
import type { Serialized } from "@langchain/core/load/serializable";
import { tool } from "@langchain/core/tools";
import type { ChainValues } from "@langchain/core/utils/types";
import type { Client } from "../../client.js";

type JsonObjectSchema = {
  type: "object";
  properties?: Record<string, unknown>;
  required?: string[];
  [k: string]: unknown;
};

const DEFAULT_SERVICE = "LangChain";
const AGENT_ASSET_TYPE = "Agent";

/** Metadata key used on a tool to declare its upstream Marmot MRN. */
export const TOOL_METADATA_KEY = "marmot_asset_mrn";

export interface MarmotCallbackHandlerOptions {
  /** Stable name for the agent — used as the asset's name. */
  name: string;
  service?: string;
  model?: string;
  version?: string;
  owner?: string;
  /** The tools attached to the agent — recorded on the asset for context. */
  tools?: { name: string }[];
  /**
   * Optional system prompt. A 16-character SHA-256 prefix is stored on the
   * asset so changes in prompt are visible without leaking the prompt itself.
   */
  systemPrompt?: string;
  extraMetadata?: Record<string, unknown>;
}

/**
 * LangChain callback handler that auto-registers the agent and captures
 * lineage to the data sources it reads.
 *
 * The first time the handler observes a chain start, it upserts an asset of
 * type `Agent` keyed by `(service="LangChain", name=name)`. As the agent
 * runs, every tool call that resolves to an asset MRN is collected; on
 * chain end (or error), a single batched lineage write attributes those
 * edges to the agent.
 *
 * See {@link marmotTool} for declaring upstream MRNs on custom tools, or
 * call {@link recordSource} from inside a tool implementation.
 */
export class MarmotCallbackHandler extends BaseCallbackHandler {
  override name = "MarmotCallbackHandler";

  private agentMrnInternal: string | null = null;
  private agentId: string | null = null;
  private rootOf = new Map<string, string>();
  private upstreams = new Map<string, Set<string>>();

  constructor(
    private readonly client: Client,
    private readonly opts: MarmotCallbackHandlerOptions,
  ) {
    super();
  }

  /** The MRN of the registered agent asset, once it has been upserted. */
  get agentMrn(): string | null {
    return this.agentMrnInternal;
  }

  /**
   * Manually record an upstream MRN as having been read during the current
   * (or specified) run. Use from inside a custom tool implementation when
   * neither metadata-tagging via {@link marmotTool} nor the auto-extraction
   * from tool output is convenient.
   */
  recordSource(mrn: string, runId?: string): void {
    let root: string | undefined;
    if (runId) {
      root = this.rootOf.get(runId) ?? runId;
    } else {
      root = this.upstreams.keys().next().value;
    }
    if (!root) return;
    let set = this.upstreams.get(root);
    if (!set) {
      set = new Set();
      this.upstreams.set(root, set);
    }
    set.add(mrn);
  }

  override async handleChainStart(
    _chain: Serialized,
    _inputs: ChainValues,
    runId: string,
    _runType?: string,
    _tags?: string[],
    _metadata?: Record<string, unknown>,
    _runName?: string,
    parentRunId?: string,
  ): Promise<void> {
    if (parentRunId === undefined) {
      this.rootOf.set(runId, runId);
      this.upstreams.set(runId, new Set());
      await this.ensureAgentRegistered();
    } else {
      const root = this.rootOf.get(parentRunId) ?? parentRunId;
      this.rootOf.set(runId, root);
    }
  }

  override async handleToolStart(
    _tool: Serialized,
    _input: string,
    runId: string,
    parentRunId?: string,
    _tags?: string[],
    metadata?: Record<string, unknown>,
  ): Promise<void> {
    const root = this.resolveRoot(runId, parentRunId);
    if (!root) return;
    this.rootOf.set(runId, root);

    const mrn = metadata?.[TOOL_METADATA_KEY];
    if (typeof mrn === "string" && mrn) {
      this.upstreamsFor(root).add(mrn);
    }
  }

  override async handleToolEnd(
    output: unknown,
    runId: string,
    parentRunId?: string,
  ): Promise<void> {
    const root = this.resolveRoot(runId, parentRunId);
    if (!root) return;
    for (const mrn of extractMrns(output)) {
      this.upstreamsFor(root).add(mrn);
    }
  }

  override async handleRetrieverStart(
    _retriever: Serialized,
    _query: string,
    runId: string,
    parentRunId?: string,
    _tags?: string[],
    metadata?: Record<string, unknown>,
  ): Promise<void> {
    const root = this.resolveRoot(runId, parentRunId);
    if (!root) return;
    this.rootOf.set(runId, root);
    const mrn = metadata?.[TOOL_METADATA_KEY];
    if (typeof mrn === "string" && mrn) {
      this.upstreamsFor(root).add(mrn);
    }
  }

  override async handleChainEnd(
    _outputs: ChainValues,
    runId: string,
    parentRunId?: string,
  ): Promise<void> {
    if (parentRunId === undefined) {
      await this.flush(runId);
    }
  }

  override async handleChainError(
    _err: unknown,
    runId: string,
    parentRunId?: string,
  ): Promise<void> {
    if (parentRunId === undefined) {
      await this.flush(runId);
    }
  }

  private resolveRoot(runId: string, parentRunId: string | undefined): string | null {
    const cached = this.rootOf.get(runId);
    if (cached) return cached;
    if (parentRunId !== undefined) {
      const fromParent = this.rootOf.get(parentRunId);
      if (fromParent) return fromParent;
    }
    return null;
  }

  private upstreamsFor(root: string): Set<string> {
    let set = this.upstreams.get(root);
    if (!set) {
      set = new Set();
      this.upstreams.set(root, set);
    }
    return set;
  }

  private async flush(rootRunId: string): Promise<void> {
    const sources = this.upstreams.get(rootRunId);
    this.upstreams.delete(rootRunId);
    for (const [k, v] of this.rootOf) {
      if (v === rootRunId) this.rootOf.delete(k);
    }
    if (!sources || sources.size === 0 || !this.agentMrnInternal) return;
    const target = this.agentMrnInternal;
    const edges = Array.from(sources, (source) => ({ source, target }));
    try {
      await this.client.lineage.batch(edges);
    } catch (e) {
      console.warn("[marmot] failed to write lineage:", e);
    }
  }

  private async ensureAgentRegistered(): Promise<void> {
    if (this.agentMrnInternal !== null) return;
    const service = this.opts.service ?? DEFAULT_SERVICE;
    let existing: Record<string, unknown> | null = null;
    try {
      existing = await this.client.assets.find({
        type: AGENT_ASSET_TYPE,
        service,
        name: this.opts.name,
      });
    } catch (e) {
      console.warn("[marmot] failed to look up agent asset:", e);
      return;
    }

    const payload = await this.buildAssetPayload(service);
    try {
      if (existing === null) {
        const created = await this.client.assets.create(payload);
        this.agentId = (created.id as string | undefined) ?? null;
        this.agentMrnInternal = (created.mrn as string | undefined) ?? null;
      } else {
        this.agentId = (existing.id as string | undefined) ?? null;
        this.agentMrnInternal = (existing.mrn as string | undefined) ?? null;
        if (this.agentId) {
          await this.client.assets.update(this.agentId, payload);
        }
      }
    } catch (e) {
      console.warn("[marmot] failed to upsert agent asset:", e);
    }
  }

  private async buildAssetPayload(service: string): Promise<Record<string, unknown>> {
    const metadata: Record<string, unknown> = { framework: "LangChain" };
    if (this.opts.model) metadata.model = this.opts.model;
    if (this.opts.version) metadata.version = this.opts.version;
    if (this.opts.owner) metadata.owner = this.opts.owner;
    if (this.opts.tools) metadata.tool_names = this.opts.tools.map((t) => t.name);
    if (this.opts.systemPrompt) {
      metadata.system_prompt_sha256_16 = (await sha256Hex(this.opts.systemPrompt)).slice(0, 16);
    }
    Object.assign(metadata, this.opts.extraMetadata ?? {});

    return {
      name: this.opts.name,
      type: AGENT_ASSET_TYPE,
      providers: [service],
      services: [service],
      metadata,
    };
  }
}

/**
 * Helper that wraps a function in a LangChain tool tagged with the upstream
 * MRN it reads. The {@link MarmotCallbackHandler} picks up the tag and
 * records an edge from `assetMrn` to the agent on every call.
 *
 * ```ts
 * const queryOrders = marmotTool({
 *   name: "query_orders",
 *   description: "Run a SQL query against the orders table.",
 *   assetMrn: "postgres://prod/sales/orders",
 *   schema: z.object({ sql: z.string() }),
 *   func: async ({ sql }) => runSql(sql),
 * });
 * ```
 */
export function marmotTool<TInput = Record<string, unknown>>(args: {
  name: string;
  description: string;
  assetMrn: string;
  schema: JsonObjectSchema;
  func: (input: TInput) => unknown | Promise<unknown>;
  metadata?: Record<string, unknown>;
}) {
  return tool(args.func as (input: unknown) => unknown, {
    name: args.name,
    description: args.description,
    schema: args.schema,
    metadata: { ...(args.metadata ?? {}), [TOOL_METADATA_KEY]: args.assetMrn },
  });
}

function extractMrns(value: unknown): Set<string> {
  const out = new Set<string>();
  walkForMrns(value, out, 0);
  return out;
}

function walkForMrns(value: unknown, out: Set<string>, depth: number): void {
  if (depth > 4) return;
  if (value === null || value === undefined) return;
  if (Array.isArray(value)) {
    for (const v of value) walkForMrns(v, out, depth + 1);
    return;
  }
  if (typeof value === "object") {
    const obj = value as Record<string, unknown>;
    const mrn = obj.mrn;
    if (typeof mrn === "string" && mrn) out.add(mrn);
    for (const v of Object.values(obj)) {
      if (v !== null && (typeof v === "object" || Array.isArray(v))) {
        walkForMrns(v, out, depth + 1);
      }
    }
  }
}

async function sha256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input);
  const buf = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, "0")).join("");
}
