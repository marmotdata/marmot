/**
 * Helpers used by more than one integration. Not part of the public surface.
 */

export function extractMrns(value: unknown): Set<string> {
  const out = new Set<string>();
  walkForMrns(value, out, 0);
  return out;
}

function walkForMrns(value: unknown, out: Set<string>, depth: number): void {
  if (depth > 5) return;
  if (value === null || value === undefined) return;
  if (typeof value === "string") {
    scanStringForMrns(value, out);
    return;
  }
  if (Array.isArray(value)) {
    for (const v of value) walkForMrns(v, out, depth + 1);
    return;
  }
  if (typeof value === "object") {
    const obj = value as Record<string, unknown>;
    const mrn = obj.mrn;
    if (typeof mrn === "string" && mrn) out.add(mrn);
    for (const v of Object.values(obj)) {
      walkForMrns(v, out, depth + 1);
    }
  }
}

// Match any URI-shaped substring. Used to pick MRNs out of free text — for
// example the markdown bodies returned by the Marmot MCP server.
const URI_RE = /\b([a-z][a-z0-9+.-]*):\/\/[^\s'"`)\]\}>,]+/gi;
const NON_MRN_SCHEMES = new Set(["http", "https", "ws", "wss", "ftp", "ftps", "file"]);

function scanStringForMrns(text: string, out: Set<string>): void {
  for (const match of text.matchAll(URI_RE)) {
    const scheme = (match[1] ?? "").toLowerCase();
    if (NON_MRN_SCHEMES.has(scheme)) continue;
    out.add(match[0]);
  }
}

export async function sha256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input);
  const buf = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, "0")).join("");
}
