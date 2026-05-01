"""Config file parsing — must read what the CLI writes."""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from marmot._config import (
    config_dir,
    load_cached_token,
    load_contexts,
    resolve_context,
)


@pytest.fixture
def isolated_config(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> Path:
    """Point XDG_CONFIG_HOME at a temp dir so tests never touch real config."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    cfg = tmp_path / "marmot"
    cfg.mkdir()
    return cfg


def test_load_contexts_empty(isolated_config: Path) -> None:
    assert load_contexts() == ({}, None)


def test_load_contexts_parses_yaml(isolated_config: Path) -> None:
    (isolated_config / "config.yaml").write_text(
        """\
current_context: prod
contexts:
  prod:
    host: https://marmot.acme.io
  staging:
    host: https://staging.marmot.acme.io
"""
    )
    contexts, active = load_contexts()
    assert active == "prod"
    assert set(contexts) == {"prod", "staging"}
    assert contexts["prod"].host == "https://marmot.acme.io"


def test_load_contexts_skips_invalid_entries(isolated_config: Path) -> None:
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
    contexts, _ = load_contexts()
    assert set(contexts) == {"ok"}


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
    assert cached.access_token == "cached-jwt"
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
    from marmot._config import Context

    contexts = {
        "prod": Context(name="prod", host="https://prod"),
        "stg": Context(name="stg", host="https://stg"),
    }
    monkeypatch.setenv("MARMOT_CONTEXT", "stg")
    assert resolve_context(explicit="prod", contexts=contexts, active="stg") == contexts["prod"]


def test_resolve_context_env_over_active(monkeypatch: pytest.MonkeyPatch) -> None:
    from marmot._config import Context

    contexts = {
        "prod": Context(name="prod", host="https://prod"),
        "stg": Context(name="stg", host="https://stg"),
    }
    assert (
        resolve_context(contexts=contexts, active="prod", env={"MARMOT_CONTEXT": "stg"})
        == contexts["stg"]
    )


def test_resolve_context_falls_back_to_active() -> None:
    from marmot._config import Context

    contexts = {"prod": Context(name="prod", host="https://prod")}
    assert resolve_context(contexts=contexts, active="prod", env={}) == contexts["prod"]


def test_config_dir_uses_xdg(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg"))
    assert config_dir() == tmp_path / "xdg" / "marmot"
