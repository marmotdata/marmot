"""Auth chain ordering — explicit > env > cached login > workload identity."""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from marmot.auth import SecurityScheme, resolve_credential, resolve_host
from marmot.auth.workload import SubjectToken
from marmot.errors import AuthError


class _StaticSource:
    """Test double that always returns the configured token."""

    name = "test"

    def __init__(self, token: str | None) -> None:
        self._token = token

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        return SubjectToken(token=self._token, source=self.name) if self._token else None


@pytest.fixture(autouse=True)
def isolated_env(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """Never read the developer's real environment or ~/.config/marmot."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    for var in ("MARMOT_API_KEY", "MARMOT_TOKEN", "MARMOT_HOST", "MARMOT_CONTEXT"):
        monkeypatch.delenv(var, raising=False)


def _write_login(tmp_path: Path, context: str, token: str, expires_in: timedelta) -> None:
    config_dir = tmp_path / "marmot"
    config_dir.mkdir(parents=True, exist_ok=True)
    (config_dir / "config.yaml").write_text(
        f"current_context: {context}\ncontexts:\n  {context}:\n    host: http://cached-host\n"
    )
    expires_at = (datetime.now(timezone.utc) + expires_in).isoformat().replace("+00:00", "Z")
    (config_dir / "credentials.json").write_text(
        json.dumps(
            {
                "tokens": {
                    context: {
                        "access_token": token,
                        "token_type": "Bearer",
                        "expires_at": expires_at,
                    }
                }
            }
        )
    )


def test_explicit_api_key_wins(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "should-not-be-used")
    monkeypatch.setenv("MARMOT_TOKEN", "should-not-be-used")
    cred = resolve_credential("http://x", api_key="explicit-key")
    assert cred.scheme is SecurityScheme.apikey
    assert cred.get_token() == "explicit-key"
    assert "explicit" in cred.source


def test_explicit_token_wins(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_TOKEN", "should-not-be-used")
    cred = resolve_credential("http://x", token="explicit-token")
    assert cred.scheme is SecurityScheme.bearer
    assert cred.get_token() == "explicit-token"


def test_api_key_beats_token_when_both_explicit() -> None:
    cred = resolve_credential("http://x", api_key="key", token="tok")
    assert cred.scheme is SecurityScheme.apikey


def test_env_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "env-key")
    cred = resolve_credential("http://x")
    assert cred.scheme is SecurityScheme.apikey
    assert cred.get_token() == "env-key"


def test_env_token_when_no_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_TOKEN", "env-tok")
    cred = resolve_credential("http://x")
    assert cred.scheme is SecurityScheme.bearer
    assert cred.get_token() == "env-tok"


def test_api_key_beats_token_in_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "key")
    monkeypatch.setenv("MARMOT_TOKEN", "tok")
    assert resolve_credential("http://x").scheme is SecurityScheme.apikey


def test_cached_login_used_when_env_empty(tmp_path: Path) -> None:
    _write_login(tmp_path, "prod", "cached-jwt", timedelta(hours=1))
    cred = resolve_credential("http://x", sources=[])
    assert cred.get_token() == "cached-jwt"
    assert "prod" in cred.source


def test_expired_cached_login_is_skipped(tmp_path: Path) -> None:
    _write_login(tmp_path, "prod", "stale-jwt", timedelta(hours=-1))
    with pytest.raises(AuthError, match="No Marmot credentials found"):
        resolve_credential("http://x", sources=[])


def test_cached_login_follows_selected_context(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _write_login(tmp_path, "prod", "prod-jwt", timedelta(hours=1))
    monkeypatch.setenv("MARMOT_CONTEXT", "missing")
    with pytest.raises(AuthError, match="No Marmot credentials found"):
        resolve_credential("http://x", sources=[])


def test_no_credential_no_workload_raises() -> None:
    with pytest.raises(AuthError, match="No Marmot credentials found"):
        resolve_credential("http://x", sources=[])


def test_workload_source_runs_when_no_other_credential(httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://x/oauth/token",
        json={"access_token": "exchanged-jwt", "token_type": "Bearer", "expires_in": 3600},
    )
    cred = resolve_credential("http://x", sources=[_StaticSource("subject-jwt")])
    assert cred.get_token() == "exchanged-jwt"
    assert cred.scheme is SecurityScheme.bearer
    assert "test" in cred.source


def test_explicit_credential_skips_workload() -> None:
    """An explicit credential must not trigger a token exchange."""
    cred = resolve_credential(
        "http://x", token="explicit", sources=[_StaticSource("should-not-fire")]
    )
    assert cred.get_token() == "explicit"


def test_host_from_argument_beats_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_HOST", "http://env-host")
    assert resolve_host("http://explicit-host") == "http://explicit-host"


def test_host_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_HOST", "http://env-host")
    assert resolve_host() == "http://env-host"


def test_host_from_context(tmp_path: Path) -> None:
    _write_login(tmp_path, "prod", "cached-jwt", timedelta(hours=1))
    assert resolve_host() == "http://cached-host"


def test_no_host_raises() -> None:
    with pytest.raises(AuthError, match="no Marmot host"):
        resolve_host()
