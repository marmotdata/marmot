"""Resource modules — verify request shapes via mocked httpx."""

from __future__ import annotations

import httpx
import pytest

from marmot.auth import Credential
from marmot.client import Client
from marmot.errors import AuthError, NotFoundError, ServerError


@pytest.fixture
def client(httpx_mock: object) -> Client:
    cred = Credential(token="test-key", scheme="X-API-Key", source="test")
    return Client(base_url="http://m", credential=cred, http_client=httpx.Client())


def test_search_sends_query_and_filters(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?query=orders&asset_types=table&limit=10",
        json={"results": [{"id": "a1"}]},
        match_headers={"X-API-Key": "test-key"},
    )
    out = client.search("orders", types=["table"], limit=10)
    assert out == {"results": [{"id": "a1"}]}


def test_assets_get(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/abc",
        json={"id": "abc", "name": "orders"},
    )
    assert client.assets.get("abc")["name"] == "orders"


def test_assets_lookup(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Table/bigquery/prod.orders",
        json={"id": "abc"},
    )
    asset = client.assets.lookup(type="Table", service="bigquery", name="prod.orders")
    assert asset["id"] == "abc"


def test_assets_find_returns_none_on_404(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Table/bigquery/missing",
        status_code=404,
        json={"error": "not found"},
    )
    assert client.assets.find(type="Table", service="bigquery", name="missing") is None


def test_assets_lookup_raises_on_404(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Table/bigquery/missing",
        status_code=404,
        json={"error": "not found"},
    )
    with pytest.raises(NotFoundError):
        client.assets.lookup(type="Table", service="bigquery", name="missing")


def test_lineage_write_default_type(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/lineage/direct",
        json={"id": "edge-1", "source": "a", "target": "b", "type": "DIRECT"},
        match_json={"source": "a", "target": "b", "type": "DIRECT"},
    )
    edge = client.lineage.write(source="a", target="b")
    assert edge["id"] == "edge-1"


def test_lineage_write_custom_type(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/lineage/direct",
        json={"id": "edge-2"},
        match_json={"source": "a", "target": "b", "type": "reads"},
    )
    client.lineage.write(source="a", target="b", type="reads")


def test_lineage_batch_normalizes_tuples(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/lineage/batch",
        json=[],
        match_json=[
            {"source": "a", "target": "b", "type": "DIRECT"},
            {"source": "c", "target": "d", "type": "writes"},
        ],
    )
    client.lineage.batch([("a", "b"), ("c", "d", "writes")])


def test_unauthorized_raises_auth_error(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/x",
        status_code=401,
        json={"error_description": "token expired"},
    )
    with pytest.raises(AuthError, match="token expired"):
        client.assets.get("x")


def test_5xx_raises_server_error(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/x",
        status_code=503,
        text="upstream down",
    )
    with pytest.raises(ServerError) as exc:
        client.assets.get("x")
    assert exc.value.status_code == 503


def test_bearer_credential_uses_authorization_header(httpx_mock: object) -> None:
    cred = Credential(token="jwt", scheme="Bearer", source="test")
    c = Client(base_url="http://m", credential=cred, http_client=httpx.Client())
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?query=x",
        json={"results": []},
        match_headers={"Authorization": "Bearer jwt"},
    )
    c.search("x")


def test_refresh_on_401(httpx_mock: object) -> None:
    """A 401 should trigger refresh once, then retry."""
    refreshed: list[int] = []

    def refresh_fn() -> Credential:
        refreshed.append(1)
        return Credential(token="new-jwt", scheme="Bearer", refresh=refresh_fn, source="r")

    cred = Credential(token="old-jwt", scheme="Bearer", refresh=refresh_fn, source="r")
    c = Client(base_url="http://m", credential=cred, http_client=httpx.Client())

    # First call returns 401; second (with new token) succeeds.
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?query=x",
        status_code=401,
        match_headers={"Authorization": "Bearer old-jwt"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?query=x",
        json={"results": []},
        match_headers={"Authorization": "Bearer new-jwt"},
    )

    c.search("x")
    assert refreshed == [1]
