import type { Transport } from "../_http.js";
import type {
  AssetCount,
  AssetsByProviderResponse,
  AssetsByTypeResponse,
  QueryCount,
  TotalAssetsResponse,
} from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface TopAssetsArgs {
  /** RFC3339 timestamp (inclusive). */
  start: string;
  /** RFC3339 timestamp (inclusive). */
  end: string;
  limit?: number;
}

export class MetricsResource {
  constructor(private readonly transport: Transport) {}

  async totalAssets(): Promise<TotalAssetsResponse> {
    return this.transport.get<TotalAssetsResponse>(`${API_PREFIX}/metrics/assets/total`);
  }

  async assetsByType(): Promise<AssetsByTypeResponse> {
    return this.transport.get<AssetsByTypeResponse>(`${API_PREFIX}/metrics/assets/by-type`);
  }

  async assetsByProvider(): Promise<AssetsByProviderResponse> {
    return this.transport.get<AssetsByProviderResponse>(`${API_PREFIX}/metrics/assets/by-provider`);
  }

  async topAssets(args: TopAssetsArgs): Promise<AssetCount[]> {
    const query: Record<string, unknown> = { start: args.start, end: args.end };
    if (args.limit !== undefined) query.limit = args.limit;
    return this.transport.get<AssetCount[]>(`${API_PREFIX}/metrics/top-assets`, query);
  }

  async topQueries(args: TopAssetsArgs): Promise<QueryCount[]> {
    const query: Record<string, unknown> = { start: args.start, end: args.end };
    if (args.limit !== undefined) query.limit = args.limit;
    return this.transport.get<QueryCount[]>(`${API_PREFIX}/metrics/top-queries`, query);
  }
}
