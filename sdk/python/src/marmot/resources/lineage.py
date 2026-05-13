"""Read and write lineage edges."""

from __future__ import annotations

from collections.abc import Iterable
from typing import Any, cast
from uuid import UUID

from marmot._adapter import unwrap
from marmot._gen.api.lineage import (
    get_lineage_assets_id,
    post_lineage_batch,
    post_lineage_direct,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.batch_lineage_result import BatchLineageResult
from marmot._gen.models.get_lineage_assets_id_direction import GetLineageAssetsIdDirection
from marmot._gen.models.lineage_edge import LineageEdge
from marmot._gen.models.lineage_response import LineageResponse
from marmot._gen.types import UNSET, Unset

DEFAULT_EDGE_TYPE = "DIRECT"


class LineageResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def get(
        self,
        asset_id: str,
        *,
        direction: str | None = None,
        limit: int | None = None,
    ) -> LineageResponse:
        """Fetch the lineage graph for an asset.

        ``direction`` is "upstream", "downstream", or "both" (the API default).
        ``limit`` caps the traversal depth.
        """
        dir_arg: GetLineageAssetsIdDirection | Unset = (
            GetLineageAssetsIdDirection(direction) if direction is not None else UNSET
        )
        limit_arg: int | Unset = limit if limit is not None else UNSET
        return cast(
            LineageResponse,
            unwrap(
                get_lineage_assets_id.sync_detailed(
                    id=UUID(asset_id), client=self._c, limit=limit_arg, direction=dir_arg
                )
            ),
        )

    def upstream(self, asset_id: str, *, limit: int | None = None) -> LineageResponse:
        """Fetch upstream lineage for an asset (convenience wrapper around :meth:`get`)."""
        return self.get(asset_id, direction="upstream", limit=limit)

    def write(
        self,
        *,
        source: str,
        target: str,
        type: str = DEFAULT_EDGE_TYPE,
        job_mrn: str | None = None,
    ) -> LineageEdge:
        """Create a single lineage edge from ``source`` MRN to ``target`` MRN."""
        edge = LineageEdge(
            source=source,
            target=target,
            type_=type,
            job_mrn=job_mrn if job_mrn else UNSET,
        )
        return cast(
            LineageEdge,
            unwrap(post_lineage_direct.sync_detailed(client=self._c, body=edge)),
        )

    def batch(
        self,
        edges: Iterable[LineageEdge | dict[str, Any] | tuple[str, str] | tuple[str, str, str]],
    ) -> list[BatchLineageResult]:
        """Create many lineage edges in one call.

        Each edge can be a :class:`LineageEdge`, a dict
        ``{"source": ..., "target": ..., "type": ...}``, or a tuple
        ``(source, target)`` / ``(source, target, type)``.
        """
        body = [self._normalize_edge(e) for e in edges]
        return cast(
            list[BatchLineageResult],
            unwrap(post_lineage_batch.sync_detailed(client=self._c, body=body)),
        )

    @staticmethod
    def _normalize_edge(e: Any) -> LineageEdge:
        if isinstance(e, LineageEdge):
            return e
        if isinstance(e, dict):
            d = dict(e)
            d.setdefault("type", DEFAULT_EDGE_TYPE)
            return LineageEdge.from_dict(d)
        if isinstance(e, tuple):
            if len(e) == 2:
                return LineageEdge(source=e[0], target=e[1], type_=DEFAULT_EDGE_TYPE)
            if len(e) == 3:
                return LineageEdge(source=e[0], target=e[1], type_=e[2])
        raise TypeError(f"unsupported edge representation: {e!r}")
