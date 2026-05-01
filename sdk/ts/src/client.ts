import { Transport } from "./_http.js";
import { type Credential, resolve } from "./auth/index.js";
import type { WorkloadIdentitySource } from "./auth/workload/index.js";
import { AssetsResource } from "./resources/assets.js";
import { LineageResource } from "./resources/lineage.js";
import { type SearchOptions, SearchResource } from "./resources/search.js";

export interface ClientOptions {
  baseUrl: string;
  credential: Credential;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export class Client {
  readonly assets: AssetsResource;
  readonly lineage: LineageResource;
  private readonly searchResource: SearchResource;
  private readonly transport: Transport;

  constructor(opts: ClientOptions) {
    this.transport = new Transport({
      baseUrl: opts.baseUrl,
      credential: opts.credential,
      ...(opts.fetchImpl ? { fetchImpl: opts.fetchImpl } : {}),
      ...(opts.timeoutMs !== undefined ? { timeoutMs: opts.timeoutMs } : {}),
    });
    this.assets = new AssetsResource(this.transport);
    this.lineage = new LineageResource(this.transport);
    this.searchResource = new SearchResource(this.transport);
  }

  get baseUrl(): string {
    return this.transport.baseUrl;
  }

  search(query: string, opts: SearchOptions = {}): Promise<Record<string, unknown>> {
    return this.searchResource.query(query, opts);
  }
}

export interface ConnectOptions {
  baseUrl?: string;
  token?: string;
  apiKey?: string;
  context?: string;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
  workloadSources?: WorkloadIdentitySource[];
}

export async function connect(opts: ConnectOptions = {}): Promise<Client> {
  const { baseUrl, credential } = await resolve({
    baseUrl: opts.baseUrl,
    apiKey: opts.apiKey,
    token: opts.token,
    context: opts.context,
    workloadSources: opts.workloadSources,
  });
  return new Client({
    baseUrl,
    credential,
    ...(opts.fetchImpl ? { fetchImpl: opts.fetchImpl } : {}),
    ...(opts.timeoutMs !== undefined ? { timeoutMs: opts.timeoutMs } : {}),
  });
}
