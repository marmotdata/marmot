import type { paths } from "../_gen/schema.js";
import type { Transport } from "../_http.js";
import type { PluginRun, RunEntitiesResponse } from "../_models.js";
import { API_PREFIX } from "./index.js";

/** Response shape for `GET /runs`. Path-derived because the schema doesn't name it. */
export type ListRunsResponse =
  paths["/runs"]["get"]["responses"][200]["content"]["application/json"];

export interface ListRunsOptions {
  /** Comma-separated list of pipeline names. */
  pipelines?: string;
  /** Comma-separated list of statuses. */
  statuses?: string;
  limit?: number;
  offset?: number;
}

export interface ListEntitiesOptions {
  entityType?: string;
  status?: string;
  limit?: number;
  offset?: number;
}

export class RunsResource {
  constructor(private readonly transport: Transport) {}

  async list(opts: ListRunsOptions = {}): Promise<ListRunsResponse> {
    const query: Record<string, unknown> = {};
    if (opts.pipelines !== undefined) query.pipelines = opts.pipelines;
    if (opts.statuses !== undefined) query.statuses = opts.statuses;
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<ListRunsResponse>(`${API_PREFIX}/runs`, query);
  }

  async get(runId: string): Promise<PluginRun> {
    return this.transport.get<PluginRun>(`${API_PREFIX}/runs/${runId}`);
  }

  async entities(runId: string, opts: ListEntitiesOptions = {}): Promise<RunEntitiesResponse> {
    const query: Record<string, unknown> = {};
    if (opts.entityType !== undefined) query.entity_type = opts.entityType;
    if (opts.status !== undefined) query.status = opts.status;
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<RunEntitiesResponse>(`${API_PREFIX}/runs/${runId}/entities`, query);
  }
}
