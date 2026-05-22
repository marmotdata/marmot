"""Data product management and tag management."""

from __future__ import annotations

# mypy doesn't allow to use the list type if the class has a method with the same name e.g. list()
from typing import List, cast  # noqa: UP035

from marmot._adapter import unwrap
from marmot._gen.api.products import (
    delete_products_tags_id,
    get_products_id,
    get_products_list,
    get_products_tags_id,
    post_products_tags_id,
    put_products_tags_id,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.add_data_product_tag_request import AddDataProductTagRequest
from marmot._gen.models.data_product import DataProduct
from marmot._gen.models.data_product_list_result import DataProductListResult
from marmot._gen.models.remove_data_product_tag_request import RemoveDataProductTagRequest
from marmot._gen.models.replace_data_product_tags_request import ReplaceDataProductTagsRequest
from marmot._gen.models.tag import Tag
from marmot._gen.types import UNSET, Unset


class ProductsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self, *, limit: int | None = None, offset: int | None = None) -> DataProductListResult:
        """List all data products with pagination."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            DataProductListResult,
            unwrap(
                get_products_list.sync_detailed(client=self._c, limit=limit_arg, offset=offset_arg)
            ),
        )

    def get(self, product_id: str) -> DataProduct:
        """Fetch a data product by ID."""
        return cast(
            DataProduct,
            unwrap(get_products_id.sync_detailed(id=product_id, client=self._c)),
        )

    def list_tags(self, product_id: str) -> List[Tag]:  # noqa: UP006
        """List all tags associated with a data product."""
        return cast(
            List[Tag],  # noqa: UP006
            unwrap(get_products_tags_id.sync_detailed(id=product_id, client=self._c)),
        )

    def add_tag(self, product_id: str, tag_id: str) -> List[Tag]:  # noqa: UP006
        """Add a single tag association to a data product."""
        body = AddDataProductTagRequest(tag_id=tag_id)
        return cast(
            List[Tag],  # noqa: UP006
            unwrap(post_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)),
        )

    def remove_tag(self, product_id: str, tag_id: str) -> dict[str, str]:
        """Remove a single tag association from a data product."""
        body = RemoveDataProductTagRequest(tag_id=tag_id)
        return cast(
            dict[str, str],
            unwrap(delete_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)),
        )

    def set_tags(self, product_id: str, tag_ids: List[str]) -> DataProduct:  # noqa: UP006
        """Atomically replace all tag associations for a data product."""
        body = ReplaceDataProductTagsRequest(tag_ids=tag_ids)
        return cast(
            DataProduct,
            unwrap(put_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)),
        )
