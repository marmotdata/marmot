/** Workload-identity detectors. Each is silent when its env isn't present. */

import { TOKEN_TYPE_ID_TOKEN } from "../exchange.js";
import { GCPWorkloadIdentitySource } from "./gcp.js";
import { GitHubActionsSource } from "./github.js";
import { KubernetesServiceAccountSource } from "./kubernetes.js";

export interface SubjectToken {
  token: string;
  tokenType: string;
}

export interface WorkloadIdentitySource {
  name: string;
  fetch(): Promise<SubjectToken | null>;
}

export function makeSubjectToken(
  token: string,
  tokenType: string = TOKEN_TYPE_ID_TOKEN,
): SubjectToken {
  return { token, tokenType };
}

export function defaultSources(): WorkloadIdentitySource[] {
  return [
    new GitHubActionsSource(),
    new GCPWorkloadIdentitySource(),
    new KubernetesServiceAccountSource(),
  ];
}
