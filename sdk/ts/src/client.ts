import { Transport } from "./_http.js";
import type { SearchResponse } from "./_models.js";
import { type Credential, resolve } from "./auth/index.js";
import type { WorkloadIdentitySource } from "./auth/workload/index.js";
import { AdminResource } from "./resources/admin.js";
import { AgentRunsResource } from "./resources/agent_runs.js";
import { APIKeysResource } from "./resources/api_keys.js";
import { AssetsResource } from "./resources/assets.js";
import { GlossaryResource } from "./resources/glossary.js";
import { LineageResource } from "./resources/lineage.js";
import { MetricsResource } from "./resources/metrics.js";
import { OwnersResource } from "./resources/owners.js";
import { DataProductsResource } from "./resources/products.js";
import { RunsResource } from "./resources/runs.js";
import { type SearchOptions, SearchResource } from "./resources/search.js";
import { TagsResource } from "./resources/tags.js";
import { TeamsResource } from "./resources/teams.js";
import { UsersResource } from "./resources/users.js";

export interface ClientOptions {
  baseUrl: string;
  credential: Credential;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export class Client {
  readonly admin: AdminResource;
  readonly agentRuns: AgentRunsResource;
  readonly apiKeys: APIKeysResource;
  readonly assets: AssetsResource;
  readonly dataProducts: DataProductsResource;
  readonly glossary: GlossaryResource;
  readonly lineage: LineageResource;
  readonly metrics: MetricsResource;
  readonly owners: OwnersResource;
  readonly runs: RunsResource;
  readonly tags: TagsResource;
  readonly teams: TeamsResource;
  readonly users: UsersResource;
  private readonly searchResource: SearchResource;
  private readonly transport: Transport;

  constructor(opts: ClientOptions) {
    this.transport = new Transport({
      baseUrl: opts.baseUrl,
      credential: opts.credential,
      ...(opts.fetchImpl ? { fetchImpl: opts.fetchImpl } : {}),
      ...(opts.timeoutMs !== undefined ? { timeoutMs: opts.timeoutMs } : {}),
    });
    this.admin = new AdminResource(this.transport);
    this.agentRuns = new AgentRunsResource(this.transport);
    this.apiKeys = new APIKeysResource(this.transport);
    this.assets = new AssetsResource(this.transport);
    this.dataProducts = new DataProductsResource(this.transport);
    this.glossary = new GlossaryResource(this.transport);
    this.lineage = new LineageResource(this.transport);
    this.metrics = new MetricsResource(this.transport);
    this.owners = new OwnersResource(this.transport);
    this.runs = new RunsResource(this.transport);
    this.tags = new TagsResource(this.transport);
    this.teams = new TeamsResource(this.transport);
    this.users = new UsersResource(this.transport);
    this.searchResource = new SearchResource(this.transport);
  }

  get baseUrl(): string {
    return this.transport.baseUrl;
  }

  search(query: string, opts: SearchOptions = {}): Promise<SearchResponse> {
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
