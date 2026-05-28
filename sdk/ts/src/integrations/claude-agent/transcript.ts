/**
 * Summarise a Claude Code session transcript for telemetry.
 *
 * Every Claude Agent SDK hook input carries a `transcript_path` pointing at
 * the session's JSONL log — the same file the underlying `claude` CLI writes.
 * At `Stop` time the tracker reads it once to extract real token totals and
 * timestamps, which hook callbacks alone cannot supply (`ResultMessage.usage`
 * only travels over the message stream, never through hooks).
 *
 * Schema is SDK-internal; everything here is best-effort and degrades to zero
 * counts on any parse failure so a missing/changed field never blocks the
 * run record from landing.
 */

import { readFile } from "node:fs/promises";

export interface TranscriptSummary {
  tokensIn: number;
  tokensOut: number;
  startedAt: Date | null;
  endedAt: Date | null;
}

/**
 * Walk a session's JSONL transcript and aggregate per-turn usage.
 *
 * Returns `null` when the file is missing or unreadable. Returns a summary
 * with zero counts when the file exists but contains no recognisable
 * assistant turns — the caller still gets timestamps if any entry had a
 * parseable `timestamp`.
 *
 * Token attribution follows Anthropic's billing buckets: `input_tokens` +
 * `cache_creation_input_tokens` + `cache_read_input_tokens` are all
 * input-side; `output_tokens` is output.
 */
export async function summarizeTranscript(path: string): Promise<TranscriptSummary | null> {
  let raw: string;
  try {
    raw = await readFile(path, "utf-8");
  } catch {
    return null;
  }

  let tokensIn = 0;
  let tokensOut = 0;
  let firstTs: Date | null = null;
  let lastTs: Date | null = null;

  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    let entry: unknown;
    try {
      entry = JSON.parse(trimmed);
    } catch {
      continue;
    }
    if (!isObject(entry)) continue;

    const ts = parseTs(entry.timestamp);
    if (ts) {
      if (!firstTs || ts < firstTs) firstTs = ts;
      if (!lastTs || ts > lastTs) lastTs = ts;
    }

    if (entry.type !== "assistant") continue;
    const message = entry.message;
    if (!isObject(message)) continue;
    const usage = message.usage;
    if (!isObject(usage)) continue;

    tokensIn += toInt(usage.input_tokens);
    tokensIn += toInt(usage.cache_creation_input_tokens);
    tokensIn += toInt(usage.cache_read_input_tokens);
    tokensOut += toInt(usage.output_tokens);
  }

  return { tokensIn, tokensOut, startedAt: firstTs, endedAt: lastTs };
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function parseTs(v: unknown): Date | null {
  if (typeof v !== "string" || !v) return null;
  const t = Date.parse(v);
  return Number.isFinite(t) ? new Date(t) : null;
}

function toInt(v: unknown): number {
  return typeof v === "number" && Number.isFinite(v) ? Math.trunc(v) : 0;
}
