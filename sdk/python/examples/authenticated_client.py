"""Runnable example: how the authenticated client resolves and uses credentials.

`AuthenticatedApiClient` owns one credential and one HTTP connection, and every
generated `*Api` class borrows it. Whether a given call sends the credential is
decided by the operation's `security` in the spec, not by the caller.

Usage:
    pip install marmot-sdk
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_API_KEY=...            # or run `marmot login` first
    python examples/authenticated_client.py
"""

from __future__ import annotations

from marmot import AuthenticatedApiClient, MetricsApi, SearchApi, UsersApi
from marmot.auth import SecurityScheme, StaticCredential, resolve_host
from marmot.errors import AuthError, MarmotError, NotFoundError

QUERY = "orders"


def resolved_from_the_environment() -> None:
    """The common case: let the chain pick a credential.

    `connect` tries explicit arguments, then `$MARMOT_API_KEY`, then
    `$MARMOT_TOKEN`, then the token `marmot login` cached, then workload
    identity. `credential.source` says which one answered.
    """
    client = AuthenticatedApiClient.connect()

    print(f"host:       {client.configuration.host}")
    print(f"credential: {client.credential.source}")
    print(f"scheme:     {client.credential.scheme.value}")

    me = UsersApi(client).get_users_me_sync()
    print(f"acting as:  {me.name} (roles: {[role.name for role in me.roles or []]})")


def one_client_many_apis() -> None:
    """One client, many API classes: they share its credential and connection."""
    client = AuthenticatedApiClient.connect()

    hits = SearchApi(client).get_search_sync(q=QUERY, limit=5)
    print(f"\n{hits.total} assets match {QUERY!r}; first {len(hits.results or [])}:")
    for result in hits.results or []:
        # `type` is an enum whose str() is the member name, so show its value
        kind = result.type.value if result.type else "?"
        print(f"  {kind:12} {result.name}")

    total = MetricsApi(client).get_metrics_assets_total_sync()
    print(f"assets in the catalog: {total.count}")


def explicit_credential() -> None:
    """Skip the chain when the caller already holds a credential.

    Useful in a service that receives the end user's token per request: build a
    client for that token rather than reading the environment.
    """
    host = resolve_host()
    credential = StaticCredential("not-a-real-key", SecurityScheme.apikey, source="example")
    client = AuthenticatedApiClient(host, credential)

    try:
        UsersApi(client).get_users_me_sync()
    except AuthError as error:
        print(f"\nrejected, as expected: {error}")


def errors_are_typed() -> None:
    """HTTP failures arrive as exceptions, not status codes to inspect."""
    client = AuthenticatedApiClient.connect()

    try:
        UsersApi(client).get_users_id_sync(id="00000000-0000-0000-0000-000000000000")
    except NotFoundError as error:
        print(f"missing user: {error}")
    except MarmotError as error:  # AuthError, ValidationError, RateLimitError, ServerError
        print(f"request failed: {type(error).__name__}: {error}")


def main() -> None:
    resolved_from_the_environment()
    one_client_many_apis()
    explicit_credential()
    errors_are_typed()


if __name__ == "__main__":
    main()
