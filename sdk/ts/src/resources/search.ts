import type { Transport } from "../_http.js";
import type { SearchResponse } from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface SearchOptions {
  types?: string[];
  providers?: string[];
  limit?: number;
  offset?: number;
}

export class SearchResource {
  constructor(private readonly transport: Transport) {}

  async query(query: string, opts: SearchOptions = {}): Promise<SearchResponse> {
    const params: Record<string, unknown> = { query };
    if (opts.types) params.asset_types = opts.types;
    if (opts.providers) params.providers = opts.providers;
    if (opts.limit !== undefined) params.limit = opts.limit;
    if (opts.offset !== undefined) params.offset = opts.offset;
    return this.transport.get<SearchResponse>(`${API_PREFIX}/search`, params);
  }
}
