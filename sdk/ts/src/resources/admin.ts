import type { Transport } from "../_http.js";
import type { ReindexAcceptedResponse, ReindexStatusResponse } from "../_models.js";
import { API_PREFIX } from "./index.js";

export class AdminResource {
  constructor(private readonly transport: Transport) {}

  async reindex(): Promise<ReindexAcceptedResponse> {
    return this.transport.post<ReindexAcceptedResponse>(`${API_PREFIX}/admin/search/reindex`);
  }

  async reindexStatus(): Promise<ReindexStatusResponse> {
    return this.transport.get<ReindexStatusResponse>(`${API_PREFIX}/admin/search/reindex`);
  }
}
