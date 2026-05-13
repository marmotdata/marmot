"""Bridge between the public auth surface and the generated `_gen` client.

The generated `AuthenticatedClient` only knows a static bearer token. We need:

- Two auth schemes (X-API-Key for keys, Bearer for OAuth/workload tokens)
- Refresh-on-401, which the v0.2.0 ``Transport`` implemented manually

Both are handled by a custom :class:`httpx.Auth` plugged into the underlying
httpx client. The generated functions never see a Credential.
"""

from __future__ import annotations

from collections.abc import Generator
from typing import Any, TypeVar

import httpx

from marmot._gen.client import AuthenticatedClient
from marmot._gen.types import Response
from marmot.auth import Credential
from marmot.errors import (
    AuthError,
    MarmotError,
    NotFoundError,
    RateLimitError,
    ServerError,
    ValidationError,
)

T = TypeVar("T")


class _MarmotAuth(httpx.Auth):
    """Inject the current Credential and refresh on 401 exactly once per call."""

    def __init__(self, credential: Credential) -> None:
        self._credential = credential

    def auth_flow(self, request: httpx.Request) -> Generator[httpx.Request, httpx.Response, None]:
        self._apply(request)
        response = yield request

        if response.status_code != 401 or self._credential.refresh is None:
            return

        try:
            self._credential = self._credential.refresh()
        except MarmotError:
            raise
        except Exception as e:
            raise AuthError(f"credential refresh failed: {e}") from e

        self._apply(request)
        yield request

    def _apply(self, request: httpx.Request) -> None:
        request.headers.pop("Authorization", None)
        request.headers.pop("X-API-Key", None)
        if self._credential.scheme == "X-API-Key":
            request.headers["X-API-Key"] = self._credential.token
        else:
            request.headers["Authorization"] = f"Bearer {self._credential.token}"


def make_gen_client(
    base_url: str,
    http_client: httpx.Client,
) -> AuthenticatedClient:
    """Wrap an httpx.Client in the generated `AuthenticatedClient`.

    The caller is responsible for installing :class:`_MarmotAuth` on
    ``http_client`` (or any other auth strategy) and for closing it when
    done.

    The OpenAPI spec declares ``/api/v1`` as the server base, so we append it
    here — generated URLs are relative (e.g. ``/assets/{id}``).
    """
    client = AuthenticatedClient(base_url=base_url.rstrip("/") + "/api/v1", token="")
    client.set_httpx_client(http_client)
    return client


def make_marmot_auth(credential: Credential) -> _MarmotAuth:
    """Build an httpx auth strategy that injects credentials and refreshes on 401."""
    return _MarmotAuth(credential)


def unwrap(response: Response[Any]) -> Any:
    """Return parsed body on 2xx, raise typed Marmot error otherwise."""
    status = int(response.status_code)
    if 200 <= status < 300:
        return response.parsed

    msg = _error_message(response)
    if status == 400:
        raise ValidationError(msg, status_code=status)
    if status in (401, 403):
        raise AuthError(msg, status_code=status)
    if status == 404:
        raise NotFoundError(msg, status_code=status)
    if status == 429:
        raise RateLimitError(msg, status_code=status)
    if status >= 500:
        raise ServerError(msg, status_code=status)
    raise MarmotError(msg, status_code=status)


def _error_message(response: Response[Any]) -> str:
    parsed = response.parsed
    for attr in ("error_description", "error", "message"):
        v = getattr(parsed, attr, None)
        if isinstance(v, str) and v:
            return v
    raw = response.content
    if raw:
        try:
            text = raw.decode("utf-8", errors="replace").strip()
            if text:
                return text
        except (UnicodeDecodeError, AttributeError):
            pass
    return f"HTTP {int(response.status_code)}"
