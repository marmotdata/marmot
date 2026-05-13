"""Record and read agent invocation telemetry."""

from __future__ import annotations

from datetime import datetime
from typing import Any, cast

from marmot._adapter import unwrap
from marmot._gen.api.agents import (
    get_agents_asset_id_activity,
    get_agents_asset_id_runs,
    get_agents_asset_id_stats,
    post_agents_runs,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.activity_response import ActivityResponse
from marmot._gen.models.agent_run import AgentRun
from marmot._gen.models.record_run_request import RecordRunRequest
from marmot._gen.models.runs_response import RunsResponse
from marmot._gen.models.stats import Stats
from marmot._gen.models.tool_call_payload import ToolCallPayload
from marmot._gen.types import UNSET, Unset


class AgentRunsResource:
    """SDK surface for the ``/agents`` endpoints.

    ``create`` posts a completed run; the server persists ``agent_runs`` and
    ``agent_tool_calls`` rows and emits one ``AGENT_LOOKUP`` observed lineage
    edge per tool call that resolved to a catalogued asset.
    """

    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def create(
        self,
        *,
        agent_mrn: str,
        run_id: str,
        started_at: datetime,
        status: str,
        ended_at: datetime | None = None,
        model: str | None = None,
        tokens_in: int = 0,
        tokens_out: int = 0,
        error: str | None = None,
        tool_calls: list[dict[str, Any]] | None = None,
        observed_assets: list[str] | None = None,
    ) -> AgentRun:
        """Record a completed agent run."""
        body = RecordRunRequest(
            agent_mrn=agent_mrn,
            run_id=run_id,
            started_at=_iso(started_at),
            status=status,
            tokens_in=tokens_in,
            tokens_out=tokens_out,
            ended_at=_iso(ended_at) if ended_at else UNSET,
            model=model or UNSET,
            error=error or UNSET,
            tool_calls=([_normalize_tool_call(tc) for tc in tool_calls] if tool_calls else UNSET),
            observed_assets=list(observed_assets) if observed_assets else UNSET,
        )
        return cast(
            AgentRun,
            unwrap(post_agents_runs.sync_detailed(client=self._c, body=body)),
        )

    def list(
        self,
        asset_id: str,
        *,
        period: str = "24h",
        limit: int = 25,
    ) -> RunsResponse:
        """List recent runs for an agent asset."""
        return cast(
            RunsResponse,
            unwrap(
                get_agents_asset_id_runs.sync_detailed(
                    asset_id=asset_id, client=self._c, period=period, limit=limit
                )
            ),
        )

    def stats(self, asset_id: str, *, period: str = "24h") -> Stats:
        return cast(
            Stats,
            unwrap(
                get_agents_asset_id_stats.sync_detailed(
                    asset_id=asset_id, client=self._c, period=period
                )
            ),
        )

    def activity(self, asset_id: str, *, period: str = "24h") -> ActivityResponse:
        return cast(
            ActivityResponse,
            unwrap(
                get_agents_asset_id_activity.sync_detailed(
                    asset_id=asset_id, client=self._c, period=period
                )
            ),
        )


def _iso(dt: datetime) -> str:
    if dt.tzinfo is None:
        return dt.isoformat() + "Z"
    return dt.isoformat()


def _normalize_tool_call(tc: dict[str, Any]) -> ToolCallPayload:
    started: Any = tc["started_at"]
    started_iso = _iso(started) if isinstance(started, datetime) else started
    target: str | Unset = tc["target_mrn"] if tc.get("target_mrn") else UNSET
    duration: int | Unset = (
        int(tc["duration_ms"]) if "duration_ms" in tc and tc["duration_ms"] is not None else UNSET
    )
    return ToolCallPayload(
        tool_name=tc["tool_name"],
        started_at=started_iso,
        status=tc.get("status", "success"),
        target_mrn=target,
        duration_ms=duration,
    )
