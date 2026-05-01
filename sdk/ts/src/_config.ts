/**
 * Read the same config files the CLI writes:
 *   ~/.config/marmot/config.yaml      contexts + current_context
 *   ~/.config/marmot/credentials.json cached OAuth tokens (per context)
 *
 * The SDK never writes to these files; the CLI owns them.
 *
 * Filesystem access is conditional — Edge runtimes (Cloudflare Workers,
 * Vercel Edge) have no fs module, so we dynamically import and degrade
 * gracefully when it isn't available.
 */

import { parse as parseYaml } from "yaml";

export interface Context {
  name: string;
  host: string;
}

export interface CachedToken {
  accessToken: string;
  tokenType: string;
  expiresAt: Date;
}

export function isExpired(token: CachedToken, leewaySeconds = 30): boolean {
  return token.expiresAt.getTime() - leewaySeconds * 1000 <= Date.now();
}

export async function configDir(): Promise<string | null> {
  const env = await safeEnv();
  if (!env) return null;

  const xdg = env.XDG_CONFIG_HOME;
  if (xdg) return joinPath(xdg, "marmot");

  const home = env.HOME ?? env.USERPROFILE;
  if (!home) return null;

  // Mirror Go's os.UserConfigDir() per platform.
  const platform = await safePlatform();
  if (platform === "darwin") {
    return joinPath(home, "Library", "Application Support", "marmot");
  }
  if (platform === "win32") {
    return joinPath(env.APPDATA ?? joinPath(home, "AppData", "Roaming"), "marmot");
  }
  return joinPath(home, ".config", "marmot");
}

export async function loadContexts(): Promise<{
  contexts: Map<string, Context>;
  active: string | null;
}> {
  const empty = { contexts: new Map<string, Context>(), active: null };
  const dir = await configDir();
  if (!dir) return empty;

  const text = await safeReadText(joinPath(dir, "config.yaml"));
  if (text === null) return empty;

  let raw: unknown;
  try {
    raw = parseYaml(text);
  } catch {
    return empty;
  }

  if (!isRecord(raw)) return empty;

  const contexts = new Map<string, Context>();
  const rawContexts = raw.contexts;
  if (isRecord(rawContexts)) {
    for (const [name, entry] of Object.entries(rawContexts)) {
      if (isRecord(entry) && typeof entry.host === "string" && entry.host) {
        contexts.set(name, { name, host: entry.host });
      }
    }
  }

  const active =
    typeof raw.current_context === "string" && raw.current_context ? raw.current_context : null;

  return { contexts, active };
}

export async function loadCachedToken(contextName: string): Promise<CachedToken | null> {
  const dir = await configDir();
  if (!dir) return null;

  const text = await safeReadText(joinPath(dir, "credentials.json"));
  if (text === null) return null;

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return null;
  }
  if (!isRecord(raw) || !isRecord(raw.tokens)) return null;

  const entry = raw.tokens[contextName];
  if (!isRecord(entry)) return null;

  const accessToken = entry.access_token;
  const tokenType = entry.token_type ?? "Bearer";
  const expiresRaw = entry.expires_at;
  if (typeof accessToken !== "string" || typeof expiresRaw !== "string") return null;

  const expiresAt = parseRFC3339(expiresRaw);
  if (!expiresAt) return null;

  return {
    accessToken,
    tokenType: typeof tokenType === "string" ? tokenType : "Bearer",
    expiresAt,
  };
}

export interface ResolveContextArgs {
  explicit?: string | undefined;
  contexts: Map<string, Context>;
  active: string | null;
  env?: Record<string, string | undefined> | undefined;
}

export function resolveContext(args: ResolveContextArgs): Context | null {
  const env = args.env ?? processEnv();
  const name = args.explicit || env.MARMOT_CONTEXT || args.active;
  if (!name) return null;
  return args.contexts.get(name) ?? null;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function joinPath(...parts: string[]): string {
  return parts.filter(Boolean).join("/").replace(/\/+/g, "/");
}

function processEnv(): Record<string, string | undefined> {
  return typeof process !== "undefined" && process.env ? process.env : {};
}

async function safeEnv(): Promise<Record<string, string | undefined> | null> {
  return processEnv();
}

async function safePlatform(): Promise<string | null> {
  if (typeof process !== "undefined" && process.platform) return process.platform;
  return null;
}

async function safeReadText(path: string): Promise<string | null> {
  try {
    const fs = await import("node:fs/promises");
    return await fs.readFile(path, "utf8");
  } catch {
    return null;
  }
}

function parseRFC3339(s: string): Date | null {
  // Date.parse handles RFC 3339 including offsets; just trim Go's nanosecond precision
  // which JS Dates can't represent.
  let trimmed = s;
  const dot = trimmed.indexOf(".");
  if (dot !== -1) {
    let i = dot + 1;
    while (i < trimmed.length && /\d/.test(trimmed[i] ?? "")) i++;
    const digits = trimmed.slice(dot + 1, Math.min(dot + 4, i));
    trimmed = trimmed.slice(0, dot + 1) + digits + trimmed.slice(i);
  }
  const ms = Date.parse(trimmed);
  if (Number.isNaN(ms)) return null;
  return new Date(ms);
}
