import type { Transport } from "../_http.js";
import type { APIKey, CreateAPIKeyRequest } from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface CreateAPIKeyArgs {
  name: string;
  /** Days until expiry. Omit for non-expiring keys. */
  expiresInDays?: number;
}

export class APIKeysResource {
  constructor(private readonly transport: Transport) {}

  async list(): Promise<APIKey[]> {
    return this.transport.get<APIKey[]>(`${API_PREFIX}/users/apikeys`);
  }

  async create(args: CreateAPIKeyArgs): Promise<APIKey> {
    const body: CreateAPIKeyRequest = { name: args.name };
    if (args.expiresInDays !== undefined && args.expiresInDays > 0) {
      body.expires_in_days = args.expiresInDays;
    }
    return this.transport.post<APIKey>(`${API_PREFIX}/users/apikeys`, body);
  }

  async delete(keyId: string): Promise<void> {
    await this.transport.delete(`${API_PREFIX}/users/apikeys/${keyId}`);
  }
}
