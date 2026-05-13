"""Search the catalog for asset owners (users and teams)."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.owners import get_owners_search
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.search_owners_response import SearchOwnersResponse
from marmot._gen.types import UNSET, Unset


class OwnersResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def search(self, query: str, *, limit: int | None = None) -> SearchOwnersResponse:
        """Return owners matching the query."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        return cast(
            SearchOwnersResponse,
            unwrap(get_owners_search.sync_detailed(client=self._c, q=query, limit=limit_arg)),
        )
