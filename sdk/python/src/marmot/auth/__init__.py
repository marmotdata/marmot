"""Auth resolution for the Marmot SDK.

The chain is non-interactive — no browser, no prompts. Order:

  1. Explicit kwargs    api_key=, token=
  2. Env vars           MARMOT_API_KEY, MARMOT_TOKEN
  3. Cached credentials ~/.config/marmot/credentials.json (written by `marmot login`)
  4. Workload identity  K8s SA / GCP metadata / GitHub Actions OIDC → RFC 8693 exchange

Each step returns a Credential or None. The first to return wins.
"""

from __future__ import annotations

import os
from collections.abc import Callable
from dataclasses import dataclass
from datetime import datetime
from typing import TYPE_CHECKING

from marmot import _config
from marmot.errors import AuthError

if TYPE_CHECKING:
    from marmot.auth.workload import WorkloadIdentitySource


@dataclass
class Credential:
    """A resolved credential.

    - For API keys: ``token`` is the key, ``scheme`` is ``"X-API-Key"``.
    - For bearer tokens: ``token`` is the JWT, ``scheme`` is ``"Bearer"``.
    """

    token: str
    scheme: str  # "Bearer" or "X-API-Key"
    expires_at: datetime | None = None
    refresh: Callable[[], Credential] | None = None
    source: str = ""  # human-readable origin, for debug/logging


def resolve(
    *,
    base_url: str | None,
    api_key: str | None = None,
    token: str | None = None,
    context: str | None = None,
    env: dict[str, str] | None = None,
    workload_sources: list[WorkloadIdentitySource] | None = None,
) -> tuple[str, Credential]:
    """Resolve (base_url, credential).

    Raises :class:`AuthError` if no source produces a credential or no host can be determined.
    """
    if env is None:
        env = dict(os.environ)

    cred = _try_explicit(api_key=api_key, token=token)
    if cred is None:
        cred = _try_env(env)

    contexts, active = _config.load_contexts()
    selected = _config.resolve_context(explicit=context, contexts=contexts, active=active, env=env)

    resolved_url = base_url or env.get("MARMOT_HOST") or (selected.host if selected else None)

    if cred is None and selected is not None:
        cred = _try_cached_token(selected.name)

    if cred is None and resolved_url:
        cred = _try_workload_identity(base_url=resolved_url, sources=workload_sources)

    if not resolved_url:
        raise AuthError(
            "no Marmot host configured. Set MARMOT_HOST, pass base_url=, or run `marmot login` first."
        )
    if cred is None:
        raise AuthError(
            "no Marmot credentials found. Set MARMOT_API_KEY / MARMOT_TOKEN, "
            "run `marmot login`, or run inside K8s/GCP/GitHub Actions for workload identity."
        )

    return resolved_url, cred


def _try_explicit(*, api_key: str | None, token: str | None) -> Credential | None:
    if api_key:
        return Credential(token=api_key, scheme="X-API-Key", source="explicit api_key")
    if token:
        return Credential(token=token, scheme="Bearer", source="explicit token")
    return None


def _try_env(env: dict[str, str]) -> Credential | None:
    if key := env.get("MARMOT_API_KEY"):
        return Credential(token=key, scheme="X-API-Key", source="env MARMOT_API_KEY")
    if tok := env.get("MARMOT_TOKEN"):
        return Credential(token=tok, scheme="Bearer", source="env MARMOT_TOKEN")
    return None


def _try_cached_token(context_name: str) -> Credential | None:
    cached = _config.load_cached_token(context_name)
    if cached is None or cached.is_expired():
        return None
    return Credential(
        token=cached.access_token,
        scheme="Bearer",
        expires_at=cached.expires_at,
        source=f"cached credential for context {context_name!r}",
    )


def _try_workload_identity(
    *,
    base_url: str,
    sources: list[WorkloadIdentitySource] | None,
) -> Credential | None:
    from marmot.auth.exchange import exchange
    from marmot.auth.workload import default_sources

    for src in sources or default_sources():
        subject_token = src.fetch()
        if subject_token is None:
            continue
        return exchange(
            base_url=base_url,
            subject_token=subject_token.token,
            subject_token_type=subject_token.token_type,
            source_name=src.name,
        )
    return None
