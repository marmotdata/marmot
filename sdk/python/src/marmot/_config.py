"""Read the same config files the CLI writes.

CLI command `marmot login` writes:
  - ~/.config/marmot/config.yaml      contexts + current_context
  - ~/.config/marmot/credentials.json cached OAuth tokens (per context)

We never write to these files; the CLI owns them.
"""

from __future__ import annotations

import json
import os
import platform
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any

import yaml


@dataclass(frozen=True)
class Context:
    """A named server entry from config.yaml."""

    name: str
    host: str


@dataclass(frozen=True)
class CachedToken:
    """An OAuth token cached by `marmot login` for a given context."""

    access_token: str
    token_type: str
    expires_at: datetime

    def is_expired(self, *, leeway_seconds: int = 30) -> bool:
        from datetime import timedelta, timezone

        now = datetime.now(timezone.utc)
        return self.expires_at - timedelta(seconds=leeway_seconds) <= now


def config_dir() -> Path:
    """Mirror Go's os.UserConfigDir() + /marmot."""
    if base := os.environ.get("XDG_CONFIG_HOME"):
        return Path(base) / "marmot"

    system = platform.system()
    home = Path.home()
    if system == "Darwin":
        return home / "Library" / "Application Support" / "marmot"
    if system == "Windows":
        appdata = os.environ.get("APPDATA")
        if appdata:
            return Path(appdata) / "marmot"
        return home / "AppData" / "Roaming" / "marmot"
    # Linux and other Unix
    return home / ".config" / "marmot"


def config_path() -> Path:
    return config_dir() / "config.yaml"


def credentials_path() -> Path:
    return config_dir() / "credentials.json"


def load_contexts() -> tuple[dict[str, Context], str | None]:
    """Return (contexts_by_name, active_context_name) or ({}, None) if no config."""
    p = config_path()
    if not p.exists():
        return {}, None

    try:
        raw: Any = yaml.safe_load(p.read_text())
    except yaml.YAMLError:
        return {}, None

    if not isinstance(raw, dict):
        return {}, None

    contexts: dict[str, Context] = {}
    raw_contexts = raw.get("contexts") or {}
    if isinstance(raw_contexts, dict):
        for name, entry in raw_contexts.items():
            if isinstance(entry, dict):
                host = entry.get("host")
                if isinstance(host, str) and host:
                    contexts[name] = Context(name=name, host=host)

    active = raw.get("current_context")
    return contexts, active if isinstance(active, str) and active else None


def load_cached_token(context_name: str) -> CachedToken | None:
    """Return the cached OAuth token for a context, or None if absent/expired."""
    p = credentials_path()
    if not p.exists():
        return None

    try:
        raw = json.loads(p.read_text())
    except (json.JSONDecodeError, OSError):
        return None

    tokens = raw.get("tokens") if isinstance(raw, dict) else None
    if not isinstance(tokens, dict):
        return None

    entry = tokens.get(context_name)
    if not isinstance(entry, dict):
        return None

    access = entry.get("access_token")
    token_type = entry.get("token_type") or "Bearer"
    expires_raw = entry.get("expires_at")
    if not isinstance(access, str) or not isinstance(expires_raw, str):
        return None

    try:
        expires_at = _parse_rfc3339(expires_raw)
    except ValueError:
        return None

    return CachedToken(access_token=access, token_type=token_type, expires_at=expires_at)


def resolve_context(
    *,
    explicit: str | None = None,
    contexts: dict[str, Context] | None = None,
    active: str | None = None,
    env: dict[str, str] | None = None,
) -> Context | None:
    """Pick the context to use.

    Order: explicit kwarg > MARMOT_CONTEXT env > active context.
    """
    if contexts is None or active is None:
        contexts, active = load_contexts()
    if env is None:
        env = dict(os.environ)

    name = explicit or env.get("MARMOT_CONTEXT") or active
    if not name:
        return None
    return contexts.get(name)


def _parse_rfc3339(s: str) -> datetime:
    """Parse Go-style time.Time JSON strings (RFC 3339 with nanoseconds + Z or offset)."""
    # Trim sub-microsecond precision Python can't represent.
    if "." in s:
        head, _, tail = s.partition(".")
        # tail looks like "123456789Z" or "123456789+00:00"
        digits = ""
        rest = tail
        for i, ch in enumerate(tail):
            if ch.isdigit():
                digits = tail[: i + 1]
                rest = tail[i + 1 :]
            else:
                rest = tail[i:]
                break
        digits = digits[:6]
        s = f"{head}.{digits}{rest}"

    if s.endswith("Z"):
        s = s[:-1] + "+00:00"

    return datetime.fromisoformat(s)
