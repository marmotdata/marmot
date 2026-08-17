"""Refresh-on-401: the retry must carry a freshly exchanged token.

``param_serialize`` applies the Authorization header before ``call_api`` runs,
so a refresh that only updates the Configuration would replay the old token.

These exercise ``GET /users/me`` because it is currently the only operation the
spec attaches a security requirement to.
"""

from __future__ import annotations

import threading

import anyio
import pytest

from marmot import AuthenticatedApiClient, SearchApi, UsersApi
from marmot.auth import OIDCCredential, SecurityScheme, StaticCredential
from marmot.auth.workload import SubjectToken
from marmot.errors import AuthError
from marmot.generated import ApiClient, Configuration

HOST = "http://x"
ME_URL = f"{HOST}/users/me"
ME_BODY = {"id": "u1", "email": "dev@example.com"}
TOKEN_URL = f"{HOST}/oauth/token"


class _StaticSource:
    name = "test"

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        return SubjectToken(token="subject-jwt", source=self.name)


def _oidc_client(
    httpx_mock: object, *tokens: str, expires_in: int = 3600
) -> AuthenticatedApiClient:
    for token in tokens:
        httpx_mock.add_response(  # type: ignore[attr-defined]
            method="POST", url=TOKEN_URL, json={"access_token": token, "expires_in": expires_in}
        )
    credential = OIDCCredential(
        api_client=ApiClient(Configuration(host=HOST)),
        audience=HOST,
        sources=[_StaticSource()],
    )
    return AuthenticatedApiClient(HOST, credential)


def test_401_refreshes_and_retries_with_the_new_token(httpx_mock: object) -> None:
    client = _oidc_client(httpx_mock, "first-jwt", "second-jwt")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        status_code=401,
        match_headers={"Authorization": "Bearer first-jwt"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer second-jwt"},
    )

    assert UsersApi(client).users_me_get_sync().id == "u1"


def test_401_refresh_does_not_deadlock_sync_callers(httpx_mock: object) -> None:
    """The sync facade runs on a shared loop; a sync exchange there would deadlock."""
    client = _oidc_client(httpx_mock, "first-jwt", "second-jwt")
    httpx_mock.add_response(method="GET", url=ME_URL, status_code=401)  # type: ignore[attr-defined]
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer second-jwt"},
    )

    finished = threading.Event()

    def call() -> None:
        UsersApi(client).users_me_get_sync()
        finished.set()

    threading.Thread(target=call, daemon=True).start()
    assert finished.wait(10), "refresh-on-401 deadlocked the shared sync event loop"


def test_401_is_not_retried_for_static_credentials(httpx_mock: object) -> None:
    client = AuthenticatedApiClient(HOST, StaticCredential("key", SecurityScheme.apikey))
    httpx_mock.add_response(method="GET", url=ME_URL, status_code=401)  # type: ignore[attr-defined]

    with pytest.raises(AuthError):
        UsersApi(client).users_me_get_sync()


def test_second_401_surfaces_as_auth_error(httpx_mock: object) -> None:
    client = _oidc_client(httpx_mock, "first-jwt", "second-jwt")
    httpx_mock.add_response(method="GET", url=ME_URL, status_code=401)  # type: ignore[attr-defined]
    httpx_mock.add_response(method="GET", url=ME_URL, status_code=401)  # type: ignore[attr-defined]

    with pytest.raises(AuthError):
        UsersApi(client).users_me_get_sync()


def test_async_callers_refresh_on_their_own_loop(httpx_mock: object) -> None:
    client = _oidc_client(httpx_mock, "first-jwt", "second-jwt")
    httpx_mock.add_response(method="GET", url=ME_URL, status_code=401)  # type: ignore[attr-defined]
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer second-jwt"},
    )

    async def call() -> str | None:
        return (await UsersApi(client).users_me_get()).id

    assert anyio.run(call) == "u1"


def test_first_call_sends_the_exchanged_token(httpx_mock: object) -> None:
    client = _oidc_client(httpx_mock, "first-jwt")
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer first-jwt"},
    )

    assert UsersApi(client).users_me_get_sync().id == "u1"


def test_api_key_is_wired_to_its_own_header_unprefixed() -> None:
    """Which endpoints actually send it is decided by each operation's `security`."""
    client = AuthenticatedApiClient(HOST, StaticCredential("secret-key", SecurityScheme.apikey))

    setting = client.configuration.auth_settings()["ApiKeyAuth"]
    assert setting["in"] == "header"
    assert setting["key"] == "X-API-KEY"
    assert setting["value"] == "secret-key"


def test_bearer_token_is_wired_with_its_prefix() -> None:
    client = AuthenticatedApiClient(HOST, StaticCredential("jwt", SecurityScheme.bearer))

    setting = client.configuration.auth_settings()["BearerAuth"]
    assert setting["key"] == "Authorization"
    assert setting["value"] == "Bearer jwt"


def test_expiring_token_is_refreshed_before_the_request(httpx_mock: object) -> None:
    """A token past its leeway is replaced pre-flight, without spending a 401."""
    client = _oidc_client(httpx_mock, "short-lived-jwt", expires_in=5)
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST", url=TOKEN_URL, json={"access_token": "renewed-jwt", "expires_in": 3600}
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer renewed-jwt"},
    )

    assert client.credential.is_stale, "expires_in=3600 within leeway is not stale; see fixture"
    assert UsersApi(client).users_me_get_sync().id == "u1"


def test_refresh_does_not_add_auth_to_unauthenticated_operations(httpx_mock: object) -> None:
    """Whether an operation sends a credential is decided by its `security`, not by us."""
    client = _oidc_client(httpx_mock, "short-lived-jwt", expires_in=5)
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST", url=TOKEN_URL, json={"access_token": "renewed-jwt", "expires_in": 3600}
    )
    httpx_mock.add_response(method="GET", url=f"{HOST}/search?q=orders", json={"results": []})  # type: ignore[attr-defined]

    SearchApi(client).search_get_sync(q="orders")

    search = [r for r in httpx_mock.get_requests() if r.url.path == "/search"]  # type: ignore[attr-defined]
    assert search
    assert "authorization" not in search[-1].headers
