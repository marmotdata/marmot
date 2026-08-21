"""Refresh-on-401: the retry must carry a freshly exchanged token.

``param_serialize`` applies the Authorization header before ``call_api`` runs,
so a refresh that only updates the Configuration would replay the old token.

``GET /users/me`` stands in for a protected operation and ``GET /ui/config`` for
a public one. Both sit under the API base path; ``/oauth/token`` does not.
"""

from __future__ import annotations

import threading

import anyio
import httpx
import pytest

from marmot import AuthenticatedApiClient, UiApi, UsersApi
from marmot.auth import OIDCCredential, SecurityScheme, StaticCredential, resolve_credential
from marmot.auth.workload import SubjectToken
from marmot.client import api_base_url
from marmot.errors import AuthError
from marmot.generated import ApiClient, Configuration

HOST = "http://x"
ME_URL = f"{HOST}/api/v1/users/me"
ME_BODY = {"id": "u1", "email": "dev@example.com"}
TOKEN_URL = f"{HOST}/oauth/token"


class _StaticSource:
    name = "test"

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        return SubjectToken(token="subject-jwt", source=self.name)


def _oidc_client(httpx_mock: object, *tokens: str) -> AuthenticatedApiClient:
    for token in tokens:
        httpx_mock.add_response(  # type: ignore[attr-defined]
            method="POST", url=TOKEN_URL, json={"access_token": token}
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


def test_api_key_is_sent_unprefixed_under_its_own_header(httpx_mock: object) -> None:
    client = AuthenticatedApiClient(HOST, StaticCredential("secret-key", SecurityScheme.apikey))
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET", url=ME_URL, json=ME_BODY, match_headers={"X-API-KEY": "secret-key"}
    )

    assert UsersApi(client).users_me_get_sync().id == "u1"


def test_bearer_token_is_sent_with_its_prefix(httpx_mock: object) -> None:
    client = AuthenticatedApiClient(HOST, StaticCredential("jwt", SecurityScheme.bearer))
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET", url=ME_URL, json=ME_BODY, match_headers={"Authorization": "Bearer jwt"}
    )

    assert UsersApi(client).users_me_get_sync().id == "u1"


def test_public_operations_are_called_without_credentials(httpx_mock: object) -> None:
    """Whether an operation sends a credential is decided by its `security`, not by us."""
    client = _oidc_client(httpx_mock, "first-jwt")
    httpx_mock.add_response(method="GET", url=f"{HOST}/api/v1/ui/config", json={})  # type: ignore[attr-defined]

    UiApi(client).ui_config_get_sync()

    public = [r for r in httpx_mock.get_requests() if r.url.path == "/api/v1/ui/config"]  # type: ignore[attr-defined]
    assert public
    assert "authorization" not in public[-1].headers


def test_concurrent_401s_all_recover(httpx_mock: object) -> None:
    """Each rejected request refreshes on its own; every one must still succeed."""
    issued = iter(f"jwt-{n}" for n in range(1, 10))

    async def exchange(request: httpx.Request) -> httpx.Response:
        await anyio.sleep(0.05)  # hold the exchanges open so they overlap
        return httpx.Response(200, json={"access_token": next(issued)})

    async def me(request: httpx.Request) -> httpx.Response:
        if request.headers.get("authorization") == "Bearer jwt-1":
            return httpx.Response(401, json={})  # the token every request starts with
        return httpx.Response(200, json=ME_BODY)

    httpx_mock.add_callback(exchange, method="POST", url=TOKEN_URL, is_reusable=True)  # type: ignore[attr-defined]
    httpx_mock.add_callback(me, method="GET", url=ME_URL, is_reusable=True)  # type: ignore[attr-defined]

    credential = OIDCCredential(
        api_client=ApiClient(Configuration(host=HOST)), audience=HOST, sources=[_StaticSource()]
    )
    api = UsersApi(AuthenticatedApiClient(HOST, credential))  # consumes "jwt-1"

    results: list[str | None] = []

    async def five_at_once() -> None:
        async def call() -> None:
            results.append((await api.users_me_get()).id)

        async with anyio.create_task_group() as group:
            for _ in range(5):
                group.start_soon(call)

    anyio.run(five_at_once)

    assert results == ["u1"] * 5


def test_api_base_path_is_added_once() -> None:
    """`marmot login` stores a bare host; the generated client wants the base URL."""
    assert api_base_url("http://x") == "http://x/api/v1"
    assert api_base_url("http://x/") == "http://x/api/v1"
    assert api_base_url("http://x/api/v1") == "http://x/api/v1"
    assert api_base_url("http://x/api/v1/") == "http://x/api/v1"


def test_token_exchange_bypasses_the_api_base_path(httpx_mock: object) -> None:
    """`/oauth/token` is served at the root, so the exchange must not be prefixed."""
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST", url=TOKEN_URL, json={"access_token": "exchanged-jwt"}
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url=ME_URL,
        json=ME_BODY,
        match_headers={"Authorization": "Bearer exchanged-jwt"},
    )

    credential = resolve_credential(HOST, sources=[_StaticSource()])
    assert UsersApi(AuthenticatedApiClient(HOST, credential)).users_me_get_sync().id == "u1"
