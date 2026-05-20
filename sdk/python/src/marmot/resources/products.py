"""Data product management and tag management."""

from __future__ import annotations

from typing import cast

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
from marmot._gen.models.dataproduct_data_product import DataproductDataProduct
from marmot._gen.models.dataproduct_list_result import DataproductListResult
from marmot._gen.models.github_com_marmotdata_marmot_internal_core_tag_tag import (
    GithubComMarmotdataMarmotInternalCoreTagTag,
)
from marmot._gen.models.v1_dataproducts_add_product_tag_request import (
    V1DataproductsAddProductTagRequest,
)
from marmot._gen.models.v1_dataproducts_remove_product_tag_request import (
    V1DataproductsRemoveProductTagRequest,
)
from marmot._gen.models.v1_dataproducts_replace_product_tags_request import (
    V1DataproductsReplaceProductTagsRequest,
)
from marmot._gen.types import UNSET, Unset

Tag = GithubComMarmotdataMarmotInternalCoreTagTag
DataProduct = DataproductDataProduct


class ProductsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self, *, limit: int | None = None, offset: int | None = None) -> DataproductListResult:
        """List all data products with pagination."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            DataproductListResult,
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

    def list_tags(self, product_id: str) -> list[Tag]:
        """List all tags associated with a data product."""
        return cast(
            list[Tag],
            unwrap(get_products_tags_id.sync_detailed(id=product_id, client=self._c)),
        )

    def add_tag(self, product_id: str, tag_id: str) -> list[Tag]:
        """Add a single tag association to a data product."""
        body = V1DataproductsAddProductTagRequest(tag_id=tag_id)
        return cast(
            list[Tag],
            unwrap(post_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)),
        )

    def remove_tag(self, product_id: str, tag_id: str) -> dict[str, str]:
        """Remove a single tag association from a data product."""
        body = V1DataproductsRemoveProductTagRequest(tag_id=tag_id)
        return cast(
            dict[str, str],
            unwrap(
                delete_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)
            ),
        )

    def set_tags(self, product_id: str, tag_ids: list[str]) -> DataProduct:
        """Atomically replace all tag associations for a data product."""
        body = V1DataproductsReplaceProductTagsRequest(tag_ids=tag_ids)
        return cast(
            DataProduct,
            unwrap(put_products_tags_id.sync_detailed(id=product_id, client=self._c, body=body)),
        )
