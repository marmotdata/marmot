"""Asset CRUD, search, summary, and tag management."""

from __future__ import annotations

from typing import Any, cast

from marmot._adapter import unwrap
from marmot._gen.api.assets import (
    delete_assets_column_tags_id,
    delete_assets_id,
    delete_assets_tags_id,
    get_assets_id,
    get_assets_lookup_type_service_name,
    get_assets_search,
    get_assets_summary,
    get_assets_tags_id,
    post_assets,
    post_assets_tags_id,
    put_assets_column_tags_id,
    put_assets_id,
    put_assets_tags_id,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.add_asset_tag_request import AddAssetTagRequest
from marmot._gen.models.asset import Asset
from marmot._gen.models.asset_search_response import AssetSearchResponse
from marmot._gen.models.asset_summary_response import AssetSummaryResponse
from marmot._gen.models.create_asset_request import CreateAssetRequest
from marmot._gen.models.remove_asset_column_tag_request import RemoveAssetColumnTagRequest
from marmot._gen.models.remove_asset_tag_request import RemoveAssetTagRequest
from marmot._gen.models.replace_asset_column_tags_request import ReplaceAssetColumnTagsRequest
from marmot._gen.models.replace_asset_tags_request import ReplaceAssetTagsRequest
from marmot._gen.models.tag import Tag
from marmot._gen.models.update_asset_request import UpdateAssetRequest
from marmot._gen.types import UNSET, Unset
from marmot.errors import NotFoundError


class AssetsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def get(self, asset_id: str) -> Asset:
        """Fetch an asset by ID."""
        return cast(Asset, unwrap(get_assets_id.sync_detailed(id=asset_id, client=self._c)))

    def lookup(self, *, type: str, service: str, name: str) -> Asset:
        """Fetch an asset by its (type, service, name) triple."""
        return cast(
            Asset,
            unwrap(
                get_assets_lookup_type_service_name.sync_detailed(
                    type_=type, service=service, name=name, client=self._c
                )
            ),
        )

    def find(self, *, type: str, service: str, name: str) -> Asset | None:
        """Like :meth:`lookup` but returns ``None`` instead of raising on 404."""
        try:
            return self.lookup(type=type, service=service, name=name)
        except NotFoundError:
            return None

    def search(
        self,
        *,
        query: str | None = None,
        types: list[str] | None = None,
        providers: list[str] | None = None,
        tags: list[str] | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> AssetSearchResponse:
        """Search assets with optional filters."""
        q_arg: str | Unset = query if query is not None else UNSET
        types_arg: list[str] | Unset = types if types else UNSET
        services_arg: list[str] | Unset = providers if providers else UNSET
        tags_arg: list[str] | Unset = tags if tags else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            AssetSearchResponse,
            unwrap(
                get_assets_search.sync_detailed(
                    client=self._c,
                    q=q_arg,
                    types=types_arg,
                    services=services_arg,
                    tags=tags_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )

    def summary(self) -> AssetSummaryResponse:
        """Return aggregate counts for the catalog (totals, by-type, etc.)."""
        return cast(
            AssetSummaryResponse,
            unwrap(get_assets_summary.sync_detailed(client=self._c)),
        )

    def create(self, asset: CreateAssetRequest | dict[str, Any]) -> Asset:
        """Create a new asset. Must include name, type, providers.

        Accepts a :class:`CreateAssetRequest` for type-safety, or a plain dict
        for ergonomic ad-hoc use.
        """
        body = (
            asset if isinstance(asset, CreateAssetRequest) else CreateAssetRequest.from_dict(asset)
        )
        return cast(
            Asset,
            unwrap(post_assets.sync_detailed(client=self._c, body=body)),
        )

    def update(self, asset_id: str, asset: UpdateAssetRequest | dict[str, Any]) -> Asset:
        """Update an existing asset by ID."""
        body = (
            asset if isinstance(asset, UpdateAssetRequest) else UpdateAssetRequest.from_dict(asset)
        )
        return cast(
            Asset,
            unwrap(put_assets_id.sync_detailed(id=asset_id, client=self._c, body=body)),
        )

    def delete(self, asset_id: str) -> None:
        """Delete an asset by ID."""
        unwrap(delete_assets_id.sync_detailed(id=asset_id, client=self._c))

    def add_tag(self, asset_id: str, tag_id: str) -> Tag:
        """Add a tag to an asset by tag ID."""
        body = AddAssetTagRequest(tag_id=tag_id)
        return cast(
            Tag,
            unwrap(post_assets_tags_id.sync_detailed(id=asset_id, client=self._c, body=body)),
        )

    def remove_tag(self, asset_id: str, tag_id: str) -> Tag:
        """Remove a tag from an asset by tag ID."""
        body = RemoveAssetTagRequest(tag_id=tag_id)
        return cast(
            Tag,
            unwrap(delete_assets_tags_id.sync_detailed(id=asset_id, client=self._c, body=body)),
        )

    def list_tags(self, asset_id: str) -> list[Tag]:
        """List all tags associated with an asset."""
        return cast(
            list[Tag],
            unwrap(get_assets_tags_id.sync_detailed(id=asset_id, client=self._c)),
        )

    def set_tags(self, asset_id: str, tag_ids: list[str]) -> list[Tag]:
        """Atomically replace all tag associations for an asset."""
        body = ReplaceAssetTagsRequest(tag_ids=tag_ids)
        return cast(
            list[Tag],
            unwrap(put_assets_tags_id.sync_detailed(id=asset_id, client=self._c, body=body)),
        )

    def set_column_tags(self, asset_id: str, column_path: str, tag_ids: list[str]) -> None:
        """Atomically replace the tag set assigned to one column."""
        body = ReplaceAssetColumnTagsRequest(column_path=column_path, tag_ids=tag_ids)
        unwrap(put_assets_column_tags_id.sync_detailed(id=asset_id, client=self._c, body=body))

    def remove_column_tag(self, asset_id: str, column_path: str, tag_id: str) -> None:
        """Remove one (column_path, tag_id) assignment for an asset."""
        body = RemoveAssetColumnTagRequest(column_path=column_path, tag_id=tag_id)
        unwrap(delete_assets_column_tags_id.sync_detailed(id=asset_id, client=self._c, body=body))
