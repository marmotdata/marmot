import type { Transport } from "../_http.js";
import type {
  Asset,
  AssetSearchResponse,
  AssetSummaryResponse,
  CreateAssetRequest,
  UpdateAssetRequest,
} from "../_models.js";
import { NotFoundError } from "../errors.js";
import { API_PREFIX } from "./index.js";

export interface LookupArgs {
  type: string;
  service: string;
  name: string;
}

export interface SearchAssetsOptions {
  query?: string;
  types?: string[];
  providers?: string[];
  tags?: string[];
  limit?: number;
  offset?: number;
}

export class AssetsResource {
  constructor(private readonly transport: Transport) {}

  async get(id: string): Promise<Asset> {
    return this.transport.get<Asset>(`${API_PREFIX}/assets/${id}`);
  }

  async lookup(args: LookupArgs): Promise<Asset> {
    return this.transport.get<Asset>(
      `${API_PREFIX}/assets/lookup/${args.type}/${args.service}/${args.name}`,
    );
  }

  async find(args: LookupArgs): Promise<Asset | null> {
    try {
      return await this.lookup(args);
    } catch (e) {
      if (e instanceof NotFoundError) return null;
      throw e;
    }
  }

  async search(opts: SearchAssetsOptions = {}): Promise<AssetSearchResponse> {
    const query: Record<string, unknown> = {};
    if (opts.query !== undefined) query.q = opts.query;
    if (opts.types?.length) query.types = opts.types;
    if (opts.providers?.length) query.services = opts.providers;
    if (opts.tags?.length) query.tags = opts.tags;
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<AssetSearchResponse>(`${API_PREFIX}/assets/search`, query);
  }

  async summary(): Promise<AssetSummaryResponse> {
    return this.transport.get<AssetSummaryResponse>(`${API_PREFIX}/assets/summary`);
  }

  async create(asset: CreateAssetRequest | Record<string, unknown>): Promise<Asset> {
    return this.transport.post<Asset>(`${API_PREFIX}/assets`, asset);
  }

  async update(id: string, asset: UpdateAssetRequest | Record<string, unknown>): Promise<Asset> {
    return this.transport.put<Asset>(`${API_PREFIX}/assets/${id}`, asset);
  }

  async delete(id: string): Promise<void> {
    await this.transport.delete(`${API_PREFIX}/assets/${id}`);
  }

  async addTag(id: string, tag: string): Promise<Asset> {
    return this.transport.post<Asset>(`${API_PREFIX}/assets/tags/${id}`, { tag });
  }

  async removeTag(id: string, tag: string): Promise<Asset> {
    return this.transport.delete<Asset>(`${API_PREFIX}/assets/tags/${id}`, { tag });
  }
}
