"""Unified search across assets, glossary terms, teams, and users."""

from __future__ import annotations

from typing import Any, cast

from marmot._adapter import unwrap
from marmot._gen.api.search import get_search
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.search_response import SearchResponse
from marmot._gen.types import UNSET, Unset


class SearchResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def query(
        self,
        query: str,
        *,
        types: list[str] | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> SearchResponse:
        """Run a unified search."""
        types_arg: list[str] | Unset = types if types else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            SearchResponse,
            unwrap(
                get_search.sync_detailed(
                    client=self._c,
                    q=query,
                    types=types_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )

    def __call__(self, query: str, **kwargs: Any) -> SearchResponse:
        """Shortcut for :meth:`query` so ``client.search(...)`` works directly."""
        return self.query(query, **kwargs)
