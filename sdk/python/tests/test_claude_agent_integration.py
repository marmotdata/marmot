"""Tests for the Claude Agent SDK integration.

Hooks are driven synchronously via ``anyio.run`` so we don't need a separate
async test plugin.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import anyio
import httpx
import pytest

from marmot.auth import Credential
from marmot.client import Client
from marmot.integrations.claude_agent import MarmotAgentTracker
from marmot.integrations.claude_agent._transcript import summarize_transcript


@pytest.fixture
def client(httpx_mock: object) -> Client:
    cred = Credential(token="test-key", scheme="X-API-Key", source="test")
    return Client(base_url="http://m", credential=cred, http_client=httpx.Client())


def _agent_lookup_url(name: str = "explorer", service: str = "ClaudeAgent") -> str:
    return f"http://m/api/v1/assets/lookup/Agent/{service}/{name}"


def _mock_runs(httpx_mock: Any, sink: list[dict[str, Any]] | None = None) -> None:
    """Register a permissive callback for POST /agents/runs.

    The Stop hook always posts a run record now, so every test that drives
    Stop needs this mock. Optionally captures bodies into ``sink``.
    """

    def on_post(request: httpx.Request) -> httpx.Response:
        if sink is not None:
            sink.append(json.loads(request.content))
        return httpx.Response(
            201,
            json={
                "id": "run-1",
                "agent_id": "agent-1",
                "run_id": "x",
                "started_at": "2026-01-01T00:00:00Z",
                "status": "success",
                "tokens_in": 0,
                "tokens_out": 0,
                "created_at": "2026-01-01T00:00:00Z",
            },
        )

    httpx_mock.add_callback(on_post, method="POST", url="http://m/api/v1/agents/runs")


def test_hooks_returns_lifecycle_events(client: Client) -> None:
    tracker = MarmotAgentTracker(client, name="explorer")
    hooks = tracker.hooks()
    assert set(hooks.keys()) == {"PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop"}
    for event in hooks:
        matchers = hooks[event]
        assert len(matchers) == 1
        assert len(matchers[0].hooks) == 1


def test_registers_on_first_hook_and_writes_lineage_on_stop(
    client: Client, httpx_mock: Any
) -> None:
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    batch_seen: list[dict[str, Any]] = []

    def on_batch(request: httpx.Request) -> httpx.Response:
        batch_seen.extend(json.loads(request.content))
        return httpx.Response(200, json=[])

    httpx_mock.add_callback(on_batch, method="POST", url="http://m/api/v1/lineage/batch")
    _mock_runs(httpx_mock)

    tracker = MarmotAgentTracker(
        client, name="explorer", model="claude-sonnet-4-5", owner="data-eng"
    )
    hooks = tracker.hooks()
    pre = hooks["PreToolUse"][0].hooks[0]
    post = hooks["PostToolUse"][0].hooks[0]
    stop = hooks["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre({"hook_event_name": "PreToolUse", "session_id": "s1"}, None, {})
        await post(
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s1",
                "tool_name": "mcp__marmot__discover_data",
                "tool_response": {
                    "results": [
                        {"id": "x", "mrn": "postgres://p/s/orders"},
                        {"id": "y", "mrn": "kafka://c/orders.events"},
                    ]
                },
            },
            None,
            {},
        )
        await stop({"hook_event_name": "Stop", "session_id": "s1"}, None, {})

    anyio.run(drive)
    assert tracker.agent_mrn == "marmot://claude/agent/explorer"
    sources = sorted(e["source"] for e in batch_seen)
    assert sources == ["kafka://c/orders.events", "postgres://p/s/orders"]
    for e in batch_seen:
        assert e["target"] == "marmot://claude/agent/explorer"


def test_post_tool_use_registers_when_pre_was_skipped(client: Client, httpx_mock: Any) -> None:
    """Python parity path — register on first PostToolUse if PreToolUse never fired."""
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    batch_seen: list[dict[str, Any]] = []
    httpx_mock.add_callback(
        lambda req: batch_seen.extend(json.loads(req.content)) or httpx.Response(200, json=[]),
        method="POST",
        url="http://m/api/v1/lineage/batch",
    )
    _mock_runs(httpx_mock)

    tracker = MarmotAgentTracker(client, name="explorer")
    hooks = tracker.hooks()
    post = hooks["PostToolUse"][0].hooks[0]
    stop = hooks["Stop"][0].hooks[0]

    async def drive() -> None:
        await post(
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s2",
                "tool_name": "mcp__marmot__lookup_term",
                "tool_response": {"mrn": "postgres://p/s/orders"},
            },
            None,
            {},
        )
        await stop({"hook_event_name": "Stop", "session_id": "s2"}, None, {})

    anyio.run(drive)
    assert tracker.agent_mrn == "marmot://claude/agent/explorer"
    assert batch_seen[0]["source"] == "postgres://p/s/orders"


def test_record_source_lets_custom_tool_attribute_runtime_mrn(
    client: Client, httpx_mock: Any
) -> None:
    httpx_mock.add_response(
        method="GET",
        url=_agent_lookup_url(),
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    httpx_mock.add_response(method="PUT", url="http://m/api/v1/assets/agent-1", json={})
    batch_seen: list[dict[str, Any]] = []
    httpx_mock.add_callback(
        lambda req: batch_seen.extend(json.loads(req.content)) or httpx.Response(200, json=[]),
        method="POST",
        url="http://m/api/v1/lineage/batch",
    )
    _mock_runs(httpx_mock)

    tracker = MarmotAgentTracker(client, name="explorer")
    pre = tracker.hooks()["PreToolUse"][0].hooks[0]
    stop = tracker.hooks()["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre({"hook_event_name": "PreToolUse", "session_id": "s3"}, None, {})
        tracker.record_source("s3://bucket/key.parquet", "s3")
        await stop({"hook_event_name": "Stop", "session_id": "s3"}, None, {})

    anyio.run(drive)
    assert batch_seen[0]["source"] == "s3://bucket/key.parquet"


def test_upserts_when_agent_asset_already_exists(client: Client, httpx_mock: Any) -> None:
    httpx_mock.add_response(
        method="GET",
        url=_agent_lookup_url(),
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    update_bodies: list[dict[str, Any]] = []
    httpx_mock.add_callback(
        lambda req: update_bodies.append(json.loads(req.content)) or httpx.Response(200, json={}),
        method="PUT",
        url="http://m/api/v1/assets/agent-1",
    )

    tracker = MarmotAgentTracker(client, name="explorer", model="claude-sonnet-4-5")

    async def drive() -> None:
        await tracker.register()

    anyio.run(drive)
    assert tracker.agent_mrn == "marmot://claude/agent/explorer"
    assert len(update_bodies) == 1
    assert update_bodies[0]["metadata"]["framework"] == "ClaudeAgent"
    assert update_bodies[0]["metadata"]["model"] == "claude-sonnet-4-5"


def test_concurrent_register_calls_only_upsert_once(client: Client, httpx_mock: Any) -> None:
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    post_count = 0

    def on_post(_req: httpx.Request) -> httpx.Response:
        nonlocal post_count
        post_count += 1
        return httpx.Response(201, json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"})

    httpx_mock.add_callback(on_post, method="POST", url="http://m/api/v1/assets")

    tracker = MarmotAgentTracker(client, name="explorer")

    async def drive() -> None:
        async with anyio.create_task_group() as tg:
            for _ in range(3):
                tg.start_soon(tracker.register)

    anyio.run(drive)
    assert post_count == 1


def test_stop_with_no_upstreams_skips_lineage_call(client: Client, httpx_mock: Any) -> None:
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    _mock_runs(httpx_mock)

    tracker = MarmotAgentTracker(client, name="explorer")
    pre = tracker.hooks()["PreToolUse"][0].hooks[0]
    stop = tracker.hooks()["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre({"hook_event_name": "PreToolUse", "session_id": "s4"}, None, {})
        await stop({"hook_event_name": "Stop", "session_id": "s4"}, None, {})

    anyio.run(drive)
    # No /lineage/batch mock registered above → would 404 if called. Reaching
    # here without an error means the tracker correctly skipped the call.


def test_captures_mrns_from_mcp_content_text_envelopes(client: Client, httpx_mock: Any) -> None:
    """Real Marmot MCP response shape — markdown text with backtick-quoted MRNs
    alongside http UI links that must be ignored."""
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    batch_seen: list[dict[str, Any]] = []
    httpx_mock.add_callback(
        lambda req: batch_seen.extend(json.loads(req.content)) or httpx.Response(200, json=[]),
        method="POST",
        url="http://m/api/v1/lineage/batch",
    )
    _mock_runs(httpx_mock)

    tracker = MarmotAgentTracker(client, name="explorer")
    hooks = tracker.hooks()
    post = hooks["PostToolUse"][0].hooks[0]
    stop = hooks["Stop"][0].hooks[0]

    async def drive() -> None:
        await post(
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s5",
                "tool_name": "mcp__marmot__discover_data",
                "tool_response": {
                    "content": [
                        {
                            "type": "text",
                            "text": (
                                "# Found 2 assets\n\n"
                                "- [orders-search](http://localhost:5173/discover/index/orders-search)"
                                " · `mrn://index/elasticsearch/orders-search` · elasticsearch\n"
                                "- [PARTNER_ORDERS](http://localhost:5173/discover/table/PARTNER_ORDERS)"
                                " · `mrn://table/snowflake/glacier.partner.partner_orders` · snowflake\n"
                            ),
                        }
                    ]
                },
            },
            None,
            {},
        )
        await stop({"hook_event_name": "Stop", "session_id": "s5"}, None, {})

    anyio.run(drive)
    sources = sorted(e["source"] for e in batch_seen)
    assert sources == [
        "mrn://index/elasticsearch/orders-search",
        "mrn://table/snowflake/glacier.partner.partner_orders",
    ]


def test_stop_posts_agent_run_with_per_tool_timing(client: Client, httpx_mock: Any) -> None:
    """End-to-end: tool timing + status flow through to agent_runs POST body."""
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    httpx_mock.add_response(method="POST", url="http://m/api/v1/lineage/batch", json=[])
    runs_seen: list[dict[str, Any]] = []
    _mock_runs(httpx_mock, runs_seen)

    tracker = MarmotAgentTracker(client, name="explorer", model="claude-sonnet-4-5")
    hooks = tracker.hooks()
    pre = hooks["PreToolUse"][0].hooks[0]
    post = hooks["PostToolUse"][0].hooks[0]
    stop = hooks["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre(
            {
                "hook_event_name": "PreToolUse",
                "session_id": "s-run",
                "tool_name": "mcp__marmot__discover_data",
            },
            "tool-call-1",
            {},
        )
        await post(
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s-run",
                "tool_name": "mcp__marmot__discover_data",
                "tool_response": {"mrn": "postgres://p/s/orders"},
            },
            "tool-call-1",
            {},
        )
        await stop({"hook_event_name": "Stop", "session_id": "s-run"}, None, {})

    anyio.run(drive)
    assert len(runs_seen) == 1
    body = runs_seen[0]
    assert body["agent_mrn"] == "marmot://claude/agent/explorer"
    assert body["run_id"] == "s-run"
    assert body["status"] == "success"
    assert body["model"] == "claude-sonnet-4-5"
    assert body["tokens_in"] == 0  # no transcript_path → no token data
    assert body["tokens_out"] == 0
    assert len(body["tool_calls"]) == 1
    tc = body["tool_calls"][0]
    assert tc["tool_name"] == "mcp__marmot__discover_data"
    assert tc["status"] == "success"
    assert tc["target_mrn"] == "postgres://p/s/orders"
    assert tc.get("duration_ms") is not None
    assert tc["duration_ms"] >= 0


def test_post_tool_use_failure_marks_run_as_error(client: Client, httpx_mock: Any) -> None:
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    runs_seen: list[dict[str, Any]] = []
    _mock_runs(httpx_mock, runs_seen)

    tracker = MarmotAgentTracker(client, name="explorer")
    hooks = tracker.hooks()
    pre = hooks["PreToolUse"][0].hooks[0]
    fail = hooks["PostToolUseFailure"][0].hooks[0]
    stop = hooks["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre(
            {"hook_event_name": "PreToolUse", "session_id": "s-err", "tool_name": "broken_tool"},
            "tc-err",
            {},
        )
        await fail(
            {
                "hook_event_name": "PostToolUseFailure",
                "session_id": "s-err",
                "tool_name": "broken_tool",
                "error": "permission denied",
            },
            "tc-err",
            {},
        )
        await stop({"hook_event_name": "Stop", "session_id": "s-err"}, None, {})

    anyio.run(drive)
    assert runs_seen[0]["status"] == "error"
    assert runs_seen[0]["error"] == "permission denied"
    assert runs_seen[0]["tool_calls"][0]["status"] == "error"


def test_stop_reads_transcript_for_tokens(client: Client, httpx_mock: Any, tmp_path: Path) -> None:
    """When transcript_path is present, tokens land in the agent_runs body."""
    httpx_mock.add_response(
        method="GET", url=_agent_lookup_url(), status_code=404, json={"error": "not found"}
    )
    httpx_mock.add_response(
        method="POST",
        url="http://m/api/v1/assets",
        status_code=201,
        json={"id": "agent-1", "mrn": "marmot://claude/agent/explorer"},
    )
    runs_seen: list[dict[str, Any]] = []
    _mock_runs(httpx_mock, runs_seen)

    transcript = tmp_path / "session.jsonl"
    transcript.write_text(
        "\n".join(
            [
                json.dumps(
                    {
                        "type": "assistant",
                        "timestamp": "2026-05-28T10:00:00.000Z",
                        "message": {
                            "usage": {
                                "input_tokens": 100,
                                "cache_creation_input_tokens": 200,
                                "cache_read_input_tokens": 50,
                                "output_tokens": 80,
                            }
                        },
                    }
                ),
                json.dumps(
                    {
                        "type": "assistant",
                        "timestamp": "2026-05-28T10:00:05.500Z",
                        "message": {"usage": {"input_tokens": 10, "output_tokens": 30}},
                    }
                ),
            ]
        )
    )

    tracker = MarmotAgentTracker(client, name="explorer")
    pre = tracker.hooks()["PreToolUse"][0].hooks[0]
    stop = tracker.hooks()["Stop"][0].hooks[0]

    async def drive() -> None:
        await pre(
            {
                "hook_event_name": "PreToolUse",
                "session_id": "s-tx",
                "tool_name": "noop",
                "transcript_path": str(transcript),
            },
            "t1",
            {},
        )
        await stop(
            {
                "hook_event_name": "Stop",
                "session_id": "s-tx",
                "transcript_path": str(transcript),
            },
            None,
            {},
        )

    anyio.run(drive)
    body = runs_seen[0]
    assert body["tokens_in"] == 100 + 200 + 50 + 10
    assert body["tokens_out"] == 80 + 30


def test_summarize_transcript_returns_none_for_missing_file(tmp_path: Path) -> None:
    assert summarize_transcript(tmp_path / "nope.jsonl") is None


def test_summarize_transcript_skips_malformed_lines(tmp_path: Path) -> None:
    p = tmp_path / "tx.jsonl"
    p.write_text(
        "\n".join(
            [
                "not json at all",
                json.dumps({"type": "user", "timestamp": "2026-05-28T10:00:00Z"}),
                json.dumps(
                    {
                        "type": "assistant",
                        "timestamp": "2026-05-28T10:00:01Z",
                        "message": {"usage": {"input_tokens": 5, "output_tokens": 7}},
                    }
                ),
                "",
            ]
        )
    )
    summary = summarize_transcript(p)
    assert summary is not None
    assert summary.tokens_in == 5
    assert summary.tokens_out == 7
    assert summary.started_at is not None
    assert summary.ended_at is not None
