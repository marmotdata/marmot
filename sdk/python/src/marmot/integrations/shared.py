"""Helpers used by more than one integration. Not part of the public surface."""

from __future__ import annotations

import hashlib
import json
import re
from typing import Any

# MRN schemes Marmot is known to emit. Conservative — matching ``<scheme>://``
# unconstrained would also catch arbitrary URLs (HTTP, etc.); we only mine for
# schemes we control or that land in the catalog.
MRN_PATTERN = re.compile(
    r"\b(?:mrn|postgres|postgresql|mysql|kafka|s3|gcs|bigquery|snowflake|redis|"
    r"clickhouse|elasticsearch|opensearch|mongodb|dynamodb|airflow|"
    r"dbt|marmot)://[^\s\"'`<>,)\]}]+"
)


def extract_mrns(output: Any) -> set[str]:
    """Walk an arbitrary tool output for asset MRNs.

    Recognises:
    - dicts with an ``mrn`` key,
    - dicts/lists containing such dicts (e.g. ``{"results": [{"mrn": ...}]}``),
    - JSON-encoded strings of any of the above,
    - free text mentioning ``<scheme>://...`` for known catalog schemes
      (markdown bodies returned by the Marmot MCP server fall here),
    - lists of content blocks shaped like ``{"type": "text", "text": ...}``
      (the MCP tool-response envelope).
    """
    found: set[str] = set()
    _walk(output, found, depth=0)
    return found


def _walk(value: Any, out: set[str], *, depth: int) -> None:
    if depth > 5:
        return
    if value is None:
        return
    if isinstance(value, str):
        _walk_string(value, out)
        return
    if isinstance(value, dict):
        mrn = value.get("mrn")
        if isinstance(mrn, str) and mrn:
            out.add(mrn)
        for v in value.values():
            _walk(v, out, depth=depth + 1)
        return
    if isinstance(value, list):
        for v in value:
            _walk(v, out, depth=depth + 1)


def _walk_string(s: str, out: set[str]) -> None:
    """Try to JSON-decode first (most structured tool outputs come back as
    JSON-encoded strings); fall back to regex scanning the raw text."""
    try:
        parsed = json.loads(s)
    except (ValueError, TypeError):
        parsed = None
    if isinstance(parsed, (dict, list)):
        _walk(parsed, out, depth=0)
    for match in MRN_PATTERN.findall(s):
        out.add(match)


def sha256_hex(value: str) -> str:
    return hashlib.sha256(value.encode()).hexdigest()
