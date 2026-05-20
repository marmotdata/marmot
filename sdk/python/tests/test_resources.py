"""Resource modules — verify request shapes via mocked httpx."""

from __future__ import annotations

import httpx
import pytest

from marmot import Asset, LineageEdge
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
        url="http://m/api/v1/search?q=orders&types=table&limit=10",
        json={"results": [{"id": "a1"}]},
        match_headers={"X-API-Key": "test-key"},
    )
    out = client.search("orders", types=["table"], limit=10)
    assert out.results is not None  # type: ignore[union-attr]
    assert len(out.results) == 1


def test_assets_get(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/abc",
        json={"id": "abc", "name": "orders"},
    )
    asset = client.assets.get("abc")
    assert isinstance(asset, Asset)
    assert asset.name == "orders"


def test_assets_lookup(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Table/bigquery/prod.orders",
        json={"id": "abc"},
    )
    asset = client.assets.lookup(type="Table", service="bigquery", name="prod.orders")
    assert asset.id == "abc"


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
    assert isinstance(edge, LineageEdge)
    assert edge.id == "edge-1"


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
        url="http://m/api/v1/search?q=x",
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

    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?q=x",
        status_code=401,
        match_headers={"Authorization": "Bearer old-jwt"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?q=x",
        json={"results": []},
        match_headers={"Authorization": "Bearer new-jwt"},
    )

    c.search("x")
    assert refreshed == [1]


def test_tags_list(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/tags",
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
    )
    tags = client.tags.list()
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"
    assert tags[0].name == "pii"


def test_tags_get(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/tags/tag-1",
        json={
            "id": "tag-1",
            "name": "pii",
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
    )
    tag = client.tags.get("tag-1")
    assert tag.id == "tag-1"
    assert tag.name == "pii"


def test_tags_create(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/tags",
        status_code=201,
        json={
            "id": "tag-1",
            "name": "pii",
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
        match_json={"name": "pii", "description": "sensitive data"},
    )
    tag = client.tags.create(name="pii", description="sensitive data")
    assert tag.id == "tag-1"
    assert tag.name == "pii"


def test_tags_create_without_description(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/tags",
        status_code=201,
        json={
            "id": "tag-1",
            "name": "pii",
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
    )
    tag = client.tags.create(name="pii")
    assert tag.name == "pii"


def test_tags_update(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/tags/tag-1",
        json={
            "id": "tag-1",
            "name": "pii-updated",
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
        match_json={"name": "pii-updated"},
    )
    tag = client.tags.update("tag-1", name="pii-updated")
    assert tag.name == "pii-updated"


def test_tags_delete(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="DELETE",
        url="http://m/api/v1/tags/tag-1",
        status_code=204,
    )
    result = client.tags.delete("tag-1")
    assert result is None


def test_assets_list_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/tags/asset-1",
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
    )
    tags = client.assets.list_tags("asset-1")
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_assets_add_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/assets/tags/asset-1",
        status_code=201,
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
        match_json={"tag_id": "tag-1"},
    )
    result = client.assets.add_tag("asset-1", "tag-1")
    assert isinstance(result, list)
    assert result[0].id == "tag-1"


def test_assets_remove_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="DELETE",
        url="http://m/api/v1/assets/tags/asset-1",
        status_code=204,
        match_json={"tag_id": "tag-1"},
    )
    result = client.assets.remove_tag("asset-1", "tag-1")
    assert result is None


def test_assets_set_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/assets/tags/asset-1",
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
        match_json={"tag_ids": ["tag-1"]},
    )
    tags = client.assets.set_tags("asset-1", ["tag-1"])
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_assets_set_column_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/assets/column-tags/asset-1",
        status_code=204,
        match_json={"column_path": "schema.table.column", "tag_ids": ["tag-1"]},
    )
    result = client.assets.set_column_tags("asset-1", "schema.table.column", ["tag-1"])
    assert result is None


def test_assets_remove_column_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="DELETE",
        url="http://m/api/v1/assets/column-tags/asset-1",
        status_code=204,
        match_json={"column_path": "schema.table.column", "tag_id": "tag-1"},
    )
    result = client.assets.remove_column_tag("asset-1", "schema.table.column", "tag-1")
    assert result is None


def test_glossary_list_term_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/glossary/tags/term-1",
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
    )
    tags = client.glossary.list_term_tags("term-1")
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_glossary_add_term_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/glossary/tags/term-1",
        status_code=201,
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
        match_json={"tag_id": "tag-1"},
    )
    tags = client.glossary.add_term_tag("term-1", "tag-1")
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_glossary_remove_term_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="DELETE",
        url="http://m/api/v1/glossary/tags/term-1",
        json={"message": "tag removed"},
        match_json={"tag_id": "tag-1"},
    )
    client.glossary.remove_term_tag("term-1", "tag-1")


def test_glossary_set_term_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/glossary/tags/term-1",
        json={"id": "term-1", "name": "PII", "definition": "Personally Identifiable Information"},
        match_json={"tag_ids": ["tag-1"]},
    )
    term = client.glossary.set_term_tags("term-1", ["tag-1"])
    assert term.id == "term-1"
    assert term.name == "PII"


def test_products_list(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/products/list",
        json={
            "data_products": [
                {
                    "id": "product-1",
                    "name": "Orders",
                    "owners": [],
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                }
            ],
            "total": 1,
        },
    )
    result = client.products.list()
    assert result.total == 1
    assert result.data_products[0].id == "product-1"  # type: ignore[index]


def test_products_list_pagination(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/products/list?limit=10&offset=20",
        json={"data_products": [], "total": 0},
    )
    result = client.products.list(limit=10, offset=20)
    assert result.total == 0


def test_products_get(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/products/product-1",
        json={
            "id": "product-1",
            "name": "Orders",
            "owners": [],
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
    )
    product = client.products.get("product-1")
    assert product.id == "product-1"
    assert product.name == "Orders"


def test_products_list_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/products/tags/product-1",
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
    )
    tags = client.products.list_tags("product-1")
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_products_add_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/products/tags/product-1",
        status_code=201,
        json=[
            {
                "id": "tag-1",
                "name": "pii",
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-01T00:00:00Z",
            }
        ],
        match_json={"tag_id": "tag-1"},
    )
    tags = client.products.add_tag("product-1", "tag-1")
    assert isinstance(tags, list)
    assert tags[0].id == "tag-1"


def test_products_remove_tag(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="DELETE",
        url="http://m/api/v1/products/tags/product-1",
        json={"message": "tag removed"},
        match_json={"tag_id": "tag-1"},
    )
    client.products.remove_tag("product-1", "tag-1")


def test_products_set_tags(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/products/tags/product-1",
        json={
            "id": "product-1",
            "name": "Orders",
            "owners": [],
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
        match_json={"tag_ids": ["tag-1"]},
    )
    product = client.products.set_tags("product-1", ["tag-1"])
    assert product.id == "product-1"
