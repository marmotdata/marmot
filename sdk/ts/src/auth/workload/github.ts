/** GitHub Actions OIDC source. */

import { TOKEN_TYPE_ID_TOKEN } from "../exchange.js";
import type { SubjectToken, WorkloadIdentitySource } from "./index.js";

export class GitHubActionsSource implements WorkloadIdentitySource {
  readonly name = "github-actions";
  private readonly audience: string | undefined;
  private readonly timeoutMs: number;
  private readonly fetchImpl: typeof fetch;

  constructor(opts: { audience?: string; timeoutMs?: number; fetchImpl?: typeof fetch } = {}) {
    this.audience = opts.audience;
    this.timeoutMs = opts.timeoutMs ?? 5_000;
    this.fetchImpl = opts.fetchImpl ?? fetch;
  }

  async fetch(): Promise<SubjectToken | null> {
    const env = processEnv();
    const url = env.ACTIONS_ID_TOKEN_REQUEST_URL;
    const bearer = env.ACTIONS_ID_TOKEN_REQUEST_TOKEN;
    if (!url || !bearer) return null;

    const audience = this.audience ?? env.MARMOT_HOST;
    const requestUrl = new URL(url);
    if (audience) requestUrl.searchParams.set("audience", audience);

    const ac = new AbortController();
    const timer = setTimeout(() => ac.abort(), this.timeoutMs);
    try {
      const resp = await this.fetchImpl(requestUrl, {
        headers: { Authorization: `Bearer ${bearer}` },
        signal: ac.signal,
      });
      if (!resp.ok) return null;
      const json = (await resp.json()) as Record<string, unknown>;
      const value = json.value;
      if (typeof value !== "string" || !value) return null;
      return { token: value, tokenType: TOKEN_TYPE_ID_TOKEN };
    } catch {
      return null;
    } finally {
      clearTimeout(timer);
    }
  }
}

function processEnv(): Record<string, string | undefined> {
  return typeof process !== "undefined" && process.env ? process.env : {};
}
