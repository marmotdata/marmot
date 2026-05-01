"""HTTP transport with auth injection and refresh-on-401."""

from __future__ import annotations

from typing import Any

import httpx

from marmot.auth import Credential
from marmot.errors import AuthError, MarmotError, NotFoundError, ServerError

_DEFAULT_TIMEOUT = 30.0


class Transport:
    """Thin httpx wrapper that injects credentials and refreshes on 401.

    All requests go through :meth:`request`; resource modules call
    :meth:`get` / :meth:`post` / etc. for ergonomics.
    """

    def __init__(
        self,
        *,
        base_url: str,
        credential: Credential,
        timeout: float = _DEFAULT_TIMEOUT,
        client: httpx.Client | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._credential = credential
        self._client = client or httpx.Client(timeout=timeout)
        self._owns_client = client is None

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def __enter__(self) -> Transport:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()

    @property
    def base_url(self) -> str:
        return self._base_url

    def request(
        self,
        method: str,
        path: str,
        *,
        json: Any = None,
        params: dict[str, Any] | None = None,
        _retried: bool = False,
    ) -> Any:
        url = self._url(path)
        resp = self._client.request(
            method, url, json=json, params=params, headers=self._auth_headers()
        )

        if resp.status_code == 401 and not _retried and self._credential.refresh is not None:
            try:
                self._credential = self._credential.refresh()
            except MarmotError:
                raise
            except Exception as e:  # network errors etc.
                raise AuthError(f"credential refresh failed: {e}") from e
            return self.request(method, path, json=json, params=params, _retried=True)

        return _parse(resp)

    def get(self, path: str, *, params: dict[str, Any] | None = None) -> Any:
        return self.request("GET", path, params=params)

    def post(self, path: str, *, json: Any = None, params: dict[str, Any] | None = None) -> Any:
        return self.request("POST", path, json=json, params=params)

    def put(self, path: str, *, json: Any = None) -> Any:
        return self.request("PUT", path, json=json)

    def delete(self, path: str) -> Any:
        return self.request("DELETE", path)

    def _url(self, path: str) -> str:
        if path.startswith("/"):
            return f"{self._base_url}{path}"
        return f"{self._base_url}/{path}"

    def _auth_headers(self) -> dict[str, str]:
        scheme = self._credential.scheme
        token = self._credential.token
        if scheme == "X-API-Key":
            return {"X-API-Key": token}
        return {"Authorization": f"Bearer {token}"}


def _parse(resp: httpx.Response) -> Any:
    if 200 <= resp.status_code < 300:
        if not resp.content:
            return None
        try:
            return resp.json()
        except ValueError as e:
            raise ServerError(f"non-JSON response from {resp.request.url}: {e}") from e

    if resp.status_code == 401:
        raise AuthError(_error_message(resp, default="unauthorized"))
    if resp.status_code == 403:
        raise AuthError(_error_message(resp, default="forbidden"))
    if resp.status_code == 404:
        raise NotFoundError(_error_message(resp, default="not found"))
    if resp.status_code >= 500:
        raise ServerError(
            _error_message(resp, default=f"server error (HTTP {resp.status_code})"),
            status_code=resp.status_code,
        )
    raise MarmotError(_error_message(resp, default=f"HTTP {resp.status_code}"))


def _error_message(resp: httpx.Response, *, default: str) -> str:
    try:
        body = resp.json()
    except ValueError:
        return resp.text or default
    if isinstance(body, dict):
        for key in ("error_description", "error", "message"):
            v = body.get(key)
            if isinstance(v, str) and v:
                return v
    return resp.text or default
