"""Read and write lineage edges."""

from __future__ import annotations

from collections.abc import Iterable
from typing import Any

from marmot._http import Transport
from marmot.resources import API_PREFIX

DEFAULT_EDGE_TYPE = "DIRECT"


class LineageResource:
    def __init__(self, transport: Transport) -> None:
        self._t = transport

    def write(
        self,
        *,
        source: str,
        target: str,
        type: str = DEFAULT_EDGE_TYPE,
        job_mrn: str | None = None,
    ) -> dict[str, Any]:
        """Create a single lineage edge from ``source`` MRN to ``target`` MRN."""
        body: dict[str, Any] = {"source": source, "target": target, "type": type}
        if job_mrn:
            body["job_mrn"] = job_mrn
        return self._t.post(f"{API_PREFIX}/lineage/direct", json=body)

    def batch(
        self,
        edges: Iterable[dict[str, Any] | tuple[str, str] | tuple[str, str, str]],
    ) -> list[dict[str, Any]]:
        """Create many lineage edges in one call.

        Each edge can be a dict ``{"source": ..., "target": ..., "type": ...}``
        or a tuple ``(source, target)`` / ``(source, target, type)``.
        """
        body = [self._normalize_edge(e) for e in edges]
        return self._t.post(f"{API_PREFIX}/lineage/batch", json=body)

    def upstream(self, asset_id: str, *, depth: int | None = None) -> dict[str, Any]:
        """Fetch upstream lineage for an asset."""
        params: dict[str, Any] = {}
        if depth is not None:
            params["depth"] = depth
        return self._t.get(f"{API_PREFIX}/lineage/assets/{asset_id}", params=params)

    @staticmethod
    def _normalize_edge(e: Any) -> dict[str, Any]:
        if isinstance(e, dict):
            edge = dict(e)
            edge.setdefault("type", DEFAULT_EDGE_TYPE)
            return edge
        if isinstance(e, tuple):
            if len(e) == 2:
                return {"source": e[0], "target": e[1], "type": DEFAULT_EDGE_TYPE}
            if len(e) == 3:
                return {"source": e[0], "target": e[1], "type": e[2]}
        raise TypeError(f"unsupported edge representation: {e!r}")
