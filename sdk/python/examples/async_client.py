"""Runnable example: the async face of the authenticated client.

Every generated operation exists twice: `get_users_me` is a coroutine and
`get_users_me_sync` runs it on a shared event loop. Both share one credential
and one connection pool, so concurrent calls reuse a single refreshed token
rather than exchanging one each.

Usage:
    pip install marmot-sdk
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_API_KEY=...            # or run `marmot login` first
    python examples/async_client.py
"""

from __future__ import annotations

import asyncio

from marmot import AuthenticatedApiClient, GlossaryApi, MetricsApi, SearchApi, UsersApi
from marmot.errors import MarmotError

QUERIES = ("orders", "customers", "payments")


async def concurrent_searches(client: AuthenticatedApiClient) -> None:
    """Fan out over one client: the credential is applied per request."""
    search = SearchApi(client)

    responses = await asyncio.gather(
        *(search.get_search(q=query, limit=3) for query in QUERIES),
        return_exceptions=True,
    )

    for query, response in zip(QUERIES, responses, strict=True):
        if isinstance(response, MarmotError):
            print(f"{query:10} failed: {type(response).__name__}: {response}")
        elif isinstance(response, BaseException):
            raise response
        else:
            print(f"{query:10} {response.total} hits")


async def gather_across_apis(client: AuthenticatedApiClient) -> None:
    """Different APIs, same client, one round of concurrency."""
    me, total, terms = await asyncio.gather(
        UsersApi(client).get_users_me(),
        MetricsApi(client).get_metrics_assets_total(),
        GlossaryApi(client).get_glossary_list(limit=5),
    )

    print(f"\nacting as: {me.name}")
    print(f"assets:    {total.count}")
    print(f"terms:     {len(terms.terms or [])} of {terms.total}")


async def main() -> None:
    # As a context manager the client releases its connection pool on exit; the
    # sync facade's event loop is shared and outlives any single client.
    async with AuthenticatedApiClient.connect() as client:
        print(f"host: {client.configuration.host} (auth via {client.credential.source})")
        await concurrent_searches(client)
        await gather_across_apis(client)


if __name__ == "__main__":
    asyncio.run(main())
