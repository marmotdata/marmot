export { Client, type ClientOptions, connect, type ConnectOptions } from "./client.js";
export { resolve } from "./auth/index.js";
export type { Credential, AuthScheme, ResolveOptions } from "./auth/index.js";
export type {
  WorkloadIdentitySource,
  SubjectToken,
} from "./auth/workload/index.js";
export { GitHubActionsSource } from "./auth/workload/github.js";
export { GCPWorkloadIdentitySource } from "./auth/workload/gcp.js";
export { KubernetesServiceAccountSource } from "./auth/workload/kubernetes.js";

export {
  AuthError,
  MarmotError,
  NotFoundError,
  RateLimitError,
  ServerError,
  ValidationError,
  isNotFound,
  isRateLimit,
} from "./errors.js";

export type * from "./_models.js";
export type { ListRunsResponse } from "./resources/runs.js";

export type { LookupArgs, SearchAssetsOptions } from "./resources/assets.js";
export type {
  EdgeInput,
  LineageGetOptions,
  LineageWriteArgs,
} from "./resources/lineage.js";
export type { SearchOptions } from "./resources/search.js";
export type {
  CreateTermArgs,
  GlossaryListOptions,
  GlossarySearchOptions,
  UpdateTermArgs,
} from "./resources/glossary.js";
export type { CreateAPIKeyArgs } from "./resources/api_keys.js";
export type { ListRunsOptions, ListEntitiesOptions } from "./resources/runs.js";
export type { ListUsersOptions } from "./resources/users.js";
export type { TopAssetsArgs } from "./resources/metrics.js";
export type { RecordAgentRunArgs, ToolCallInput } from "./resources/agent_runs.js";

export const VERSION = "0.2.0";
