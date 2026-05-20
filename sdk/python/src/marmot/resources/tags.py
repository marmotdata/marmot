"""Tag vocabulary management."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.tags import (
    delete_tags_id,
    get_tags,
    get_tags_id,
    post_tags,
    put_tags_id,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.create_tag_request import CreateTagRequest
from marmot._gen.models.tag import Tag
from marmot._gen.types import UNSET, Unset


class TagsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self) -> list[Tag]:
        """List all tags in the catalog."""
        return cast(
            list[Tag],
            unwrap(get_tags.sync_detailed(client=self._c)),
        )

    def get(self, tag_id: str) -> Tag:
        """Fetch a tag by ID."""
        return cast(
            Tag,
            unwrap(get_tags_id.sync_detailed(id=tag_id, client=self._c)),
        )

    def create(self, *, name: str, description: str | None = None) -> Tag:
        """Create a new tag in the catalog."""
        desc_arg: str | Unset = description if description is not None else UNSET
        body = CreateTagRequest(name=name, description=desc_arg)
        return cast(
            Tag,
            unwrap(post_tags.sync_detailed(client=self._c, body=body)),
        )

    def update(
        self, tag_id: str, *, name: str | None = None, description: str | None = None
    ) -> Tag:
        """Update an existing tag."""
        name_arg: str | Unset = name if name is not None else UNSET
        desc_arg: str | Unset = description if description is not None else UNSET
        body = CreateTagRequest(name=name_arg, description=desc_arg)
        return cast(
            Tag,
            unwrap(put_tags_id.sync_detailed(id=tag_id, client=self._c, body=body)),
        )

    def delete(self, tag_id: str) -> None:
        """Delete a tag from the catalog."""
        unwrap(delete_tags_id.sync_detailed(id=tag_id, client=self._c))
