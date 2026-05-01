export { Client, type ClientOptions, connect, type ConnectOptions } from "./client.js";
export type { Credential, AuthScheme, ResolveOptions } from "./auth/index.js";
export type {
  WorkloadIdentitySource,
  SubjectToken,
} from "./auth/workload/index.js";
export { GitHubActionsSource } from "./auth/workload/github.js";
export { GCPWorkloadIdentitySource } from "./auth/workload/gcp.js";
export { KubernetesServiceAccountSource } from "./auth/workload/kubernetes.js";
export { AuthError, MarmotError, NotFoundError, ServerError } from "./errors.js";
export type { LineageEdge, LineageWriteArgs, EdgeInput } from "./resources/lineage.js";
export type { LookupArgs } from "./resources/assets.js";
export type { SearchOptions } from "./resources/search.js";

export const VERSION = "0.1.0";
