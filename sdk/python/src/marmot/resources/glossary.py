"""Glossary term CRUD and search."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.glossary import (
    delete_glossary_id,
    get_glossary_id,
    get_glossary_list,
    get_glossary_search,
    post_glossary,
    put_glossary_id,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.create_term_request import CreateTermRequest
from marmot._gen.models.glossary_list_result import GlossaryListResult
from marmot._gen.models.glossary_term import GlossaryTerm
from marmot._gen.models.update_term_request import UpdateTermRequest
from marmot._gen.types import UNSET, Unset


class GlossaryResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self, *, limit: int | None = None, offset: int | None = None) -> GlossaryListResult:
        """Return paginated glossary terms."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            GlossaryListResult,
            unwrap(
                get_glossary_list.sync_detailed(client=self._c, limit=limit_arg, offset=offset_arg)
            ),
        )

    def search(
        self,
        *,
        query: str | None = None,
        parent_term_id: str | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> GlossaryListResult:
        """Search glossary terms."""
        q_arg: str | Unset = query if query is not None else UNSET
        parent_arg: str | Unset = parent_term_id if parent_term_id is not None else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            GlossaryListResult,
            unwrap(
                get_glossary_search.sync_detailed(
                    client=self._c,
                    q=q_arg,
                    parent_term_id=parent_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )

    def get(self, term_id: str) -> GlossaryTerm:
        """Fetch a glossary term by ID."""
        return cast(
            GlossaryTerm,
            unwrap(get_glossary_id.sync_detailed(id=term_id, client=self._c)),
        )

    def create(
        self,
        *,
        name: str,
        definition: str,
        description: str = "",
        parent_term_id: str = "",
    ) -> GlossaryTerm:
        """Create a new glossary term."""
        body = CreateTermRequest(
            name=name,
            definition=definition,
            description=description if description else UNSET,
            parent_term_id=parent_term_id if parent_term_id else UNSET,
        )
        return cast(
            GlossaryTerm,
            unwrap(post_glossary.sync_detailed(client=self._c, body=body)),
        )

    def update(
        self,
        term_id: str,
        *,
        name: str = "",
        definition: str = "",
        description: str = "",
        parent_term_id: str = "",
    ) -> GlossaryTerm:
        """Update an existing glossary term."""
        body = UpdateTermRequest(
            name=name if name else UNSET,
            definition=definition if definition else UNSET,
            description=description if description else UNSET,
            parent_term_id=parent_term_id if parent_term_id else UNSET,
        )
        return cast(
            GlossaryTerm,
            unwrap(put_glossary_id.sync_detailed(id=term_id, client=self._c, body=body)),
        )

    def delete(self, term_id: str) -> None:
        """Delete a glossary term."""
        unwrap(delete_glossary_id.sync_detailed(id=term_id, client=self._c))
