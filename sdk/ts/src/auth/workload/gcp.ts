/** GCP workload identity source via the metadata server. */

import { TOKEN_TYPE_ID_TOKEN } from "../exchange.js";
import type { SubjectToken, WorkloadIdentitySource } from "./index.js";

export const METADATA_HOST = "metadata.google.internal";
export const IDENTITY_PATH = "/computeMetadata/v1/instance/service-accounts/default/identity";

export class GCPWorkloadIdentitySource implements WorkloadIdentitySource {
  readonly name = "gcp";
  private readonly audience: string | undefined;
  private readonly timeoutMs: number;
  private readonly fetchImpl: typeof fetch;

  constructor(opts: { audience?: string; timeoutMs?: number; fetchImpl?: typeof fetch } = {}) {
    this.audience = opts.audience;
    this.timeoutMs = opts.timeoutMs ?? 2_000;
    this.fetchImpl = opts.fetchImpl ?? fetch;
  }

  async fetch(): Promise<SubjectToken | null> {
    if (!looksLikeGCP()) return null;

    const audience = this.audience ?? processEnv().MARMOT_HOST;
    if (!audience) return null;

    const url = new URL(`http://${METADATA_HOST}${IDENTITY_PATH}`);
    url.searchParams.set("audience", audience);
    url.searchParams.set("format", "full");

    const ac = new AbortController();
    const timer = setTimeout(() => ac.abort(), this.timeoutMs);
    try {
      const resp = await this.fetchImpl(url, {
        headers: { "Metadata-Flavor": "Google" },
        signal: ac.signal,
      });
      if (!resp.ok) return null;
      const text = (await resp.text()).trim();
      if (!text) return null;
      return { token: text, tokenType: TOKEN_TYPE_ID_TOKEN };
    } catch {
      return null;
    } finally {
      clearTimeout(timer);
    }
  }
}

function looksLikeGCP(): boolean {
  const env = processEnv();
  return Boolean(
    env.GOOGLE_CLOUD_PROJECT || env.GCLOUD_PROJECT || env.K_SERVICE || env.FUNCTION_TARGET,
  );
}

function processEnv(): Record<string, string | undefined> {
  return typeof process !== "undefined" && process.env ? process.env : {};
}
