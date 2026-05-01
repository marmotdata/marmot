/** HTTP transport with auth injection and refresh-on-401. */

import type { Credential } from "./auth/index.js";
import { AuthError, MarmotError, NotFoundError, ServerError } from "./errors.js";

export interface TransportOptions {
  baseUrl: string;
  credential: Credential;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export class Transport {
  private readonly baseUrl_: string;
  private credential: Credential;
  private readonly fetchImpl: typeof fetch;
  private readonly timeoutMs: number;

  constructor(opts: TransportOptions) {
    this.baseUrl_ = opts.baseUrl.replace(/\/$/, "");
    this.credential = opts.credential;
    this.fetchImpl = opts.fetchImpl ?? fetch;
    this.timeoutMs = opts.timeoutMs ?? 30_000;
  }

  get baseUrl(): string {
    return this.baseUrl_;
  }

  async request<T = unknown>(
    method: string,
    path: string,
    opts: { json?: unknown; query?: Record<string, unknown> | undefined } = {},
    retried = false,
  ): Promise<T> {
    const url = this.url(path, opts.query);
    const ac = new AbortController();
    const timer = setTimeout(() => ac.abort(), this.timeoutMs);

    let resp: Response;
    try {
      resp = await this.fetchImpl(url, {
        method,
        headers: this.headers(opts.json !== undefined),
        body: opts.json !== undefined ? JSON.stringify(opts.json) : null,
        signal: ac.signal,
      });
    } finally {
      clearTimeout(timer);
    }

    if (resp.status === 401 && !retried && this.credential.refresh) {
      try {
        this.credential = await this.credential.refresh();
      } catch (e) {
        if (e instanceof MarmotError) throw e;
        throw new AuthError(`credential refresh failed: ${(e as Error).message}`);
      }
      return this.request<T>(method, path, opts, true);
    }

    return this.parse<T>(resp);
  }

  get<T = unknown>(path: string, query?: Record<string, unknown>): Promise<T> {
    return this.request<T>("GET", path, { query });
  }

  post<T = unknown>(path: string, body?: unknown, query?: Record<string, unknown>): Promise<T> {
    return this.request<T>("POST", path, { json: body, query });
  }

  put<T = unknown>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("PUT", path, { json: body });
  }

  delete<T = unknown>(path: string): Promise<T> {
    return this.request<T>("DELETE", path);
  }

  private url(path: string, query?: Record<string, unknown>): string {
    const base = path.startsWith("/") ? `${this.baseUrl_}${path}` : `${this.baseUrl_}/${path}`;
    if (!query) return base;
    const url = new URL(base);
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null) continue;
      if (Array.isArray(v)) {
        for (const item of v) url.searchParams.append(k, String(item));
      } else {
        url.searchParams.set(k, String(v));
      }
    }
    return url.toString();
  }

  private headers(hasBody: boolean): Record<string, string> {
    const h: Record<string, string> = {};
    if (this.credential.scheme === "X-API-Key") {
      h["X-API-Key"] = this.credential.token;
    } else {
      h.Authorization = `Bearer ${this.credential.token}`;
    }
    if (hasBody) h["Content-Type"] = "application/json";
    return h;
  }

  private async parse<T>(resp: Response): Promise<T> {
    if (resp.status >= 200 && resp.status < 300) {
      const text = await resp.text();
      if (!text) return undefined as T;
      try {
        return JSON.parse(text) as T;
      } catch {
        throw new ServerError(`non-JSON response from ${resp.url}`);
      }
    }
    const msg = await errorMessage(resp);
    if (resp.status === 401) throw new AuthError(msg);
    if (resp.status === 403) throw new AuthError(msg);
    if (resp.status === 404) throw new NotFoundError(msg);
    if (resp.status >= 500) throw new ServerError(msg, resp.status);
    throw new MarmotError(msg);
  }
}

async function errorMessage(resp: Response): Promise<string> {
  try {
    const body = (await resp.json()) as Record<string, unknown>;
    for (const key of ["error_description", "error", "message"]) {
      const v = body[key];
      if (typeof v === "string" && v) return v;
    }
  } catch {
    // fallthrough
  }
  try {
    return (await resp.text()) || `HTTP ${resp.status}`;
  } catch {
    return `HTTP ${resp.status}`;
  }
}
