import type { Transport } from "../_http.js";
import { API_PREFIX } from "./index.js";

export const DEFAULT_EDGE_TYPE = "DIRECT";

export interface LineageEdge {
  source: string;
  target: string;
  type?: string;
  job_mrn?: string;
}

export type EdgeInput = LineageEdge | [string, string] | [string, string, string];

export interface LineageWriteArgs {
  source: string;
  target: string;
  type?: string;
  jobMrn?: string;
}

export class LineageResource {
  constructor(private readonly transport: Transport) {}

  async write(args: LineageWriteArgs): Promise<Record<string, unknown>> {
    const body: LineageEdge = {
      source: args.source,
      target: args.target,
      type: args.type ?? DEFAULT_EDGE_TYPE,
    };
    if (args.jobMrn) body.job_mrn = args.jobMrn;
    return this.transport.post<Record<string, unknown>>(`${API_PREFIX}/lineage/direct`, body);
  }

  async batch(edges: Iterable<EdgeInput>): Promise<Record<string, unknown>[]> {
    const body = Array.from(edges, normalize);
    return this.transport.post<Record<string, unknown>[]>(`${API_PREFIX}/lineage/batch`, body);
  }

  async upstream(assetId: string, opts: { depth?: number } = {}): Promise<Record<string, unknown>> {
    const query: Record<string, unknown> = {};
    if (opts.depth !== undefined) query.depth = opts.depth;
    return this.transport.get<Record<string, unknown>>(
      `${API_PREFIX}/lineage/assets/${assetId}`,
      query,
    );
  }
}

function normalize(e: EdgeInput): LineageEdge {
  if (Array.isArray(e)) {
    if (e.length === 2) return { source: e[0], target: e[1], type: DEFAULT_EDGE_TYPE };
    return { source: e[0], target: e[1], type: e[2] };
  }
  return { type: DEFAULT_EDGE_TYPE, ...e };
}
