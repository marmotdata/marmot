"""LangChain tools backed directly by the Marmot SDK.

Each tool is a thin wrapper over a :class:`CatalogReader` call, exposed as a
:class:`langchain_core.tools.StructuredTool` so an LLM agent can call it.
"""

from __future__ import annotations

import asyncio
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from langchain_core.tools import BaseTool

    from marmot.integrations.catalog import CatalogReader


def catalog_tools(catalog: CatalogReader) -> list[BaseTool]:
    """Return a list of LangChain tools that read from the given catalog.

    The tools are bound to ``catalog``; they share its client, auth and HTTP
    session. Hand the list straight to an agent factory:

        from marmot import AuthenticatedApiClient
        from marmot.integrations import MarmotCatalog
        from marmot.integrations.langchain import catalog_tools

        catalog = MarmotCatalog(AuthenticatedApiClient.connect())
        tools = catalog_tools(catalog)
        agent = create_react_agent(llm, tools)
    """
    try:
        from langchain_core.tools import StructuredTool
    except ImportError as e:
        raise ImportError(
            "langchain-core is required for marmot.integrations.langchain. "
            "Install via `pip install marmot-sdk[langchain]`."
        ) from e

    async def search_catalog(query: str, limit: int = 5) -> dict[str, Any]:
        """Find assets by name, type, provider, or metadata. Use ``count_assets`` for totals.

        ``query`` accepts plain free text OR Marmot's structured query language.
        Prefer structured queries when you know any of: name, type, provider, or metadata.

        Field filters (combine with AND / OR / NOT, group with parentheses):

          @type: "Table"             - asset type, e.g. Table, Topic, Bucket, Alias, Agent
          @provider: "postgres"      - source platform, e.g. postgres, kafka, OpenSearch
          @name: "users"             - exact name match
          @name contains "customer"  - substring on name
          @name: "customer*"         - wildcard
          @metadata.team: "platform" - any metadata key (dot notation for nested)
          @metadata.partitions > 10  - numeric comparisons: > < >= <=
          @metadata.size range [100 TO 500]

        Examples — pick the most specific query you can:

          @name: "metrics-current"
          @name: "metrics-current" AND @provider: "OpenSearch"
          @type: "Topic" AND @provider: "kafka"
          (@type: "Table" OR @type: "View") AND @provider: "postgres" AND @name contains "customer"

        Returns matched assets. Once you have a candidate, use ``lookup_asset``
        (type+provider+name) or ``get_asset`` (id) for full details.
        """
        response = await catalog.asearch(query, limit=limit)
        hits = [
            {
                "id": r.id,
                "name": r.name,
                "type": (r.metadata or {}).get("type"),
                "provider": (r.metadata or {}).get("primary_provider"),
                "mrn": (r.metadata or {}).get("mrn"),
                "description": r.description,
            }
            for r in (response.results or [])
        ]
        return {"results": hits}

    async def get_asset(asset_id: str) -> dict[str, Any]:
        """Fetch the full details of a single asset by its Marmot ID.

        Returns the asset's name, MRN, type, provider, description, owner,
        schema, and any provider-specific metadata. Use this after
        ``search_catalog`` finds a candidate, when you need column/schema
        details to write a query or understand structure.
        """
        return (await catalog.aget_asset(asset_id)).to_dict()

    async def lookup_asset(asset_type: str, service: str, name: str) -> dict[str, Any] | None:
        """Look up a single asset by its (type, service, name) triple.

        Use this when you already know the natural identifiers — for example
        ``asset_type="table"``, ``service="postgres"``, ``name="prod.orders"``.
        Returns ``None`` if no asset matches.
        """
        found = await catalog.alookup_asset(asset_type=asset_type, service=service, name=name)
        return found.to_dict() if found else None

    async def get_upstream_lineage(asset_id: str, depth: int = 2) -> dict[str, Any]:
        """Trace the upstream lineage of an asset — what feeds into it.

        Returns the graph of ancestors up to ``depth`` hops. Use this to
        understand where data comes from, who/what writes to a table, or to
        find a root source you can query directly.
        """
        return (await catalog.aget_upstream_lineage(asset_id, depth=depth)).to_dict()

    async def count_assets(query: str = "") -> dict[str, Any]:
        """Return asset count and a per-type breakdown. Use for ANY "how many" question.

        Call once — the breakdown is already included, do not call per type.
        Pass no argument (or empty string) to count all assets.
        Pass a field filter to count a subset — same syntax as ``search_catalog``.

        Returns ``{"count": N, "by_type": {"Table": N, "Topic": N, ...}}``.

        Examples:
          All assets:              count_assets()
          All tables:              count_assets('@type: "Table"')
          Kafka topics only:       count_assets('@type: "Topic" AND @provider: "kafka"')
          Names matching pattern:  count_assets('@name contains "customer"')
        """
        effective = '@name: "*"' if not query.strip() or query.strip() == "*" else query.strip()

        sample = await catalog.asearch(effective, limit=20)
        total = sample.total or 0
        types_seen = sorted(
            {((r.metadata or {}).get("type") or "Unknown") for r in (sample.results or [])}
        )

        if not types_seen:
            return {"count": total}

        per_type = await asyncio.gather(
            *(catalog.asearch(f'@type: "{t}"', limit=1) for t in types_seen)
        )
        by_type = {
            t: (r.total or 0)
            for t, r in zip(types_seen, per_type, strict=True)
            if (r.total or 0) > 0
        }

        return {"count": total, "by_type": by_type}

    # Tools whose return value uniquely identifies the asset the agent fetched
    # opt in to lineage emission. search_catalog deliberately does NOT — its
    # output is a list of *candidates*, not a chosen lookup.
    lookup_metadata = {"marmot_record_lookups": True}

    return [
        StructuredTool.from_function(coroutine=count_assets),
        StructuredTool.from_function(coroutine=search_catalog),
        StructuredTool.from_function(coroutine=get_asset, metadata=lookup_metadata),
        StructuredTool.from_function(coroutine=lookup_asset, metadata=lookup_metadata),
        StructuredTool.from_function(coroutine=get_upstream_lineage, metadata=lookup_metadata),
    ]
