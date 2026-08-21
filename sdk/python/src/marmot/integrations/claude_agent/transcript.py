"""Summarise a Claude Code session transcript for telemetry.

Every Claude Agent SDK hook input carries a ``transcript_path`` pointing at
the session's JSONL log — the same file the underlying ``claude`` CLI writes.
At ``Stop`` time the tracker reads it once to extract real token totals and
timestamps, which hook callbacks alone cannot supply (``ResultMessage.usage``
only travels over the message stream, never through hooks).

Schema is SDK-internal; everything here is best-effort and degrades to zero
counts on any parse failure so a missing/changed field never blocks the run
record from landing.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass(frozen=True)
class TranscriptSummary:
    """Token totals + wall-clock bounds for one session transcript."""

    tokens_in: int
    tokens_out: int
    started_at: datetime | None
    ended_at: datetime | None


def summarize_transcript(path: str | Path) -> TranscriptSummary | None:
    """Walk a session's JSONL transcript and aggregate per-turn usage.

    Returns ``None`` when the file is missing or unreadable. Returns a
    summary with zero counts when the file exists but contains no
    recognisable assistant turns — the caller still gets timestamps if any
    entry had a parseable ``timestamp``.

    Token attribution follows Anthropic's billing buckets: ``input_tokens``
    + ``cache_creation_input_tokens`` + ``cache_read_input_tokens`` are all
    input-side (the latter two are priced separately by the API but bill as
    input from the user's perspective); ``output_tokens`` is output.
    """
    p = Path(path)
    try:
        raw = p.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return None

    tokens_in = 0
    tokens_out = 0
    first_ts: datetime | None = None
    last_ts: datetime | None = None

    for raw_line in raw.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(entry, dict):
            continue

        ts = _parse_ts(entry.get("timestamp"))
        if ts is not None:
            if first_ts is None or ts < first_ts:
                first_ts = ts
            if last_ts is None or ts > last_ts:
                last_ts = ts

        if entry.get("type") != "assistant":
            continue
        message = entry.get("message")
        if not isinstance(message, dict):
            continue
        usage = message.get("usage")
        if not isinstance(usage, dict):
            continue

        tokens_in += _int(usage.get("input_tokens"))
        tokens_in += _int(usage.get("cache_creation_input_tokens"))
        tokens_in += _int(usage.get("cache_read_input_tokens"))
        tokens_out += _int(usage.get("output_tokens"))

    return TranscriptSummary(
        tokens_in=tokens_in,
        tokens_out=tokens_out,
        started_at=first_ts,
        ended_at=last_ts,
    )


def _parse_ts(value: object) -> datetime | None:
    if not isinstance(value, str) or not value:
        return None
    normalised = value[:-1] + "+00:00" if value.endswith("Z") else value
    try:
        dt = datetime.fromisoformat(normalised)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def _int(value: object) -> int:
    if isinstance(value, bool):
        return 0
    if isinstance(value, int):
        return value
    return 0
