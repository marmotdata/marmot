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
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator

ENVIRONMENT_MARMOT_CONTEXT = "MARMOT_CONTEXT"
ENVIRONMENT_HOST = "MARMOT_HOST"


class Context(BaseModel):
    """A named server entry from config.yaml."""

    model_config = ConfigDict(frozen=True)

    name: str
    host: str = Field(min_length=1)


class MarmotConfig(BaseModel):
    """The contents of config.yaml, as written by `marmot login`."""

    model_config = ConfigDict(frozen=True)

    current_context: str | None = None
    contexts: dict[str, Context] = Field(default_factory=dict)


class CachedToken(BaseModel):
    """An OAuth token cached by `marmot login` for a given context.

    Fields are aliased to the credentials.json spelling. Go writes timestamps with
    nanosecond precision, which pydantic truncates to what ``datetime`` can hold.
    """

    model_config = ConfigDict(frozen=True, populate_by_name=True)

    token: str = Field(min_length=1, alias="access_token")
    token_scheme: str = Field(default="Bearer", alias="token_type")
    expires_at: datetime

    @field_validator("token_scheme", mode="before")
    @classmethod
    def _default_scheme(cls, value: Any) -> Any:
        return value or "Bearer"

    def is_expired(self, *, leeway_seconds: int = 30) -> bool:
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


def load_config() -> MarmotConfig:
    """Read config.yaml, keeping whichever entries are usable.

    The CLI owns this file, so one malformed context must not lock the SDK out.
    """
    path = config_path()
    if not path.exists():
        return MarmotConfig()

    try:
        raw: Any = yaml.safe_load(path.read_text())
    except (yaml.YAMLError, OSError):
        return MarmotConfig()
    if not isinstance(raw, dict):
        return MarmotConfig()

    contexts = {}
    for name, entry in (raw.get("contexts") or {}).items():
        if not isinstance(entry, dict):
            continue
        try:
            contexts[name] = Context(name=name, **entry)
        except ValidationError:
            continue

    active = raw.get("current_context")
    return MarmotConfig(
        contexts=contexts, current_context=active if isinstance(active, str) and active else None
    )


def load_cached_token(context_name: str) -> CachedToken | None:
    """Return the cached OAuth token for a context, or None if there is no usable one."""
    p = credentials_path()
    if not p.exists():
        return None

    try:
        raw = json.loads(p.read_text())
    except (json.JSONDecodeError, OSError):
        return None

    tokens = raw.get("tokens") if isinstance(raw, dict) else None
    entry = tokens.get(context_name) if isinstance(tokens, dict) else None
    if not isinstance(entry, dict):
        return None

    try:
        return CachedToken.model_validate(entry)
    except ValidationError:
        return None


def resolve_context(
    context_name: str | None = None, config: MarmotConfig | None = None
) -> Context | None:
    """Pick the context to use.

    Order: explicit kwarg > env var > active context.
    """
    if config is None:
        config = load_config()

    name = context_name or os.environ.get(ENVIRONMENT_MARMOT_CONTEXT) or config.current_context
    if not name:
        return None
    return config.contexts.get(name)
