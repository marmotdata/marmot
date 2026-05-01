"""Workload-identity detectors: each must be silent when its env is absent."""

from __future__ import annotations

from pathlib import Path

import pytest

from marmot.auth.workload.gcp import GCPWorkloadIdentitySource
from marmot.auth.workload.github import GitHubActionsSource
from marmot.auth.workload.kubernetes import KubernetesServiceAccountSource


def test_kubernetes_returns_none_when_token_missing(tmp_path: Path) -> None:
    src = KubernetesServiceAccountSource(token_path=tmp_path / "missing")
    assert src.fetch() is None


def test_kubernetes_reads_token_from_path(tmp_path: Path) -> None:
    p = tmp_path / "token"
    p.write_text("k8s-sa-jwt\n")
    src = KubernetesServiceAccountSource(token_path=p)
    tok = src.fetch()
    assert tok is not None
    assert tok.token == "k8s-sa-jwt"


def test_kubernetes_returns_none_for_empty_token(tmp_path: Path) -> None:
    p = tmp_path / "token"
    p.write_text("   \n")
    src = KubernetesServiceAccountSource(token_path=p)
    assert src.fetch() is None


def test_gcp_returns_none_outside_gcp(monkeypatch: pytest.MonkeyPatch) -> None:
    for var in ("GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "K_SERVICE", "FUNCTION_TARGET"):
        monkeypatch.delenv(var, raising=False)
    src = GCPWorkloadIdentitySource(audience="http://marmot")
    assert src.fetch() is None


def test_gcp_fetches_token_from_metadata(
    monkeypatch: pytest.MonkeyPatch, httpx_mock: object
) -> None:
    monkeypatch.setenv("GOOGLE_CLOUD_PROJECT", "my-project")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=http://marmot&format=full",
        text="gcp-id-token",
    )
    src = GCPWorkloadIdentitySource(audience="http://marmot")
    tok = src.fetch()
    assert tok is not None
    assert tok.token == "gcp-id-token"


def test_gcp_returns_none_on_metadata_error(
    monkeypatch: pytest.MonkeyPatch, httpx_mock: object
) -> None:
    monkeypatch.setenv("GOOGLE_CLOUD_PROJECT", "my-project")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=http://marmot&format=full",
        status_code=500,
    )
    src = GCPWorkloadIdentitySource(audience="http://marmot")
    assert src.fetch() is None


def test_github_returns_none_without_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for var in ("ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN"):
        monkeypatch.delenv(var, raising=False)
    src = GitHubActionsSource(audience="http://marmot")
    assert src.fetch() is None


def test_github_fetches_oidc_token(monkeypatch: pytest.MonkeyPatch, httpx_mock: object) -> None:
    monkeypatch.setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "http://gh-oidc/token")
    monkeypatch.setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "request-bearer")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://gh-oidc/token?audience=http://marmot",
        json={"value": "github-oidc-jwt"},
        match_headers={"Authorization": "Bearer request-bearer"},
    )
    src = GitHubActionsSource(audience="http://marmot")
    tok = src.fetch()
    assert tok is not None
    assert tok.token == "github-oidc-jwt"


def test_github_returns_none_on_bad_response(
    monkeypatch: pytest.MonkeyPatch, httpx_mock: object
) -> None:
    monkeypatch.setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "http://gh-oidc/token")
    monkeypatch.setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "request-bearer")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://gh-oidc/token?audience=http://marmot",
        json={"unexpected": "shape"},
    )
    src = GitHubActionsSource(audience="http://marmot")
    assert src.fetch() is None
