import type { Transport } from "../_http.js";
import type { BatchLineageResult, LineageEdge, LineageResponse } from "../_models.js";
import { API_PREFIX } from "./index.js";

export const DEFAULT_EDGE_TYPE = "DIRECT";

export type EdgeInput = LineageEdge | [string, string] | [string, string, string];

export interface LineageWriteArgs {
  source: string;
  target: string;
  type?: string;
  jobMrn?: string;
}

export interface LineageGetOptions {
  direction?: "upstream" | "downstream" | "both";
  /** Maximum traversal depth. */
  depth?: number;
}

export class LineageResource {
  constructor(private readonly transport: Transport) {}

  async get(assetId: string, opts: LineageGetOptions = {}): Promise<LineageResponse> {
    const query: Record<string, unknown> = {};
    if (opts.direction !== undefined) query.direction = opts.direction;
    if (opts.depth !== undefined) query.depth = opts.depth;
    return this.transport.get<LineageResponse>(`${API_PREFIX}/lineage/assets/${assetId}`, query);
  }

  async upstream(assetId: string, opts: { depth?: number } = {}): Promise<LineageResponse> {
    const args: LineageGetOptions = { direction: "upstream" };
    if (opts.depth !== undefined) args.depth = opts.depth;
    return this.get(assetId, args);
  }

  async downstream(assetId: string, opts: { depth?: number } = {}): Promise<LineageResponse> {
    const args: LineageGetOptions = { direction: "downstream" };
    if (opts.depth !== undefined) args.depth = opts.depth;
    return this.get(assetId, args);
  }

  async write(args: LineageWriteArgs): Promise<LineageEdge> {
    const body: LineageEdge = {
      source: args.source,
      target: args.target,
      type: args.type ?? DEFAULT_EDGE_TYPE,
    };
    if (args.jobMrn) body.job_mrn = args.jobMrn;
    return this.transport.post<LineageEdge>(`${API_PREFIX}/lineage/direct`, body);
  }

  async batch(edges: Iterable<EdgeInput>): Promise<BatchLineageResult[]> {
    const body = Array.from(edges, normalize);
    return this.transport.post<BatchLineageResult[]>(`${API_PREFIX}/lineage/batch`, body);
  }
}

function normalize(e: EdgeInput): LineageEdge {
  if (Array.isArray(e)) {
    if (e.length === 2) return { source: e[0], target: e[1], type: DEFAULT_EDGE_TYPE };
    return { source: e[0], target: e[1], type: e[2] };
  }
  return { type: DEFAULT_EDGE_TYPE, ...e };
}
