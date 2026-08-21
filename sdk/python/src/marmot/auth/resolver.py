"""Resolve the host and credential a client should use.

Credentials are tried most-specific first: explicit arguments, then the
environment, then the token cached by `marmot login`, then workload identity.
"""

from __future__ import annotations

import os
from collections.abc import Callable
from functools import partial
from typing import TYPE_CHECKING

from marmot.auth.credential import (
    ENVIRONMENT_API_KEY,
    ENVIRONMENT_BEARER_TOKEN,
    Credential,
    from_api_key,
    from_bearer_token,
    from_cached_login,
    from_environment_api_key,
    from_environment_bearer_token,
)
from marmot.auth.oidc import OIDCCredential
from marmot.config import ENVIRONMENT_HOST, resolve_context
from marmot.errors import AuthError
from marmot.generated import ApiClient, Configuration

if TYPE_CHECKING:
    from collections.abc import Sequence

    from marmot.auth.workload import WorkloadIdentitySource

CredentialProvider = Callable[[], Credential]


class CredentialChain:
    """Tries each named provider in order and returns the first credential."""

    def __init__(self, providers: Sequence[tuple[str, CredentialProvider]]) -> None:
        self.providers = list(providers)

    def resolve(self) -> Credential:
        attempts = []
        for name, provider in self.providers:
            try:
                return provider()
            except AuthError as e:
                attempts.append(f"  {name}: {e}")

        raise AuthError(
            f"No Marmot credentials found. Set {ENVIRONMENT_API_KEY} / {ENVIRONMENT_BEARER_TOKEN}, "
            "run `marmot login`, or run inside K8s/GCP/GitHub Actions for workload identity.\n"
            + "\n".join(attempts)
        )


def resolve_host(host: str | None = None, context_name: str | None = None) -> str:
    context = resolve_context(context_name=context_name)
    host = host or os.environ.get(ENVIRONMENT_HOST) or (context.host if context else None)
    if not host:
        raise AuthError(
            f"no Marmot host configured. Set {ENVIRONMENT_HOST}, pass it explicitly, or run `marmot login` first."
        )
    return host.rstrip("/")


def resolve_credential(
    host: str,
    api_key: str | None = None,
    token: str | None = None,
    context_name: str | None = None,
    api_client: ApiClient | None = None,
    sources: Sequence[WorkloadIdentitySource] | None = None,
) -> Credential:
    """Resolve a credential for ``host``.

    ``api_client`` is used for the workload token exchange and can be unauthorized.
    """
    providers: list[tuple[str, CredentialProvider]] = []
    if api_key:
        providers.append(("explicit api key", partial(from_api_key, api_key)))
    if token:
        providers.append(("explicit bearer token", partial(from_bearer_token, token)))
    providers += [
        (f"${ENVIRONMENT_API_KEY}", from_environment_api_key),
        (f"${ENVIRONMENT_BEARER_TOKEN}", from_environment_bearer_token),
        ("cached login", partial(from_cached_login, context_name)),
        ("workload identity", partial(_exchanged_credential, host, api_client, sources)),
    ]
    return CredentialChain(providers).resolve()


def _exchanged_credential(
    host: str,
    api_client: ApiClient | None,
    sources: Sequence[WorkloadIdentitySource] | None,
) -> Credential:
    root = host.rstrip("/")
    credential = OIDCCredential(
        api_client=api_client or ApiClient(Configuration(host=root)),
        audience=root,
        sources=sources,
    )
    credential.get_token()  # exchange now so a failure is reported by the chain
    return credential
