import type { Client } from "../../client.js";
import { extractMrns, sha256Hex } from "../_shared.js";
import { type TranscriptSummary, summarizeTranscript } from "./transcript.js";

const DEFAULT_SERVICE = "ClaudeAgent";
const AGENT_ASSET_TYPE = "Agent";

export interface MarmotAgentTrackerOptions {
  /** Stable name for the agent — used as the asset's name. */
  name: string;
  /** Asset `service`/`providers` value. Defaults to `"ClaudeAgent"`. */
  service?: string;
  /** Recorded on the asset for context. */
  model?: string;
  version?: string;
  owner?: string;
  /**
   * Optional system prompt. A 16-character SHA-256 prefix is stored on the
   * asset so changes in prompt are visible without leaking the prompt itself.
   */
  systemPrompt?: string;
  extraMetadata?: Record<string, unknown>;
}

/**
 * Shape of the input object Claude Agent SDK passes to a hook callback.
 * Declared locally so the SDK package isn't required at compile time.
 */
interface HookInput {
  hook_event_name?: string;
  session_id?: string;
  tool_name?: string;
  tool_input?: unknown;
  tool_response?: unknown;
  tool_output?: unknown;
  tool_use_id?: string;
  transcript_path?: string;
  error?: string;
  [k: string]: unknown;
}

type HookCallback = (
  input: HookInput,
  toolUseID?: string,
  context?: unknown,
) => Promise<Record<string, unknown>>;

interface HookMatcher {
  matcher?: string;
  hooks: HookCallback[];
}

export interface MarmotAgentHooks {
  SessionStart: HookMatcher[];
  PreToolUse: HookMatcher[];
  PostToolUse: HookMatcher[];
  PostToolUseFailure: HookMatcher[];
  Stop: HookMatcher[];
}

interface ToolOpen {
  toolName: string;
  startedAt: Date;
}

interface ToolCallRecord {
  toolName: string;
  startedAt: Date;
  status: string;
  targetMrn?: string;
  durationMs?: number;
}

interface RunState {
  startedAt: Date;
  transcriptPath?: string;
  upstreams: Set<string>;
  toolCalls: ToolCallRecord[];
  toolOpen: Map<string, ToolOpen>;
  error?: string;
}

/**
 * Auto-registers a Claude Agent SDK agent as a Marmot `Agent` asset and
 * captures, per session, lineage edges, per-tool timing, and token usage
 * from the on-disk transcript.
 *
 * Pass the result of {@link hooks} to `ClaudeAgentOptions.hooks`:
 *
 * ```ts
 * const tracker = new MarmotAgentTracker(client, { name: "catalog-explorer" });
 * for await (const msg of query({
 *   prompt: "Find orders data",
 *   options: {
 *     mcpServers: { marmot: { command: "marmot", args: ["mcp"] } },
 *     hooks: tracker.hooks(),
 *   },
 * })) { ... }
 * console.log(tracker.agentMrn);
 * ```
 *
 * The first time any hook fires, the tracker upserts the agent asset and
 * starts a per-session `RunState`. On `Stop` it reads the JSONL transcript
 * at `transcript_path` for real token totals + wall-clock bounds, POSTs one
 * `agent_runs` record (with hook-captured per-tool timing), then POSTs a
 * single batched lineage call. Transcript reads are best-effort — a missing
 * or malformed file leaves the run record populated with hook-derived data
 * only (tokens = 0).
 */
export class MarmotAgentTracker {
  private agentMrnInternal: string | null = null;
  private agentId: string | null = null;
  private registerPromise: Promise<void> | null = null;
  private runs = new Map<string, RunState>();
  private fallbackRun: RunState | null = null;

  constructor(
    private readonly client: Client,
    private readonly opts: MarmotAgentTrackerOptions,
  ) {}

  /** MRN of the registered agent asset, once it has been upserted. */
  get agentMrn(): string | null {
    return this.agentMrnInternal;
  }

  /**
   * Manually record an upstream MRN. Use from inside a custom tool when the
   * tool's output doesn't include an `mrn` field.
   */
  recordSource(mrn: string, sessionId?: string): void {
    this.runState(sessionId).upstreams.add(mrn);
  }

  /**
   * Returns a hook map suitable for `ClaudeAgentOptions.hooks` in Claude
   * Agent SDK 0.x.
   */
  hooks(): MarmotAgentHooks {
    const onSessionStart: HookCallback = async (input) => {
      await this.ensureRegistered();
      const state = this.runState(sessionIdOf(input));
      captureTranscriptPath(state, input);
      return {};
    };
    const onPreToolUse: HookCallback = async (input, toolUseId) => {
      await this.ensureRegistered();
      const state = this.runState(sessionIdOf(input));
      captureTranscriptPath(state, input);
      const id = toolUseIdOf(input, toolUseId);
      const name = typeof input.tool_name === "string" ? input.tool_name : undefined;
      if (id && name) {
        state.toolOpen.set(id, { toolName: name, startedAt: new Date() });
      }
      return {};
    };
    const onPostToolUse: HookCallback = async (input, toolUseId) => {
      await this.ensureRegistered();
      const state = this.runState(sessionIdOf(input));
      captureTranscriptPath(state, input);
      const output = input.tool_response ?? input.tool_output;
      let observed: string[] = [];
      if (output !== undefined) {
        observed = Array.from(extractMrns(output));
        for (const mrn of observed) state.upstreams.add(mrn);
      }
      this.closeToolCall(state, input, toolUseIdOf(input, toolUseId), "success", observed[0]);
      return {};
    };
    const onPostToolUseFailure: HookCallback = async (input, toolUseId) => {
      await this.ensureRegistered();
      const state = this.runState(sessionIdOf(input));
      captureTranscriptPath(state, input);
      if (typeof input.error === "string" && input.error) {
        state.error = input.error;
      }
      this.closeToolCall(state, input, toolUseIdOf(input, toolUseId), "error", undefined);
      return {};
    };
    const onStop: HookCallback = async (input) => {
      const existing = this.runStateOptional(sessionIdOf(input));
      if (existing) captureTranscriptPath(existing, input);
      await this.flush(sessionIdOf(input));
      return {};
    };
    return {
      SessionStart: [{ hooks: [onSessionStart] }],
      PreToolUse: [{ hooks: [onPreToolUse] }],
      PostToolUse: [{ hooks: [onPostToolUse] }],
      PostToolUseFailure: [{ hooks: [onPostToolUseFailure] }],
      Stop: [{ hooks: [onStop] }],
    };
  }

  /**
   * Manually upsert the Agent asset. Normally called automatically on the
   * first hook invocation; call directly when your flow can't guarantee a
   * hook will fire (e.g. an agent that never calls a tool).
   */
  async register(): Promise<void> {
    await this.ensureRegistered();
  }

  /**
   * Flush the pending run for `sessionId` (or the fallback bucket). Posts
   * the run record and lineage edges, then clears state. Called
   * automatically on the `Stop` hook.
   */
  async flush(sessionId?: string): Promise<void> {
    const run = this.takeRun(sessionId);
    if (!run) return;
    await this.ensureRegistered();
    if (!this.agentMrnInternal) return;

    let summary: TranscriptSummary | null = null;
    if (run.transcriptPath) {
      try {
        summary = await summarizeTranscript(run.transcriptPath);
      } catch {
        summary = null;
      }
    }
    await this.postRun(sessionId, run, summary);

    if (run.upstreams.size > 0) {
      const target = this.agentMrnInternal;
      const edges = Array.from(run.upstreams)
        .sort()
        .map((source) => ({ source, target }));
      try {
        await this.client.lineage.batch(edges);
      } catch (e) {
        console.warn("[marmot] failed to write lineage:", e);
      }
    }
  }

  private runState(sessionId: string | undefined): RunState {
    if (!sessionId) {
      if (!this.fallbackRun) this.fallbackRun = newRunState();
      return this.fallbackRun;
    }
    let state = this.runs.get(sessionId);
    if (!state) {
      state = newRunState();
      this.runs.set(sessionId, state);
    }
    return state;
  }

  private runStateOptional(sessionId: string | undefined): RunState | null {
    if (!sessionId) return this.fallbackRun;
    return this.runs.get(sessionId) ?? null;
  }

  private takeRun(sessionId: string | undefined): RunState | null {
    if (!sessionId) {
      const run = this.fallbackRun;
      this.fallbackRun = null;
      return run;
    }
    const run = this.runs.get(sessionId) ?? null;
    if (run) this.runs.delete(sessionId);
    return run;
  }

  private closeToolCall(
    state: RunState,
    input: HookInput,
    toolUseId: string | undefined,
    status: string,
    targetMrn: string | undefined,
  ): void {
    let opened: ToolOpen | undefined;
    if (toolUseId) {
      opened = state.toolOpen.get(toolUseId);
      if (opened) state.toolOpen.delete(toolUseId);
    }
    const ended = new Date();
    let toolName: string;
    let startedAt: Date;
    let durationMs: number | undefined;
    if (opened) {
      toolName = opened.toolName;
      startedAt = opened.startedAt;
      durationMs = Math.max(0, ended.getTime() - startedAt.getTime());
    } else {
      toolName = typeof input.tool_name === "string" ? input.tool_name : "tool";
      startedAt = ended;
      durationMs = undefined;
    }
    const record: ToolCallRecord = { toolName, startedAt, status };
    if (targetMrn !== undefined) record.targetMrn = targetMrn;
    if (durationMs !== undefined) record.durationMs = durationMs;
    state.toolCalls.push(record);
  }

  private async postRun(
    sessionId: string | undefined,
    run: RunState,
    summary: TranscriptSummary | null,
  ): Promise<void> {
    const startedAt = summary?.startedAt ?? run.startedAt;
    const endedAt = summary?.endedAt ?? new Date();
    const tokensIn = summary?.tokensIn ?? 0;
    const tokensOut = summary?.tokensOut ?? 0;
    const status = run.error ? "error" : "success";
    const runId = sessionId ?? syntheticRunId(startedAt);

    const explicit = new Set(
      run.toolCalls.map((tc) => tc.targetMrn).filter((m): m is string => !!m),
    );
    const observedExtras = Array.from(run.upstreams)
      .filter((m) => !explicit.has(m) && m !== this.agentMrnInternal)
      .sort();

    const args: import("../../resources/agent_runs.js").RecordAgentRunArgs = {
      agentMrn: this.agentMrnInternal ?? "",
      runId,
      startedAt,
      endedAt,
      status,
      tokensIn,
      tokensOut,
    };
    if (this.opts.model !== undefined) args.model = this.opts.model;
    if (run.error !== undefined) args.error = run.error;
    if (run.toolCalls.length) args.toolCalls = run.toolCalls.map(toToolCallInput);
    if (observedExtras.length) args.observedAssets = observedExtras;
    try {
      await this.client.agentRuns.create(args);
    } catch (e) {
      console.warn("[marmot] failed to record Claude Agent run:", e);
    }
  }

  private ensureRegistered(): Promise<void> {
    if (this.agentMrnInternal !== null) return Promise.resolve();
    if (this.registerPromise) return this.registerPromise;
    this.registerPromise = this.doRegister();
    return this.registerPromise;
  }

  private async doRegister(): Promise<void> {
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
    const metadata: Record<string, unknown> = { framework: "ClaudeAgent" };
    if (this.opts.model) metadata.model = this.opts.model;
    if (this.opts.version) metadata.version = this.opts.version;
    if (this.opts.owner) metadata.owner = this.opts.owner;
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

function newRunState(): RunState {
  return {
    startedAt: new Date(),
    upstreams: new Set(),
    toolCalls: [],
    toolOpen: new Map(),
  };
}

function captureTranscriptPath(state: RunState, input: HookInput): void {
  if (state.transcriptPath) return;
  if (typeof input.transcript_path === "string" && input.transcript_path) {
    state.transcriptPath = input.transcript_path;
  }
}

function sessionIdOf(input: HookInput): string | undefined {
  return typeof input.session_id === "string" ? input.session_id : undefined;
}

function toolUseIdOf(input: HookInput, arg: string | undefined): string | undefined {
  if (arg) return arg;
  return typeof input.tool_use_id === "string" ? input.tool_use_id : undefined;
}

function syntheticRunId(startedAt: Date): string {
  return `run-${startedAt.getTime()}`;
}

function toToolCallInput(
  tc: ToolCallRecord,
): import("../../resources/agent_runs.js").ToolCallInput {
  const input: import("../../resources/agent_runs.js").ToolCallInput = {
    toolName: tc.toolName,
    startedAt: tc.startedAt,
    status: tc.status,
  };
  if (tc.targetMrn !== undefined) input.targetMrn = tc.targetMrn;
  if (tc.durationMs !== undefined) input.durationMs = tc.durationMs;
  return input;
}
