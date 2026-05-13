import type { Transport } from "../_http.js";
import type {
  ActivityResponse,
  AgentRun,
  RecordRunRequest,
  RunsResponse,
  Stats,
  ToolCallPayload,
} from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface ToolCallInput {
  toolName: string;
  startedAt: Date | string;
  status?: string;
  targetMrn?: string;
  durationMs?: number;
}

export interface RecordAgentRunArgs {
  agentMrn: string;
  runId: string;
  startedAt: Date | string;
  status: string;
  endedAt?: Date | string;
  model?: string;
  tokensIn?: number;
  tokensOut?: number;
  error?: string;
  toolCalls?: ToolCallInput[];
  observedAssets?: string[];
}

/**
 * Record and read agent invocation telemetry.
 *
 * `create` posts a completed run; the server persists `agent_runs` and
 * `agent_tool_calls` rows and emits one `AGENT_LOOKUP` observed lineage
 * edge per tool call that resolved to a catalogued asset.
 */
export class AgentRunsResource {
  constructor(private readonly transport: Transport) {}

  async create(args: RecordAgentRunArgs): Promise<AgentRun> {
    const body: RecordRunRequest = {
      agent_mrn: args.agentMrn,
      run_id: args.runId,
      started_at: toIso(args.startedAt),
      status: args.status,
      tokens_in: args.tokensIn ?? 0,
      tokens_out: args.tokensOut ?? 0,
    };
    if (args.endedAt !== undefined) body.ended_at = toIso(args.endedAt);
    if (args.model !== undefined) body.model = args.model;
    if (args.error !== undefined) body.error = args.error;
    if (args.toolCalls?.length) body.tool_calls = args.toolCalls.map(normalizeToolCall);
    if (args.observedAssets?.length) body.observed_assets = [...args.observedAssets];
    return this.transport.post<AgentRun>(`${API_PREFIX}/agents/runs`, body);
  }

  async list(
    assetId: string,
    opts: { period?: string; limit?: number } = {},
  ): Promise<RunsResponse> {
    const query: Record<string, unknown> = {
      period: opts.period ?? "24h",
      limit: opts.limit ?? 25,
    };
    return this.transport.get<RunsResponse>(`${API_PREFIX}/agents/${assetId}/runs`, query);
  }

  async stats(assetId: string, opts: { period?: string } = {}): Promise<Stats> {
    return this.transport.get<Stats>(`${API_PREFIX}/agents/${assetId}/stats`, {
      period: opts.period ?? "24h",
    });
  }

  async activity(assetId: string, opts: { period?: string } = {}): Promise<ActivityResponse> {
    return this.transport.get<ActivityResponse>(`${API_PREFIX}/agents/${assetId}/activity`, {
      period: opts.period ?? "24h",
    });
  }
}

function toIso(t: Date | string): string {
  if (typeof t === "string") return t;
  return t.toISOString();
}

function normalizeToolCall(tc: ToolCallInput): ToolCallPayload {
  const payload: ToolCallPayload = {
    tool_name: tc.toolName,
    started_at: toIso(tc.startedAt),
    status: tc.status ?? "success",
  };
  if (tc.targetMrn !== undefined) payload.target_mrn = tc.targetMrn;
  if (tc.durationMs !== undefined) payload.duration_ms = tc.durationMs;
  return payload;
}
