"""Record and read agent invocation telemetry."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from marmot._http import Transport
from marmot.resources import API_PREFIX


class AgentRunsResource:
    """SDK surface for the ``/agents`` endpoints.

    ``create`` posts a completed run; the server persists ``agent_runs`` and
    ``agent_tool_calls`` rows and emits one ``AGENT_LOOKUP`` observed lineage
    edge per tool call that resolved to a catalogued asset.
    """

    def __init__(self, transport: Transport) -> None:
        self._t = transport

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
    ) -> dict[str, Any]:
        """Record a completed agent run.

        ``tool_calls`` is a list of dicts with the keys ``tool_name``,
        ``started_at`` (datetime), ``status`` (``success`` / ``error``), and
        optionally ``target_mrn`` and ``duration_ms``.
        """
        body: dict[str, Any] = {
            "agent_mrn": agent_mrn,
            "run_id": run_id,
            "started_at": _iso(started_at),
            "status": status,
            "tokens_in": tokens_in,
            "tokens_out": tokens_out,
        }
        if ended_at is not None:
            body["ended_at"] = _iso(ended_at)
        if model:
            body["model"] = model
        if error:
            body["error"] = error
        if tool_calls:
            body["tool_calls"] = [_normalize_tool_call(tc) for tc in tool_calls]
        if observed_assets:
            body["observed_assets"] = list(observed_assets)
        return self._t.post(f"{API_PREFIX}/agents/runs", json=body)

    def list(
        self,
        asset_id: str,
        *,
        period: str = "24h",
        limit: int = 25,
    ) -> dict[str, Any]:
        """List recent runs for an agent asset."""
        return self._t.get(
            f"{API_PREFIX}/agents/{asset_id}/runs",
            params={"period": period, "limit": limit},
        )

    def stats(self, asset_id: str, *, period: str = "24h") -> dict[str, Any]:
        return self._t.get(
            f"{API_PREFIX}/agents/{asset_id}/stats", params={"period": period}
        )

    def activity(self, asset_id: str, *, period: str = "24h") -> dict[str, Any]:
        return self._t.get(
            f"{API_PREFIX}/agents/{asset_id}/activity", params={"period": period}
        )


def _iso(dt: datetime) -> str:
    # The Go server expects RFC3339; isoformat() with a tz is acceptable.
    if dt.tzinfo is None:
        return dt.isoformat() + "Z"
    return dt.isoformat()


def _normalize_tool_call(tc: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {
        "tool_name": tc["tool_name"],
        "started_at": _iso(tc["started_at"]) if isinstance(tc["started_at"], datetime) else tc["started_at"],
        "status": tc.get("status", "success"),
    }
    if "target_mrn" in tc and tc["target_mrn"]:
        out["target_mrn"] = tc["target_mrn"]
    if "duration_ms" in tc and tc["duration_ms"] is not None:
        out["duration_ms"] = int(tc["duration_ms"])
    return out
