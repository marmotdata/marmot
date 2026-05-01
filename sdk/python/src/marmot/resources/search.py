"""Search the catalog."""

from __future__ import annotations

from typing import Any

from marmot._http import Transport
from marmot.resources import API_PREFIX


class SearchResource:
    def __init__(self, transport: Transport) -> None:
        self._t = transport

    def __call__(
        self,
        query: str,
        *,
        types: list[str] | None = None,
        providers: list[str] | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> dict[str, Any]:
        """Run a catalog search.

        Returns the raw response dict from ``GET /api/v1/search``. Schema is
        documented in the OpenAPI spec.
        """
        params: dict[str, Any] = {"query": query}
        if types:
            params["asset_types"] = types
        if providers:
            params["providers"] = providers
        if limit is not None:
            params["limit"] = limit
        if offset is not None:
            params["offset"] = offset
        return self._t.get(f"{API_PREFIX}/search", params=params)
