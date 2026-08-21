"""Tests for the Claude Agent SDK integration.

The tracker depends on the :class:`AgentRegistry` protocol, so these drive a
fake registry instead of mocking HTTP: what matters here is which runs, edges
and specs the tracker produces, not how the client serialises them.

Hooks are driven synchronously via ``anyio.run`` so we don't need a separate
async test plugin.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import anyio
import pytest

from marmot.generated import Asset, LineageEdge
from marmot.integrations import AgentRunRecord, AgentSpec
from marmot.integrations.claude_agent import MarmotAgentTracker
from marmot.integrations.claude_agent.transcript import summarize_transcript

AGENT_MRN = "marmot://claude/agent/explorer"


class _FakeRegistry:
    """Captures what the tracker reports, and returns a stable agent asset."""

    def __init__(self, *, agent_mrn: str = AGENT_MRN) -> None:
        self.agent_mrn = agent_mrn
        self.specs: list[AgentSpec] = []
        self.runs: list[AgentRunRecord] = []
        self.edges: list[LineageEdge] = []

    def register_agent(self, spec: AgentSpec) -> Asset:
        self.specs.append(spec)
        return Asset(id="agent-1", mrn=self.agent_mrn)

    def record_run(self, run: AgentRunRecord) -> None:
        self.runs.append(run)

    def write_edges(self, edges: Any) -> None:
        self.edges.extend(edges)


@pytest.fixture
def registry() -> _FakeRegistry:
    return _FakeRegistry()


def _hooks(tracker: MarmotAgentTracker) -> dict[str, Any]:
    hooks = tracker.hooks()
    return {name: matchers[0].hooks[0] for name, matchers in hooks.items()}


def test_hooks_returns_lifecycle_events(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer")
    assert set(tracker.hooks()) == {
        "PreToolUse",
        "PostToolUse",
        "PostToolUseFailure",
        "Stop",
    }


def test_registers_on_first_hook_and_writes_lineage_on_stop(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(
        registry, name="explorer", model="claude-sonnet-4-5", owner="data-eng"
    )
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"]({"hook_event_name": "PreToolUse", "session_id": "s1"}, None, {})
        await hooks["PostToolUse"](
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
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s1"}, None, {})

    anyio.run(drive)
    assert tracker.agent_mrn == AGENT_MRN
    assert sorted(edge.source for edge in registry.edges) == [
        "kafka://c/orders.events",
        "postgres://p/s/orders",
    ]
    assert {edge.target for edge in registry.edges} == {AGENT_MRN}


def test_post_tool_use_registers_when_pre_was_skipped(registry: _FakeRegistry) -> None:
    """Python parity path — register on first PostToolUse if PreToolUse never fired."""
    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PostToolUse"](
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s2",
                "tool_name": "mcp__marmot__lookup_term",
                "tool_response": {"mrn": "postgres://p/s/orders"},
            },
            None,
            {},
        )
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s2"}, None, {})

    anyio.run(drive)
    assert tracker.agent_mrn == AGENT_MRN
    assert registry.edges[0].source == "postgres://p/s/orders"


def test_record_source_lets_custom_tool_attribute_runtime_mrn(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"]({"hook_event_name": "PreToolUse", "session_id": "s3"}, None, {})
        tracker.record_source("s3://bucket/key.parquet", "s3")
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s3"}, None, {})

    anyio.run(drive)
    assert registry.edges[0].source == "s3://bucket/key.parquet"


def test_registers_agent_with_describing_metadata(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer", model="claude-sonnet-4-5")

    anyio.run(tracker.register)

    assert tracker.agent_mrn == AGENT_MRN
    assert len(registry.specs) == 1
    metadata = registry.specs[0].metadata()
    assert metadata["framework"] == "ClaudeAgent"
    assert metadata["model"] == "claude-sonnet-4-5"


def test_concurrent_register_calls_only_upsert_once(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer")

    async def drive() -> None:
        async with anyio.create_task_group() as group:
            for _ in range(3):
                group.start_soon(tracker.register)

    anyio.run(drive)
    assert len(registry.specs) == 1


def test_stop_with_no_upstreams_skips_lineage_call(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"]({"hook_event_name": "PreToolUse", "session_id": "s4"}, None, {})
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s4"}, None, {})

    anyio.run(drive)
    assert registry.edges == []
    assert len(registry.runs) == 1


def test_captures_mrns_from_mcp_content_text_envelopes(registry: _FakeRegistry) -> None:
    """Real Marmot MCP response shape — markdown text with backtick-quoted MRNs
    alongside http UI links that must be ignored."""
    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PostToolUse"](
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
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s5"}, None, {})

    anyio.run(drive)
    assert sorted(edge.source for edge in registry.edges) == [
        "mrn://index/elasticsearch/orders-search",
        "mrn://table/snowflake/glacier.partner.partner_orders",
    ]


def test_stop_posts_agent_run_with_per_tool_timing(registry: _FakeRegistry) -> None:
    """End-to-end: tool timing + status flow through to the recorded run."""
    tracker = MarmotAgentTracker(registry, name="explorer", model="claude-sonnet-4-5")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"](
            {
                "hook_event_name": "PreToolUse",
                "session_id": "s-run",
                "tool_name": "mcp__marmot__discover_data",
            },
            "tool-call-1",
            {},
        )
        await hooks["PostToolUse"](
            {
                "hook_event_name": "PostToolUse",
                "session_id": "s-run",
                "tool_name": "mcp__marmot__discover_data",
                "tool_response": {"mrn": "postgres://p/s/orders"},
            },
            "tool-call-1",
            {},
        )
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s-run"}, None, {})

    anyio.run(drive)
    assert len(registry.runs) == 1
    run = registry.runs[0]
    assert run.agent_mrn == AGENT_MRN
    assert run.run_id == "s-run"
    assert run.status == "success"
    assert run.model == "claude-sonnet-4-5"
    assert run.tokens_in == 0  # no transcript_path → no token data
    assert run.tokens_out == 0
    assert len(run.tool_calls) == 1
    call = run.tool_calls[0]
    assert call.tool_name == "mcp__marmot__discover_data"
    assert call.status == "success"
    assert call.target_mrn == "postgres://p/s/orders"
    assert call.duration_ms is not None
    assert call.duration_ms >= 0


def test_post_tool_use_failure_marks_run_as_error(registry: _FakeRegistry) -> None:
    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"](
            {"hook_event_name": "PreToolUse", "session_id": "s-err", "tool_name": "broken_tool"},
            "tc-err",
            {},
        )
        await hooks["PostToolUseFailure"](
            {
                "hook_event_name": "PostToolUseFailure",
                "session_id": "s-err",
                "tool_name": "broken_tool",
                "error": "permission denied",
            },
            "tc-err",
            {},
        )
        await hooks["Stop"]({"hook_event_name": "Stop", "session_id": "s-err"}, None, {})

    anyio.run(drive)
    assert registry.runs[0].status == "error"
    assert registry.runs[0].error == "permission denied"
    assert registry.runs[0].tool_calls[0].status == "error"


def test_stop_reads_transcript_for_tokens(registry: _FakeRegistry, tmp_path: Path) -> None:
    """When transcript_path is present, tokens land in the recorded run."""
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

    tracker = MarmotAgentTracker(registry, name="explorer")
    hooks = _hooks(tracker)

    async def drive() -> None:
        await hooks["PreToolUse"](
            {
                "hook_event_name": "PreToolUse",
                "session_id": "s-tx",
                "tool_name": "noop",
                "transcript_path": str(transcript),
            },
            "t1",
            {},
        )
        await hooks["Stop"](
            {
                "hook_event_name": "Stop",
                "session_id": "s-tx",
                "transcript_path": str(transcript),
            },
            None,
            {},
        )

    anyio.run(drive)
    run = registry.runs[0]
    assert run.tokens_in == 100 + 200 + 50 + 10
    assert run.tokens_out == 80 + 30


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
