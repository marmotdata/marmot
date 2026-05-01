"""Auth chain ordering — explicit > env > cached > workload."""

from __future__ import annotations

import pytest

from marmot.auth import Credential, resolve
from marmot.auth.workload import SubjectToken, WorkloadIdentitySource
from marmot.errors import AuthError


class _StaticSource:
    """Test double that always returns the configured token."""

    def __init__(self, name: str, token: str | None) -> None:
        self.name = name
        self._token = token

    def fetch(self) -> SubjectToken | None:
        return SubjectToken(token=self._token) if self._token else None


def test_explicit_api_key_wins(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "should-not-be-used")
    monkeypatch.setenv("MARMOT_TOKEN", "should-not-be-used")
    base, cred = resolve(base_url="http://x", api_key="explicit-key")
    assert base == "http://x"
    assert cred.scheme == "X-API-Key"
    assert cred.token == "explicit-key"
    assert "explicit" in cred.source


def test_explicit_token_wins() -> None:
    _, cred = resolve(base_url="http://x", token="explicit-token")
    assert cred.scheme == "Bearer"
    assert cred.token == "explicit-token"


def test_env_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "env-key")
    monkeypatch.delenv("MARMOT_TOKEN", raising=False)
    _, cred = resolve(base_url="http://x")
    assert cred.scheme == "X-API-Key"
    assert cred.token == "env-key"


def test_env_token_when_no_api_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MARMOT_API_KEY", raising=False)
    monkeypatch.setenv("MARMOT_TOKEN", "env-tok")
    _, cred = resolve(base_url="http://x")
    assert cred.scheme == "Bearer"
    assert cred.token == "env-tok"


def test_api_key_beats_token_in_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MARMOT_API_KEY", "key")
    monkeypatch.setenv("MARMOT_TOKEN", "tok")
    _, cred = resolve(base_url="http://x")
    assert cred.scheme == "X-API-Key"


def test_no_credential_no_workload_raises(
    monkeypatch: pytest.MonkeyPatch, tmp_path: pytest.TempPathFactory
) -> None:
    # Isolate from the dev's actual ~/.config/marmot
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    for var in ("MARMOT_API_KEY", "MARMOT_TOKEN", "MARMOT_HOST", "MARMOT_CONTEXT"):
        monkeypatch.delenv(var, raising=False)

    with pytest.raises(AuthError, match="no Marmot credentials"):
        resolve(base_url="http://x", workload_sources=[])


def test_no_host_raises(monkeypatch: pytest.MonkeyPatch, tmp_path: pytest.TempPathFactory) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    for var in ("MARMOT_HOST", "MARMOT_CONTEXT"):
        monkeypatch.delenv(var, raising=False)

    with pytest.raises(AuthError, match="no Marmot host"):
        resolve(base_url=None, api_key="k", workload_sources=[])


def test_workload_source_runs_when_no_other_credential(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: pytest.TempPathFactory,
    httpx_mock: object,  # pytest-httpx fixture
) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    for var in ("MARMOT_API_KEY", "MARMOT_TOKEN", "MARMOT_HOST", "MARMOT_CONTEXT"):
        monkeypatch.delenv(var, raising=False)

    # pytest-httpx import + assertion
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://x/oauth/token",
        json={"access_token": "exchanged-jwt", "token_type": "Bearer", "expires_in": 3600},
    )

    src: WorkloadIdentitySource = _StaticSource("test", "subject-jwt")
    base, cred = resolve(base_url="http://x", workload_sources=[src])
    assert base == "http://x"
    assert cred.token == "exchanged-jwt"
    assert cred.scheme == "Bearer"
    assert cred.expires_at is not None
    assert "test" in cred.source


def test_explicit_credential_skips_workload(monkeypatch: pytest.MonkeyPatch) -> None:
    """If an explicit credential is provided, workload sources are not invoked."""
    src: WorkloadIdentitySource = _StaticSource("should-not-fire", "x")
    _, cred = resolve(base_url="http://x", token="explicit", workload_sources=[src])
    assert cred.token == "explicit"


def test_credential_dataclass() -> None:
    """Quick sanity: Credential is constructable with the fields we expect."""
    c = Credential(token="t", scheme="Bearer", source="test")
    assert c.token == "t"
    assert c.scheme == "Bearer"
    assert c.refresh is None
