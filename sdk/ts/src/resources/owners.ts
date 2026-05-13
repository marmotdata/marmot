import type { Transport } from "../_http.js";
import type { SearchOwnersResponse } from "../_models.js";
import { API_PREFIX } from "./index.js";

export class OwnersResource {
  constructor(private readonly transport: Transport) {}

  async search(query: string, opts: { limit?: number } = {}): Promise<SearchOwnersResponse> {
    const params: Record<string, unknown> = { q: query };
    if (opts.limit !== undefined) params.limit = opts.limit;
    return this.transport.get<SearchOwnersResponse>(`${API_PREFIX}/owners/search`, params);
  }
}
