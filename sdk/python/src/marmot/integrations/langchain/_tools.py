"""LangChain tools backed directly by the Marmot SDK.

Each tool is a thin wrapper over a :class:`marmot.Client` method, exposed as a
:class:`langchain_core.tools.StructuredTool` so an LLM agent can call it.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from marmot.errors import NotFoundError

if TYPE_CHECKING:
    from langchain_core.tools import BaseTool

    from marmot.client import Client


def catalog_tools(client: Client) -> list[BaseTool]:
    """Return a list of LangChain tools that read from the given Marmot client.

    The tools are bound to ``client``; they share its auth and HTTP session.
    Hand the list straight to an agent factory:

        from marmot import connect
        from marmot.integrations.langchain import catalog_tools

        with connect() as client:
            tools = catalog_tools(client)
            agent = create_react_agent(llm, tools)
    """
    try:
        from langchain_core.tools import StructuredTool
    except ImportError as e:
        raise ImportError(
            "langchain-core is required for marmot.integrations.langchain. "
            "Install via `pip install marmot-sdk[langchain]`."
        ) from e

    def search_catalog(query: str, limit: int = 20) -> dict[str, Any]:
        """Search the Marmot data catalog. Returns up to ``limit`` matches (max 100).

        ``query`` accepts plain free text OR Marmot's structured query language.
        Catalogs can hold millions of assets — prefer structured queries over
        broad free-text when you know any of: name, type, provider, or metadata.

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

          Looking for an asset by name:
            @name: "metrics-current"

          Looking for a name on a specific platform:
            @name: "metrics-current" AND @provider: "OpenSearch"

          All Kafka topics:
            @type: "Topic" AND @provider: "kafka"

          Customer-related Postgres tables only:
            (@type: "Table" OR @type: "View") AND @provider: "postgres" AND @name contains "customer"

          Free text fallback when you don't know fields:
            user orders

        After this returns, use ``lookup_asset`` (when you know type+provider+name)
        or ``get_asset`` (when you have an id from these results) for full details.
        """
        raw = client.search(query, limit=limit)
        hits = []
        for r in raw.get("results") or []:
            md = r.get("metadata") or {}
            hits.append(
                {
                    "id": r.get("id"),
                    "name": r.get("name"),
                    "type": md.get("type"),
                    "provider": md.get("primary_provider"),
                    "mrn": md.get("mrn"),
                    "description": r.get("description"),
                }
            )
        return {"results": hits, "total": raw.get("total", len(hits))}

    def get_asset(asset_id: str) -> dict[str, Any]:
        """Fetch the full details of a single asset by its Marmot ID.

        Returns the asset's name, MRN, type, provider, description, owner,
        schema, and any provider-specific metadata. Use this after
        ``search_catalog`` finds a candidate, when you need column/schema
        details to write a query or understand structure.
        """
        return client.assets.get(asset_id)

    def lookup_asset(
        asset_type: str, service: str, name: str
    ) -> dict[str, Any] | None:
        """Look up a single asset by its (type, service, name) triple.

        Use this when you already know the natural identifiers — for example
        ``asset_type="table"``, ``service="postgres"``, ``name="prod.orders"``.
        Returns ``None`` if no asset matches.
        """
        try:
            return client.assets.lookup(type=asset_type, service=service, name=name)
        except NotFoundError:
            return None

    def get_upstream_lineage(asset_id: str, depth: int = 2) -> dict[str, Any]:
        """Trace the upstream lineage of an asset — what feeds into it.

        Returns the graph of ancestors up to ``depth`` hops. Use this to
        understand where data comes from, who/what writes to a table, or to
        find a root source you can query directly.
        """
        return client.lineage.upstream(asset_id, depth=depth)

    # Tools whose return value uniquely identifies the asset the agent fetched
    # opt in to lineage emission. search_catalog deliberately does NOT — its
    # output is a list of *candidates*, not a chosen lookup.
    lookup_metadata = {"marmot_record_lookups": True}

    return [
        StructuredTool.from_function(search_catalog),
        StructuredTool.from_function(get_asset, metadata=lookup_metadata),
        StructuredTool.from_function(lookup_asset, metadata=lookup_metadata),
        StructuredTool.from_function(get_upstream_lineage, metadata=lookup_metadata),
    ]
