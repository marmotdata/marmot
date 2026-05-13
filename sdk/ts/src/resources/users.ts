import type { Transport } from "../_http.js";
import type { ListUsersResponse, User } from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface ListUsersOptions {
  query?: string;
  active?: boolean;
  roleIds?: string[];
  limit?: number;
  offset?: number;
}

export class UsersResource {
  constructor(private readonly transport: Transport) {}

  async list(opts: ListUsersOptions = {}): Promise<ListUsersResponse> {
    const query: Record<string, unknown> = {};
    if (opts.query !== undefined) query.query = opts.query;
    if (opts.active !== undefined) query.active = opts.active;
    if (opts.roleIds?.length) query.role_ids = opts.roleIds;
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<ListUsersResponse>(`${API_PREFIX}/users`, query);
  }

  async get(userId: string): Promise<User> {
    return this.transport.get<User>(`${API_PREFIX}/users/${userId}`);
  }

  async me(): Promise<User> {
    return this.transport.get<User>(`${API_PREFIX}/users/me`);
  }
}
