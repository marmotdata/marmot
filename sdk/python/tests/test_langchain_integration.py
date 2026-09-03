"""Tests for the LangChain integration.

Both halves depend on protocols from :mod:`marmot.integrations.catalog`, so the
tools are driven by a fake reader and the handler by a fake registry.
"""

from __future__ import annotations

from typing import Any
from uuid import uuid4

import pytest
from langchain_core.messages import AIMessage
from langchain_core.messages.ai import UsageMetadata
from langchain_core.outputs import ChatGeneration, Generation, LLMResult
from langchain_core.tools import StructuredTool

from marmot.generated import Asset, LineageEdge, LineageResponse, Result, SearchResponse
from marmot.integrations import AgentRunRecord, AgentSpec
from marmot.integrations.langchain import MarmotCallbackHandler, catalog_tools, marmot_tool

AGENT_MRN = "marmot://langchain/agent/orders-analyst"
ORDERS_MRN = "postgres://prod/sales/orders"


class _FakeReader:
    def __init__(self, *, asset: Asset | None = None) -> None:
        self.asset = asset or Asset(id="a1", mrn=ORDERS_MRN, name="orders")
        self.searched: list[tuple[str, int]] = []

    def search(self, query: str, *, limit: int = 20) -> SearchResponse:
        self.searched.append((query, limit))
        return SearchResponse(
            total=1,
            results=[
                Result(
                    id="a1",
                    name="orders",
                    description="order facts",
                    metadata={"type": "Table", "primary_provider": "postgres", "mrn": ORDERS_MRN},
                )
            ],
        )

    def get_asset(self, asset_id: str) -> Asset:
        return self.asset

    def lookup_asset(self, *, asset_type: str, service: str, name: str) -> Asset | None:
        return self.asset if name == "orders" else None

    def upstream_lineage(self, asset_id: str, *, depth: int = 2) -> LineageResponse:
        return LineageResponse(edges=[LineageEdge(source=ORDERS_MRN, target="mrn://x")])


class _FakeRegistry:
    def __init__(self) -> None:
        self.specs: list[AgentSpec] = []
        self.runs: list[AgentRunRecord] = []
        self.edges: list[LineageEdge] = []

    def register_agent(self, spec: AgentSpec) -> Asset:
        self.specs.append(spec)
        return Asset(id="agent-1", mrn=AGENT_MRN)

    def record_run(self, run: AgentRunRecord) -> None:
        self.runs.append(run)

    def write_edges(self, edges: Any) -> None:
        self.edges.extend(edges)


@pytest.fixture
def reader() -> _FakeReader:
    return _FakeReader()


@pytest.fixture
def registry() -> _FakeRegistry:
    return _FakeRegistry()


def _tools_by_name(reader: _FakeReader) -> dict[str, StructuredTool]:
    return {tool.name: tool for tool in catalog_tools(reader)}


def test_catalog_tools_exposes_the_read_surface(reader: _FakeReader) -> None:
    tools = _tools_by_name(reader)
    assert set(tools) == {"search_catalog", "get_asset", "lookup_asset", "get_upstream_lineage"}


def test_only_lookup_tools_opt_into_lineage_recording(reader: _FakeReader) -> None:
    """search returns candidates, so mining its output would invent edges."""
    tools = _tools_by_name(reader)
    assert not (tools["search_catalog"].metadata or {}).get("marmot_record_lookups")
    for name in ("get_asset", "lookup_asset", "get_upstream_lineage"):
        assert tools[name].metadata["marmot_record_lookups"] is True


def test_search_tool_flattens_results(reader: _FakeReader) -> None:
    out = _tools_by_name(reader)["search_catalog"].invoke({"query": "orders", "limit": 5})
    assert reader.searched == [("orders", 5)]
    assert out["total"] == 1
    assert out["results"] == [
        {
            "id": "a1",
            "name": "orders",
            "type": "Table",
            "provider": "postgres",
            "mrn": ORDERS_MRN,
            "description": "order facts",
        }
    ]


def test_lookup_tool_returns_none_when_absent(reader: _FakeReader) -> None:
    tool = _tools_by_name(reader)["lookup_asset"]
    found = tool.invoke({"asset_type": "Table", "service": "postgres", "name": "orders"})
    assert found["mrn"] == ORDERS_MRN
    assert tool.invoke({"asset_type": "Table", "service": "postgres", "name": "nope"}) is None


def test_handler_registers_agent_on_first_chain_start(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst", model="claude-opus-4-7")
    handler.on_chain_start({}, {}, run_id=uuid4())

    assert handler.agent_mrn == AGENT_MRN
    assert len(registry.specs) == 1
    metadata = registry.specs[0].metadata()
    assert metadata["framework"] == "LangChain"
    assert metadata["model"] == "claude-opus-4-7"


def test_handler_emits_declared_edges_for_tagged_tools(registry: _FakeRegistry) -> None:
    @marmot_tool(asset_mrn=ORDERS_MRN)
    def query_orders(sql: str) -> str:
        """Run a read-only query."""
        return "rows"

    handler = MarmotCallbackHandler(registry, name="orders-analyst", tools=[query_orders])
    handler.on_chain_start({}, {}, run_id=uuid4())

    assert [(e.source, e.target, e.type) for e in registry.edges] == [
        (AGENT_MRN, ORDERS_MRN, "AGENT_INVOKES")
    ]
    assert registry.specs[0].tool_names == ["query_orders"]


def test_handler_records_run_with_tool_calls(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root, tool_run = uuid4(), uuid4()

    handler.on_chain_start({}, {}, run_id=root)
    handler.on_tool_start(
        {"name": "query_orders"},
        "sql",
        run_id=tool_run,
        parent_run_id=root,
        metadata={"marmot_asset_mrn": ORDERS_MRN},
    )
    handler.on_tool_end("rows", run_id=tool_run, parent_run_id=root)
    handler.on_chain_end({}, run_id=root)

    assert len(registry.runs) == 1
    run = registry.runs[0]
    assert run.agent_mrn == AGENT_MRN
    assert run.run_id == str(root)
    assert run.status == "success"
    assert [(c.tool_name, c.target_mrn, c.status) for c in run.tool_calls] == [
        ("query_orders", ORDERS_MRN, "success")
    ]
    # the MRN is already attributed via the tool call, so it is not repeated
    assert run.observed_assets == []


def test_handler_mines_lookup_tool_output_for_observed_assets(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root, tool_run = uuid4(), uuid4()

    handler.on_chain_start({}, {}, run_id=root)
    handler.on_tool_start(
        {"name": "get_asset"},
        "a1",
        run_id=tool_run,
        parent_run_id=root,
        metadata={"marmot_record_lookups": True},
    )
    handler.on_tool_end({"mrn": ORDERS_MRN}, run_id=tool_run, parent_run_id=root)
    handler.on_chain_end({}, run_id=root)

    assert registry.runs[0].observed_assets == [ORDERS_MRN]


def test_handler_records_error_status_on_chain_error(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root = uuid4()

    handler.on_chain_start({}, {}, run_id=root)
    handler.on_chain_error(ValueError("boom"), run_id=root)

    run = registry.runs[0]
    assert run.status == "error"
    assert run.error == "ValueError: boom"


def _llm_result_with_usage_metadata(prompt: int, completion: int) -> LLMResult:
    """What a modern chat model reports: provider-agnostic usage_metadata."""
    message = AIMessage(
        content="ok",
        usage_metadata=UsageMetadata(
            input_tokens=prompt, output_tokens=completion, total_tokens=prompt + completion
        ),
    )
    return LLMResult(generations=[[ChatGeneration(message=message)]])


def test_handler_counts_tokens_across_llm_calls(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root = uuid4()

    handler.on_chain_start({}, {}, run_id=root)
    handler.on_llm_end(_llm_result_with_usage_metadata(11, 5), run_id=uuid4(), parent_run_id=root)
    handler.on_llm_end(_llm_result_with_usage_metadata(11, 5), run_id=uuid4(), parent_run_id=root)
    handler.on_chain_end({}, run_id=root)

    assert (registry.runs[0].tokens_in, registry.runs[0].tokens_out) == (22, 10)


def test_handler_falls_back_to_llm_output_token_usage(registry: _FakeRegistry) -> None:
    """Providers that predate usage_metadata only fill llm_output."""
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root = uuid4()
    result = LLMResult(
        generations=[[Generation(text="ok")]],
        llm_output={"token_usage": {"prompt_tokens": 7, "completion_tokens": 3}},
    )

    handler.on_chain_start({}, {}, run_id=root)
    handler.on_llm_end(result, run_id=uuid4(), parent_run_id=root)
    handler.on_chain_end({}, run_id=root)

    assert (registry.runs[0].tokens_in, registry.runs[0].tokens_out) == (7, 3)


def test_record_source_attributes_a_runtime_mrn(registry: _FakeRegistry) -> None:
    handler = MarmotCallbackHandler(registry, name="orders-analyst")
    root = uuid4()

    handler.on_chain_start({}, {}, run_id=root)
    handler.record_source("s3://bucket/key.parquet", run_id=root)
    handler.on_chain_end({}, run_id=root)

    assert registry.runs[0].observed_assets == ["s3://bucket/key.parquet"]
