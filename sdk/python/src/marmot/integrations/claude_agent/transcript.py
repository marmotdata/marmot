"""Summarise a Claude Code session transcript for telemetry.

Every Claude Agent SDK hook input carries a ``transcript_path`` pointing at
the session's JSONL log — the same file the underlying ``claude`` CLI writes.
At ``Stop`` time the tracker reads it once to extract real token totals and
timestamps, which hook callbacks alone cannot supply (``ResultMessage.usage``
only travels over the message stream, never through hooks).

The JSONL is the CLI's own log format, not part of the Agent SDK's API, so
there is no published type to import: :class:`TranscriptEntry` declares the
fields we consume and ignores the rest. Lines that don't fit are skipped, so a
changed field never blocks the run record from landing.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, ValidationError, field_validator


class Usage(BaseModel):
    """Per-turn token counts, in Anthropic's billing buckets."""

    model_config = ConfigDict(frozen=True)

    input_tokens: int = 0
    cache_creation_input_tokens: int = 0
    cache_read_input_tokens: int = 0
    output_tokens: int = 0

    @field_validator("*", mode="before")
    @classmethod
    def _missing_counts_as_zero(cls, value: Any) -> Any:
        return value or 0

    @property
    def total_in(self) -> int:
        """Cache writes and reads are priced apart, but both are input side."""

        return self.input_tokens + self.cache_creation_input_tokens + self.cache_read_input_tokens


class TranscriptMessage(BaseModel):
    model_config = ConfigDict(frozen=True)

    usage: Usage = Usage()


class TranscriptEntry(BaseModel):
    """One JSONL line, narrowed to what telemetry needs."""

    model_config = ConfigDict(frozen=True)

    type: str | None = None
    timestamp: datetime | None = None
    message: TranscriptMessage = TranscriptMessage()

    @field_validator("timestamp", "type", mode="before")
    @classmethod
    def _only_strings(cls, value: Any) -> Any:
        return value if isinstance(value, str) and value else None

    @property
    def is_assistant_turn(self) -> bool:
        return self.type == "assistant"


@dataclass(frozen=True)
class TranscriptSummary:
    """Token totals + wall-clock bounds for one session transcript."""

    tokens_in: int
    tokens_out: int
    started_at: datetime | None
    ended_at: datetime | None


def summarize_transcript(path: str | Path) -> TranscriptSummary | None:
    """Walk a session's JSONL transcript and aggregate per-turn usage.

    Returns ``None`` when the file is missing or unreadable. Returns a summary
    with zero counts when the file exists but holds no recognisable assistant
    turns — the caller still gets timestamps from any entry that carried one.
    """
    try:
        raw = Path(path).read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return None

    tokens_in = 0
    tokens_out = 0
    timestamps: list[datetime] = []

    for entry in _parse_entries(raw):
        if entry.timestamp is not None:
            timestamps.append(entry.timestamp)
        if entry.is_assistant_turn:
            tokens_in += entry.message.usage.total_in
            tokens_out += entry.message.usage.output_tokens

    return TranscriptSummary(
        tokens_in=tokens_in,
        tokens_out=tokens_out,
        started_at=min(timestamps, default=None),
        ended_at=max(timestamps, default=None),
    )


def _parse_entries(raw: str) -> list[TranscriptEntry]:
    entries = []
    for line in raw.splitlines():
        if not line.strip():
            continue
        try:
            entries.append(TranscriptEntry.model_validate_json(line))
        except ValidationError:
            continue
    return entries
