"""Config file parsing — must read what the CLI writes."""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from marmot.config import (
    ENVIRONMENT_MARMOT_CONTEXT,
    MarmotConfig,
    config_dir,
    load_cached_token,
    load_config,
    resolve_context,
)


@pytest.fixture
def isolated_config(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> Path:
    """Point XDG_CONFIG_HOME at a temp dir so tests never touch real config."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    cfg = tmp_path / "marmot"
    cfg.mkdir()
    return cfg


def test_load_config_empty(isolated_config: Path) -> None:
    config = load_config()
    assert config.contexts == {}
    assert config.current_context is None


def test_load_config_parses_yaml(isolated_config: Path) -> None:
    (isolated_config / "config.yaml").write_text(
        "current_context: prod\n"
        "contexts:\n"
        "  prod:\n"
        "    host: https://marmot.io \n"  # trailing space must be tolerated
        "  staging:\n"
        "    host: https://staging.marmot.io\n"
    )
    config = load_config()
    contexts, active = config.contexts, config.current_context
    assert active == "prod"
    assert set(contexts) == {"prod", "staging"}
    assert contexts["prod"].host == "https://marmot.io"


def test_load_config_skips_invalid_entries(isolated_config: Path) -> None:
    (isolated_config / "config.yaml").write_text(
        """\
current_context: ok
contexts:
  ok:
    host: https://valid.example.com
  no-host: {}
  not-a-mapping: "string-value"
"""
    )
    assert set(load_config().contexts) == {"ok"}


def test_load_cached_token_missing_returns_none(isolated_config: Path) -> None:
    assert load_cached_token("any") is None


def test_load_cached_token_parses_go_rfc3339(isolated_config: Path) -> None:
    in_one_hour = datetime.now(timezone.utc) + timedelta(hours=1)
    # Go's time.Time JSON output: RFC 3339 with fractional seconds and explicit tz
    timestamp = in_one_hour.strftime("%Y-%m-%dT%H:%M:%S.%f000Z")
    (isolated_config / "credentials.json").write_text(
        json.dumps(
            {
                "tokens": {
                    "prod": {
                        "access_token": "cached-jwt",
                        "token_type": "Bearer",
                        "expires_at": timestamp,
                    }
                }
            }
        )
    )
    cached = load_cached_token("prod")
    assert cached is not None
    assert cached.token == "cached-jwt"
    assert not cached.is_expired()


def test_cached_token_is_expired(isolated_config: Path) -> None:
    past = datetime.now(timezone.utc) - timedelta(hours=1)
    (isolated_config / "credentials.json").write_text(
        json.dumps(
            {
                "tokens": {
                    "prod": {
                        "access_token": "old",
                        "token_type": "Bearer",
                        "expires_at": past.strftime("%Y-%m-%dT%H:%M:%S.000000Z"),
                    }
                }
            }
        )
    )
    cached = load_cached_token("prod")
    assert cached is not None
    assert cached.is_expired()


def test_resolve_context_explicit_wins(monkeypatch: pytest.MonkeyPatch) -> None:
    from marmot.config import Context

    contexts = {
        "prod": Context(name="prod", host="https://prod"),
        "stg": Context(name="stg", host="https://stg"),
    }
    monkeypatch.setenv(ENVIRONMENT_MARMOT_CONTEXT, "stg")
    config = MarmotConfig(contexts=contexts, current_context="stg")
    assert resolve_context(context_name="prod", config=config) == contexts["prod"]


def test_resolve_context_env_over_active(monkeypatch: pytest.MonkeyPatch) -> None:
    from marmot.config import Context

    contexts = {
        "prod": Context(name="prod", host="https://prod"),
        "stg": Context(name="stg", host="https://stg"),
    }
    monkeypatch.setenv(ENVIRONMENT_MARMOT_CONTEXT, "stg")
    config = MarmotConfig(contexts=contexts, current_context="prod")
    assert resolve_context(config=config) == contexts["stg"]


def test_resolve_context_falls_back_to_active(monkeypatch: pytest.MonkeyPatch) -> None:
    from marmot.config import Context

    contexts = {"prod": Context(name="prod", host="https://prod")}
    monkeypatch.delenv(ENVIRONMENT_MARMOT_CONTEXT, raising=False)
    config = MarmotConfig(contexts=contexts, current_context="prod")
    assert resolve_context(config=config) == contexts["prod"]


def test_config_dir_uses_xdg(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg"))
    assert config_dir() == tmp_path / "xdg" / "marmot"


def test_cached_token_defaults_scheme_when_absent(isolated_config: Path) -> None:
    (isolated_config / "credentials.json").write_text(
        json.dumps(
            {"tokens": {"prod": {"access_token": "jwt", "expires_at": "2099-01-01T00:00:00Z"}}}
        )
    )
    cached = load_cached_token("prod")
    assert cached is not None
    assert cached.token_scheme == "Bearer"


def test_cached_token_unusable_entry_returns_none(isolated_config: Path) -> None:
    (isolated_config / "credentials.json").write_text(
        json.dumps(
            {
                "tokens": {
                    "no-expiry": {"access_token": "jwt"},
                    "empty-token": {"access_token": "", "expires_at": "2099-01-01T00:00:00Z"},
                    "bad-date": {"access_token": "jwt", "expires_at": "not-a-date"},
                }
            }
        )
    )
    assert load_cached_token("no-expiry") is None
    assert load_cached_token("empty-token") is None
    assert load_cached_token("bad-date") is None


def test_cached_token_keeps_nanosecond_timestamps(isolated_config: Path) -> None:
    """Go writes nanoseconds; pydantic truncates rather than rejecting."""
    (isolated_config / "credentials.json").write_text(
        json.dumps(
            {
                "tokens": {
                    "prod": {"access_token": "jwt", "expires_at": "2099-01-01T10:00:00.123456789Z"}
                }
            }
        )
    )
    cached = load_cached_token("prod")
    assert cached is not None
    assert cached.expires_at.microsecond == 123456
    assert cached.expires_at.tzinfo is not None
