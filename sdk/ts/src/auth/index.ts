/**
 * Auth resolution for the Marmot SDK.
 *
 * The chain is non-interactive — no browser, no prompts. Order:
 *
 *   1. Explicit kwargs    apiKey, token
 *   2. Env vars           MARMOT_API_KEY, MARMOT_TOKEN
 *   3. Cached credentials ~/.config/marmot/credentials.json (written by `marmot login`)
 *   4. Workload identity  K8s SA / GCP metadata / GitHub Actions OIDC → RFC 8693 exchange
 */

import { loadCachedToken, loadContexts, resolveContext } from "../_config.js";
import { AuthError } from "../errors.js";
import { exchange } from "./exchange.js";
import { type WorkloadIdentitySource, defaultSources } from "./workload/index.js";

export type AuthScheme = "Bearer" | "X-API-Key";

export interface Credential {
  token: string;
  scheme: AuthScheme;
  expiresAt?: Date | undefined;
  refresh?: (() => Promise<Credential>) | undefined;
  source: string;
}

export interface ResolveOptions {
  baseUrl?: string | undefined;
  apiKey?: string | undefined;
  token?: string | undefined;
  context?: string | undefined;
  env?: Record<string, string | undefined> | undefined;
  workloadSources?: WorkloadIdentitySource[] | undefined;
}

export async function resolve(opts: ResolveOptions): Promise<{
  baseUrl: string;
  credential: Credential;
}> {
  const env = opts.env ?? processEnv();

  let cred = tryExplicit({ apiKey: opts.apiKey, token: opts.token }) ?? tryEnv(env);

  const { contexts, active } = await loadContexts();
  const selected = resolveContext({
    explicit: opts.context,
    contexts,
    active,
    env,
  });

  const resolvedUrl = opts.baseUrl ?? env.MARMOT_HOST ?? selected?.host ?? null;

  if (!cred && selected) {
    cred = await tryCachedToken(selected.name);
  }

  if (!cred && resolvedUrl) {
    cred = await tryWorkloadIdentity({
      baseUrl: resolvedUrl,
      sources: opts.workloadSources,
    });
  }

  if (!resolvedUrl) {
    throw new AuthError(
      "no Marmot host configured. Set MARMOT_HOST, pass baseUrl, or run `marmot login` first.",
    );
  }
  if (!cred) {
    throw new AuthError(
      "no Marmot credentials found. Set MARMOT_API_KEY / MARMOT_TOKEN, " +
        "run `marmot login`, or run inside K8s/GCP/GitHub Actions for workload identity.",
    );
  }

  return { baseUrl: resolvedUrl, credential: cred };
}

function tryExplicit(args: {
  apiKey?: string | undefined;
  token?: string | undefined;
}): Credential | null {
  if (args.apiKey) {
    return { token: args.apiKey, scheme: "X-API-Key", source: "explicit apiKey" };
  }
  if (args.token) {
    return { token: args.token, scheme: "Bearer", source: "explicit token" };
  }
  return null;
}

function tryEnv(env: Record<string, string | undefined>): Credential | null {
  if (env.MARMOT_API_KEY) {
    return { token: env.MARMOT_API_KEY, scheme: "X-API-Key", source: "env MARMOT_API_KEY" };
  }
  if (env.MARMOT_TOKEN) {
    return { token: env.MARMOT_TOKEN, scheme: "Bearer", source: "env MARMOT_TOKEN" };
  }
  return null;
}

async function tryCachedToken(contextName: string): Promise<Credential | null> {
  const cached = await loadCachedToken(contextName);
  if (!cached) return null;
  if (cached.expiresAt.getTime() - 30_000 <= Date.now()) return null;
  return {
    token: cached.accessToken,
    scheme: "Bearer",
    expiresAt: cached.expiresAt,
    source: `cached credential for context "${contextName}"`,
  };
}

async function tryWorkloadIdentity(args: {
  baseUrl: string;
  sources: WorkloadIdentitySource[] | undefined;
}): Promise<Credential | null> {
  for (const src of args.sources ?? defaultSources()) {
    const subjectToken = await src.fetch();
    if (!subjectToken) continue;
    return exchange({
      baseUrl: args.baseUrl,
      subjectToken: subjectToken.token,
      subjectTokenType: subjectToken.tokenType,
      sourceName: src.name,
    });
  }
  return null;
}

function processEnv(): Record<string, string | undefined> {
  return typeof process !== "undefined" && process.env ? process.env : {};
}
