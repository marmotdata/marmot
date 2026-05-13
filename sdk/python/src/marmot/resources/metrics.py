"""Catalog usage and asset breakdown metrics."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.metrics import (
    get_metrics_assets_by_provider,
    get_metrics_assets_by_type,
    get_metrics_assets_total,
    get_metrics_top_assets,
    get_metrics_top_queries,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.asset_count import AssetCount
from marmot._gen.models.assets_by_provider_response import AssetsByProviderResponse
from marmot._gen.models.assets_by_type_response import AssetsByTypeResponse
from marmot._gen.models.query_count import QueryCount
from marmot._gen.models.total_assets_response import TotalAssetsResponse
from marmot._gen.types import UNSET, Unset


class MetricsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def total_assets(self) -> TotalAssetsResponse:
        """Return the total number of assets in the catalog."""
        return cast(
            TotalAssetsResponse,
            unwrap(get_metrics_assets_total.sync_detailed(client=self._c)),
        )

    def assets_by_type(self) -> AssetsByTypeResponse:
        """Return asset counts grouped by type."""
        return cast(
            AssetsByTypeResponse,
            unwrap(get_metrics_assets_by_type.sync_detailed(client=self._c)),
        )

    def assets_by_provider(self) -> AssetsByProviderResponse:
        """Return asset counts grouped by provider."""
        return cast(
            AssetsByProviderResponse,
            unwrap(get_metrics_assets_by_provider.sync_detailed(client=self._c)),
        )

    def top_assets(
        self,
        *,
        start: str,
        end: str,
        limit: int | None = None,
    ) -> list[AssetCount]:
        """Return the most-viewed assets in [start, end] (RFC3339 timestamps)."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        return cast(
            list[AssetCount],
            unwrap(
                get_metrics_top_assets.sync_detailed(
                    client=self._c, start=start, end=end, limit=limit_arg
                )
            ),
        )

    def top_queries(
        self,
        *,
        start: str,
        end: str,
        limit: int | None = None,
    ) -> list[QueryCount]:
        """Return the most-run queries in [start, end] (RFC3339 timestamps)."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        return cast(
            list[QueryCount],
            unwrap(
                get_metrics_top_queries.sync_detailed(
                    client=self._c, start=start, end=end, limit=limit_arg
                )
            ),
        )
