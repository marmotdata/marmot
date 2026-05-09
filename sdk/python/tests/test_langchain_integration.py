"""Integration tests for marmot.integrations.langchain.

Skipped automatically if langchain-core isn't installed.
"""

from __future__ import annotations

from uuid import uuid4

import httpx
import pytest

pytest.importorskip("langchain_core")

from marmot.auth import Credential  # noqa: E402
from marmot.client import Client  # noqa: E402
from marmot.integrations.langchain import (  # noqa: E402
    MarmotCallbackHandler,
    catalog_tools,
    marmot_tool,
)


@pytest.fixture
def client(httpx_mock: object) -> Client:
    cred = Credential(token="test-key", scheme="X-API-Key", source="test")
    return Client(base_url="http://m", credential=cred, http_client=httpx.Client())


def test_catalog_tools_exposes_expected_tool_names(client: Client) -> None:
    tools = catalog_tools(client)
    names = {t.name for t in tools}
    assert names == {
        "search_catalog",
        "get_asset",
        "lookup_asset",
        "get_upstream_lineage",
    }


def test_catalog_tool_calls_search_endpoint(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/search?query=orders&limit=5",
        json={
            "results": [
                {
                    "id": "a1",
                    "name": "orders",
                    "description": "the orders table",
                    "metadata": {
                        "mrn": "postgres://p/s/orders",
                        "type": "Table",
                        "primary_provider": "postgres",
                    },
                }
            ],
            "total": 1,
        },
    )
    tool = next(t for t in catalog_tools(client) if t.name == "search_catalog")
    result = tool.invoke({"query": "orders", "limit": 5})
    hit = result["results"][0]
    assert hit["mrn"] == "postgres://p/s/orders"
    assert hit["type"] == "Table"
    assert hit["provider"] == "postgres"
    assert hit["name"] == "orders"


def test_handler_registers_new_agent_on_chain_start(
    client: Client, httpx_mock: object
) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Agent/LangChain/explorer",
        status_code=404,
        json={"error": "not found"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/assets",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )

    handler = MarmotCallbackHandler(
        client, name="explorer", model="gpt-4o", owner="data-eng"
    )
    handler.on_chain_start({}, {}, run_id=uuid4(), parent_run_id=None)

    assert handler.agent_mrn == "marmot://langchain/agent/explorer"


def test_handler_updates_existing_agent(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Agent/LangChain/explorer",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/assets/agent-1",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )

    handler = MarmotCallbackHandler(client, name="explorer")
    handler.on_chain_start({}, {}, run_id=uuid4(), parent_run_id=None)

    assert handler.agent_mrn == "marmot://langchain/agent/explorer"


def test_handler_records_run_with_tool_call(
    client: Client, httpx_mock: object
) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Agent/LangChain/explorer",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT",
        url="http://m/api/v1/assets/agent-1",
        json={},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST",
        url="http://m/api/v1/agents/runs",
        status_code=201,
        json={},
    )

    handler = MarmotCallbackHandler(client, name="explorer", model="gpt-4o")
    root = uuid4()
    tool_run = uuid4()
    handler.on_chain_start({}, {}, run_id=root, parent_run_id=None)
    handler.on_tool_start(
        {"name": "query_orders"},
        "select * from orders",
        run_id=tool_run,
        parent_run_id=root,
        metadata={"marmot_asset_mrn": "postgres://prod/sales/orders"},
    )
    handler.on_tool_end("ok", run_id=tool_run, parent_run_id=root)
    handler.on_chain_end({}, run_id=root, parent_run_id=None)

    import json as _json

    run_call = next(
        r for r in httpx_mock.get_requests()  # type: ignore[attr-defined]
        if r.url.path.endswith("/agents/runs")
    )
    body = _json.loads(run_call.read())
    assert body["agent_mrn"] == "marmot://langchain/agent/explorer"
    assert body["run_id"] == str(root)
    assert body["status"] == "success"
    assert body["model"] == "gpt-4o"
    assert len(body["tool_calls"]) == 1
    tc = body["tool_calls"][0]
    assert tc["tool_name"] == "query_orders"
    assert tc["target_mrn"] == "postgres://prod/sales/orders"
    assert tc["status"] == "success"
    assert "duration_ms" in tc


def test_handler_records_error_run(client: Client, httpx_mock: object) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Agent/LangChain/explorer",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT", url="http://m/api/v1/assets/agent-1", json={}
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST", url="http://m/api/v1/agents/runs", status_code=201, json={}
    )

    handler = MarmotCallbackHandler(client, name="explorer")
    root = uuid4()
    handler.on_chain_start({}, {}, run_id=root, parent_run_id=None)
    handler.on_chain_error(RuntimeError("boom"), run_id=root, parent_run_id=None)

    import json as _json

    run_call = next(
        r for r in httpx_mock.get_requests()  # type: ignore[attr-defined]
        if r.url.path.endswith("/agents/runs")
    )
    body = _json.loads(run_call.read())
    assert body["status"] == "error"
    assert "RuntimeError" in body["error"]


def test_handler_emits_declared_invokes_edges_at_registration(
    client: Client, httpx_mock: object
) -> None:
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="GET",
        url="http://m/api/v1/assets/lookup/Agent/LangChain/explorer",
        json={"id": "agent-1", "mrn": "marmot://langchain/agent/explorer"},
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="PUT", url="http://m/api/v1/assets/agent-1", json={}
    )
    httpx_mock.add_response(  # type: ignore[attr-defined]
        method="POST", url="http://m/api/v1/lineage/batch", json=[]
    )

    @marmot_tool(asset_mrn="postgres://p/s/orders")
    def query_orders(sql: str) -> str:
        """Run a SQL query."""
        return "ok"

    handler = MarmotCallbackHandler(client, name="explorer", tools=[query_orders])
    handler.on_chain_start({}, {}, run_id=uuid4(), parent_run_id=None)

    import json as _json

    batch_call = next(
        r for r in httpx_mock.get_requests()  # type: ignore[attr-defined]
        if r.url.path.endswith("/lineage/batch")
    )
    body = _json.loads(batch_call.read())
    assert body == [
        {
            "source": "marmot://langchain/agent/explorer",
            "target": "postgres://p/s/orders",
            "type": "AGENT_INVOKES",
        }
    ]


def test_marmot_tool_decorator_attaches_metadata() -> None:
    @marmot_tool(asset_mrn="postgres://p/s/orders")
    def query_orders(sql: str) -> str:
        """Run a SQL query."""
        return "ok"

    assert query_orders.metadata == {"marmot_asset_mrn": "postgres://p/s/orders"}
    assert query_orders.name == "query_orders"
