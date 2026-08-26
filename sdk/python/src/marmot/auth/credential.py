"""Credentials the SDK can present to the API.

A credential pairs a token with the security scheme it satisfies. Every
factory here either returns a usable credential or raises :class:`AuthError`,
so :mod:`marmot.auth.resolver` can treat them as an ordered chain.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Protocol, runtime_checkable

from marmot.compat import StrEnum
from marmot.config import load_cached_token, resolve_context
from marmot.errors import AuthError

ENVIRONMENT_API_KEY = "MARMOT_API_KEY"
ENVIRONMENT_BEARER_TOKEN = "MARMOT_TOKEN"  # noqa: S105


class SecurityScheme(StrEnum):
    """Mirrors supported security schemes from openAPI spec. See :class:`Configuration`"""

    apikey = "ApiKeyAuth"
    bearer = "BearerAuth"


@runtime_checkable
class Credential(Protocol):
    @property
    def scheme(self) -> SecurityScheme:
        """The security scheme implemented by this credential"""
        ...

    @property
    def source(self) -> str:
        """Human-readable origin, for debug/logging"""
        ...

    def get_token(self) -> str:
        """Return a valid access token or secret, acquiring one if needed.

        Raises :class:`AuthError` if no token can be produced.
        """
        ...


@runtime_checkable
class Refreshable(Protocol):
    """A credential that can replace a token the API rejected."""

    async def refresh(self) -> str:
        """Obtain a new token, discarding the one held. Raises :class:`AuthError` on failure."""
        ...


@dataclass(frozen=True)
class StaticCredential:
    """A credential whose token is fixed at construction."""

    token: str
    scheme: SecurityScheme
    source: str = ""

    def get_token(self) -> str:
        return self.token


def from_api_key(api_key: str | None) -> StaticCredential:
    if not api_key:
        raise AuthError("No API key was passed")
    return StaticCredential(api_key, SecurityScheme.apikey, "explicit api key")


def from_bearer_token(token: str | None) -> StaticCredential:
    if not token:
        raise AuthError("No bearer token was passed")
    return StaticCredential(token, SecurityScheme.bearer, "explicit bearer token")


def from_environment_api_key() -> StaticCredential:
    api_key = os.environ.get(ENVIRONMENT_API_KEY)
    if not api_key:
        raise AuthError(f"{ENVIRONMENT_API_KEY} is not set")
    return StaticCredential(api_key, SecurityScheme.apikey, f"${ENVIRONMENT_API_KEY}")


def from_environment_bearer_token() -> StaticCredential:
    token = os.environ.get(ENVIRONMENT_BEARER_TOKEN)
    if not token:
        raise AuthError(f"{ENVIRONMENT_BEARER_TOKEN} is not set")
    return StaticCredential(token, SecurityScheme.bearer, f"${ENVIRONMENT_BEARER_TOKEN}")


def from_cached_login(context_name: str | None = None) -> StaticCredential:
    """Read the token `marmot login` cached for the selected context."""

    context = resolve_context(context_name=context_name)
    if context is None:
        raise AuthError("No Marmot context is selected")

    cached = load_cached_token(context.name)
    if cached is None:
        raise AuthError(f"No cached token for context {context.name!r}; run `marmot login`")
    if cached.is_expired():
        raise AuthError(f"Cached token for context {context.name!r} expired; run `marmot login`")

    return StaticCredential(
        cached.token, SecurityScheme.bearer, f"cached login for context {context.name!r}"
    )
