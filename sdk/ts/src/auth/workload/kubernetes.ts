/** Kubernetes service-account token source. */

import { TOKEN_TYPE_ID_TOKEN } from "../exchange.js";
import type { SubjectToken, WorkloadIdentitySource } from "./index.js";

export const DEFAULT_TOKEN_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/token";

export class KubernetesServiceAccountSource implements WorkloadIdentitySource {
  readonly name = "kubernetes";
  private readonly tokenPath: string;

  constructor(opts: { tokenPath?: string } = {}) {
    this.tokenPath = opts.tokenPath ?? DEFAULT_TOKEN_PATH;
  }

  async fetch(): Promise<SubjectToken | null> {
    try {
      const fs = await import("node:fs/promises");
      const raw = (await fs.readFile(this.tokenPath, "utf8")).trim();
      if (!raw) return null;
      return { token: raw, tokenType: TOKEN_TYPE_ID_TOKEN };
    } catch {
      return null;
    }
  }
}
