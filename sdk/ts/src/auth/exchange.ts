/** RFC 8693 token exchange against Marmot's /oauth/token endpoint. */

import { AuthError, ServerError } from "../errors.js";
import type { Credential } from "./index.js";

export const GRANT_TYPE = "urn:ietf:params:oauth:grant-type:token-exchange";
export const TOKEN_TYPE_ID_TOKEN = "urn:ietf:params:oauth:token-type:id_token";
export const TOKEN_TYPE_ACCESS_TOKEN = "urn:ietf:params:oauth:token-type:access_token";

export interface ExchangeArgs {
  baseUrl: string;
  subjectToken: string;
  subjectTokenType: string;
  sourceName: string;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export async function exchange(args: ExchangeArgs): Promise<Credential> {
  const doExchange = async (): Promise<Credential> => {
    const url = `${args.baseUrl.replace(/\/$/, "")}/oauth/token`;
    const body = new URLSearchParams({
      grant_type: GRANT_TYPE,
      subject_token: args.subjectToken,
      subject_token_type: args.subjectTokenType,
    });

    const ac = new AbortController();
    const timer = setTimeout(() => ac.abort(), args.timeoutMs ?? 10_000);

    let resp: Response;
    try {
      resp = await (args.fetchImpl ?? fetch)(url, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
        signal: ac.signal,
      });
    } finally {
      clearTimeout(timer);
    }

    if (resp.status === 400) throw new AuthError(await oauthErrorMessage(resp));
    if (resp.status === 401) {
      throw new AuthError(
        `server rejected workload-identity token from ${args.sourceName}: ${await oauthErrorMessage(resp)}`,
      );
    }
    if (resp.status >= 500) {
      throw new ServerError(`token exchange failed: HTTP ${resp.status}`, resp.status);
    }
    if (resp.status !== 200) {
      throw new ServerError(
        `unexpected response from /oauth/token: HTTP ${resp.status}`,
        resp.status,
      );
    }

    const json = (await resp.json()) as Record<string, unknown>;
    const accessToken = json.access_token;
    if (typeof accessToken !== "string" || !accessToken) {
      throw new ServerError("token exchange response missing access_token");
    }

    const expiresIn = typeof json.expires_in === "number" ? json.expires_in : 0;
    const expiresAt = expiresIn > 0 ? new Date(Date.now() + expiresIn * 1000) : undefined;

    return {
      token: accessToken,
      scheme: "Bearer",
      expiresAt,
      refresh: doExchange,
      source: `token exchange via ${args.sourceName}`,
    };
  };

  return doExchange();
}

async function oauthErrorMessage(resp: Response): Promise<string> {
  try {
    const body = (await resp.json()) as Record<string, unknown>;
    const err = typeof body.error === "string" ? body.error : "";
    const desc = typeof body.error_description === "string" ? body.error_description : "";
    if (err && desc) return `${err}: ${desc}`;
    return err || desc || `HTTP ${resp.status}`;
  } catch {
    try {
      return (await resp.text()) || `HTTP ${resp.status}`;
    } catch {
      return `HTTP ${resp.status}`;
    }
  }
}
