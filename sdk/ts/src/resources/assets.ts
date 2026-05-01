import type { Transport } from "../_http.js";
import { NotFoundError } from "../errors.js";
import { API_PREFIX } from "./index.js";

export interface LookupArgs {
  type: string;
  service: string;
  name: string;
}

export class AssetsResource {
  constructor(private readonly transport: Transport) {}

  async get(id: string): Promise<Record<string, unknown>> {
    return this.transport.get<Record<string, unknown>>(`${API_PREFIX}/assets/${id}`);
  }

  async lookup(args: LookupArgs): Promise<Record<string, unknown>> {
    return this.transport.get<Record<string, unknown>>(
      `${API_PREFIX}/assets/lookup/${args.type}/${args.service}/${args.name}`,
    );
  }

  async find(args: LookupArgs): Promise<Record<string, unknown> | null> {
    try {
      return await this.lookup(args);
    } catch (e) {
      if (e instanceof NotFoundError) return null;
      throw e;
    }
  }

  async create(asset: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.transport.post<Record<string, unknown>>(`${API_PREFIX}/assets`, asset);
  }

  async update(id: string, asset: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.transport.put<Record<string, unknown>>(`${API_PREFIX}/assets/${id}`, asset);
  }

  async delete(id: string): Promise<void> {
    await this.transport.delete(`${API_PREFIX}/assets/${id}`);
  }
}
