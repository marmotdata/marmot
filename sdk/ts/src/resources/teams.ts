import type { Transport } from "../_http.js";
import type { ListMembersResponse, ListTeamsResponse, Team } from "../_models.js";
import { API_PREFIX } from "./index.js";

export class TeamsResource {
  constructor(private readonly transport: Transport) {}

  async list(opts: { limit?: number; offset?: number } = {}): Promise<ListTeamsResponse> {
    const query: Record<string, unknown> = {};
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<ListTeamsResponse>(`${API_PREFIX}/teams`, query);
  }

  async get(teamId: string): Promise<Team> {
    return this.transport.get<Team>(`${API_PREFIX}/teams/${teamId}`);
  }

  async members(teamId: string): Promise<ListMembersResponse> {
    return this.transport.get<ListMembersResponse>(`${API_PREFIX}/teams/${teamId}/members`);
  }
}
