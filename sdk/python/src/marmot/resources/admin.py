"""Administrative operations: search reindexing."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.admin import get_admin_search_reindex, post_admin_search_reindex
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.reindex_accepted_response import ReindexAcceptedResponse
from marmot._gen.models.reindex_status_response import ReindexStatusResponse


class AdminResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def reindex(self) -> ReindexAcceptedResponse:
        """Trigger a full search reindex."""
        return cast(
            ReindexAcceptedResponse,
            unwrap(post_admin_search_reindex.sync_detailed(client=self._c)),
        )

    def reindex_status(self) -> ReindexStatusResponse:
        """Return current reindex progress."""
        return cast(
            ReindexStatusResponse,
            unwrap(get_admin_search_reindex.sync_detailed(client=self._c)),
        )
