"""Tracker that registers a Claude Agent SDK agent in Marmot and writes
lineage edges plus per-run telemetry for the data sources it touches."""

from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Any, cast

from marmot.generated import LineageEdge
from marmot.integrations.catalog import AgentRunRecord, AgentSpec, ToolCall
from marmot.integrations.claude_agent.transcript import (
    TranscriptSummary,
    summarize_transcript,
)
from marmot.integrations.shared import extract_mrns, sha256_hex

if TYPE_CHECKING:
    from claude_agent_sdk import (
        BaseHookInput,
        HookInput,
        HookJSONOutput,
        PostToolUseFailureHookInput,
        PostToolUseHookInput,
        PreToolUseHookInput,
    )

    from marmot.integrations.catalog import AgentRegistry


_LOG = logging.getLogger("marmot.integrations.claude_agent")

_DEFAULT_SERVICE = "ClaudeAgent"
_FRAMEWORK = "ClaudeAgent"


@dataclass
class _ToolOpen:
    tool_name: str
    started_at: datetime


@dataclass
class _RunState:
    started_at: datetime
    transcript_path: str | None = None
    upstreams: set[str] = field(default_factory=set)
    tool_calls: list[ToolCall] = field(default_factory=list)
    tool_open: dict[str, _ToolOpen] = field(default_factory=dict)
    error: str | None = None


class MarmotAgentTracker:
    """Auto-registers a Claude Agent SDK agent as a Marmot ``Agent`` asset
    and captures, per session, lineage edges, per-tool timing, and token
    usage from the on-disk transcript.

    Pass the result of :meth:`hooks` to
    :class:`claude_agent_sdk.ClaudeAgentOptions`::

        import asyncio
        from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions
        import marmot
        from marmot.integrations import MarmotCatalog
        from marmot.integrations.claude_agent import MarmotAgentTracker

        async def main():
            catalog = MarmotCatalog(marmot.AuthenticatedApiClient.connect())
            tracker = MarmotAgentTracker(
                catalog,
                name="catalog-explorer",
                model="claude-sonnet-4-5",
            )
            options = ClaudeAgentOptions(
                mcp_servers={"marmot": {...}},
                hooks=tracker.hooks(),
            )
            async with ClaudeSDKClient(options=options) as agent:
                await agent.query("Find orders data")
                async for _ in agent.receive_response():
                    pass
            print(tracker.agent_mrn)

    Python's claude-agent-sdk has no ``SessionStart`` hook, so the tracker
    registers lazily on the first ``PreToolUse`` / ``PostToolUse`` and starts
    the run clock there. If your agent never calls a tool, call
    :meth:`register` explicitly. State is bucketed per ``session_id`` so a
    long-lived tracker handling sequential queries keeps each session's
    telemetry isolated.

    On ``Stop`` the tracker:

    1. reads the JSONL transcript at ``transcript_path`` for real token
       totals and wall-clock bounds (no opt-in needed);
    2. POSTs one ``agent_runs`` record with hook-captured per-tool timing
       and transcript-derived tokens/latency;
    3. POSTs a single batched lineage call with the observed upstream MRNs.

    Transcript reads are best-effort: a missing or malformed file leaves the
    run record populated with hook-derived data only (tokens = 0).
    """

    def __init__(
        self,
        catalog: AgentRegistry,
        *,
        name: str,
        service: str = _DEFAULT_SERVICE,
        model: str | None = None,
        version: str | None = None,
        owner: str | None = None,
        system_prompt: str | None = None,
        extra_metadata: dict[str, Any] | None = None,
    ) -> None:
        self._catalog = catalog
        self._spec = AgentSpec(
            name=name,
            service=service,
            framework=_FRAMEWORK,
            model=model,
            version=version,
            owner=owner,
            system_prompt_hash=sha256_hex(system_prompt)[:16] if system_prompt else None,
            extra_metadata=extra_metadata or {},
        )

        self._agent_mrn: str | None = None
        self._agent_id: str | None = None
        self._register_lock = asyncio.Lock()
        self._runs: dict[str, _RunState] = {}
        self._fallback_run: _RunState | None = None

    @property
    def agent_mrn(self) -> str | None:
        """MRN of the registered agent asset, once it has been upserted."""
        return self._agent_mrn

    def record_source(self, mrn: str, session_id: str | None = None) -> None:
        """Manually record an upstream MRN. Use from a custom tool when the
        tool's output doesn't include an ``mrn`` field or a recognisable
        ``<scheme>://`` URI."""
        self._run_state(session_id).upstreams.add(mrn)

    def hooks(self) -> dict[str, list[Any]]:
        """Return a hook map suitable for ``ClaudeAgentOptions.hooks``.

        Wraps the tracker's lifecycle callbacks in ``HookMatcher`` so they
        fire on every PreToolUse/PostToolUse/PostToolUseFailure/Stop event
        regardless of tool name.
        """
        try:
            from claude_agent_sdk import HookMatcher
        except ImportError as e:
            raise ImportError(
                "claude-agent-sdk is required for MarmotAgentTracker.hooks(). "
                "Install via `pip install marmot-sdk[claude-agent]`."
            ) from e
        return {
            "PreToolUse": [HookMatcher(hooks=[self._on_pre_tool_use])],
            "PostToolUse": [HookMatcher(hooks=[self._on_post_tool_use])],
            "PostToolUseFailure": [HookMatcher(hooks=[self._on_post_tool_use_failure])],
            "Stop": [HookMatcher(hooks=[self._on_stop])],
        }

    async def register(self) -> None:
        """Manually upsert the Agent asset. Normally called automatically on
        the first hook invocation; call directly when your flow can't
        guarantee a hook will fire (e.g. an agent that never calls a tool).
        """
        await self._ensure_registered()

    async def flush(self, session_id: str | None = None) -> None:
        """Flush the pending run for ``session_id`` (or the fallback bucket).

        Posts the run record and lineage edges, then clears state. Called
        automatically on the ``Stop`` hook; safe to call manually if needed.
        """
        run = self._take_run(session_id)
        if run is None:
            return
        await self._ensure_registered()
        if self._agent_mrn is None:
            return

        summary = summarize_transcript(run.transcript_path) if run.transcript_path else None
        ended_at = datetime.now(timezone.utc)
        await asyncio.to_thread(self._post_run, session_id, run, summary, ended_at)

        if run.upstreams:
            edges = [
                LineageEdge(source=mrn, target=self._agent_mrn) for mrn in sorted(run.upstreams)
            ]
            try:
                await asyncio.to_thread(self._catalog.write_edges, edges)
            except Exception as e:
                _LOG.warning("failed to write lineage: %s", e)

    # ------------------------------------------------------------------
    # Hook callbacks

    async def _on_pre_tool_use(
        self, input_data: HookInput, tool_use_id: str | None, context: Any
    ) -> HookJSONOutput:
        event = cast("PreToolUseHookInput", input_data)
        await self._ensure_registered()
        state = self._run_state(_session_id_of(event))
        _capture_transcript_path(state, event)
        name = event.get("tool_name")
        if isinstance(tool_use_id, str) and tool_use_id and name:
            state.tool_open[tool_use_id] = _ToolOpen(
                tool_name=name, started_at=datetime.now(timezone.utc)
            )
        return {}

    async def _on_post_tool_use(
        self, input_data: HookInput, tool_use_id: str | None, context: Any
    ) -> HookJSONOutput:
        event = cast("PostToolUseHookInput", input_data)
        await self._ensure_registered()
        state = self._run_state(_session_id_of(event))
        _capture_transcript_path(state, event)

        response = event.get("tool_response")
        observed: list[str] = []
        if response is not None:
            for mrn in extract_mrns(response):
                state.upstreams.add(mrn)
                observed.append(mrn)

        self._close_tool_call(
            state,
            event,
            tool_use_id,
            status="success",
            target_mrn=observed[0] if observed else None,
        )
        return {}

    async def _on_post_tool_use_failure(
        self, input_data: HookInput, tool_use_id: str | None, context: Any
    ) -> HookJSONOutput:
        event = cast("PostToolUseFailureHookInput", input_data)
        await self._ensure_registered()
        state = self._run_state(_session_id_of(event))
        _capture_transcript_path(state, event)
        if error := event.get("error"):
            state.error = error
        self._close_tool_call(state, event, tool_use_id, status="error", target_mrn=None)
        return {}

    async def _on_stop(
        self, input_data: HookInput, tool_use_id: str | None, context: Any
    ) -> HookJSONOutput:
        # Snapshot transcript path on the run before the take_run() in flush()
        # discards state — Stop is the last hook that carries it.
        state = self._run_state_optional(_session_id_of(input_data))
        if state is not None:
            _capture_transcript_path(state, input_data)
        await self.flush(_session_id_of(input_data))
        return {}

    # ------------------------------------------------------------------
    # Run-state helpers

    def _run_state(self, session_id: str | None) -> _RunState:
        if not session_id:
            if self._fallback_run is None:
                self._fallback_run = _RunState(started_at=datetime.now(timezone.utc))
            return self._fallback_run
        state = self._runs.get(session_id)
        if state is None:
            state = _RunState(started_at=datetime.now(timezone.utc))
            self._runs[session_id] = state
        return state

    def _run_state_optional(self, session_id: str | None) -> _RunState | None:
        if not session_id:
            return self._fallback_run
        return self._runs.get(session_id)

    def _take_run(self, session_id: str | None) -> _RunState | None:
        if not session_id:
            run = self._fallback_run
            self._fallback_run = None
            return run
        return self._runs.pop(session_id, None)

    def _close_tool_call(
        self,
        state: _RunState,
        input_data: PreToolUseHookInput | PostToolUseHookInput | PostToolUseFailureHookInput,
        tool_use_id: str | None,
        *,
        status: str,
        target_mrn: str | None,
    ) -> None:
        opened: _ToolOpen | None = None
        if isinstance(tool_use_id, str) and tool_use_id:
            opened = state.tool_open.pop(tool_use_id, None)
        ended = datetime.now(timezone.utc)
        if opened is None:
            name = input_data.get("tool_name")
            tool_name = name if isinstance(name, str) else "tool"
            started = ended
            duration_ms: int | None = None
        else:
            tool_name = opened.tool_name
            started = opened.started_at
            duration_ms = max(0, int((ended - started).total_seconds() * 1000))
        state.tool_calls.append(
            ToolCall(
                tool_name=tool_name,
                target_mrn=target_mrn,
                started_at=started,
                duration_ms=duration_ms,
                status=status,
            )
        )

    # ------------------------------------------------------------------
    # Run record submission

    def _post_run(
        self,
        session_id: str | None,
        run: _RunState,
        summary: TranscriptSummary | None,
        fallback_ended_at: datetime,
    ) -> None:
        started_at = summary.started_at if summary and summary.started_at else run.started_at
        ended_at = summary.ended_at if summary and summary.ended_at else fallback_ended_at
        tokens_in = summary.tokens_in if summary else 0
        tokens_out = summary.tokens_out if summary else 0
        status = "error" if run.error else "success"
        run_id = session_id or _synthetic_run_id(started_at)

        observed_extras: list[str] = []
        if run.upstreams:
            explicit = {call.target_mrn for call in run.tool_calls if call.target_mrn}
            observed_extras = sorted((run.upstreams - explicit) - {self._agent_mrn or ""})

        try:
            self._catalog.record_run(
                AgentRunRecord(
                    agent_mrn=self._agent_mrn or "",
                    run_id=run_id,
                    started_at=started_at,
                    ended_at=ended_at,
                    status=status,
                    model=self._spec.model,
                    tokens_in=tokens_in,
                    tokens_out=tokens_out,
                    error=run.error,
                    tool_calls=run.tool_calls,
                    observed_assets=observed_extras,
                )
            )
        except Exception as e:
            _LOG.warning("failed to record Claude Agent run: %s", e)

    # ------------------------------------------------------------------
    # Agent registration

    async def _ensure_registered(self) -> None:
        if self._agent_mrn is not None:
            return
        async with self._register_lock:
            if self._agent_mrn is not None:
                return
            await asyncio.to_thread(self._register)

    def _register(self) -> None:
        try:
            asset = self._catalog.register_agent(self._spec)
        except Exception as e:
            _LOG.warning("failed to register agent asset: %s", e)
            return
        self._agent_id = _str_or_none(asset.id)
        self._agent_mrn = _str_or_none(asset.mrn)


def _capture_transcript_path(state: _RunState, input_data: BaseHookInput) -> None:
    if state.transcript_path:
        return
    if path := input_data.get("transcript_path"):
        state.transcript_path = path


def _session_id_of(input_data: BaseHookInput) -> str | None:
    return input_data.get("session_id") or None


def _synthetic_run_id(started_at: datetime) -> str:
    return f"run-{int(started_at.timestamp() * 1000)}"


def _str_or_none(value: Any) -> str | None:
    return value if isinstance(value, str) and value else None
