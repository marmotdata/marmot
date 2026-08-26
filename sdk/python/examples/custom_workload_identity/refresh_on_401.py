"""Runnable example: a rejected token is re-exchanged and the call retried.

Marmot's JWT lasts 24 hours, so waiting for a real expiry is impractical. Here
the token the client holds is replaced with a stale one, which makes the server
answer `401` exactly as it would after an expiry. The client then asks its
credential to refresh — a real RFC 8693 exchange — reapplies the header, and
retries once. A caller sees only the successful response.

Only refreshable credentials get that treatment: an API key that stops working
raises rather than looping.

Needs a server that trusts the local issuer; see examples/README.md.

Usage:
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_SUBJECT_TOKEN_FILE=/tmp/marmot-subject.jwt
    python examples/custom_workload_identity/refresh_on_401.py
"""

from __future__ import annotations

import sys

from custom_workload_identity import FileSubjectTokenSource

from marmot import ApiClient, AuthenticatedApiClient, Configuration, SecurityScheme, UsersApi
from marmot.auth import OIDCCredential, StaticCredential, resolve_host
from marmot.errors import AuthError

STALE_TOKEN = "stale.jwt.value"  # noqa: S105 — stands in for an expired token


def _stored_token(client: AuthenticatedApiClient) -> str:
    """The token the client currently sends, as `param_serialize` will read it."""
    return str(client.configuration.api_key[client.credential.scheme])


def refreshable_credential_recovers() -> None:
    host = resolve_host()
    source = FileSubjectTokenSource()
    if source.fetch(host) is None:
        print(f"no subject token at {source.path}; run local_oidc_issuer.py first")
        sys.exit(1)

    credential = OIDCCredential(
        api_client=ApiClient(Configuration(host=host)),
        audience=host,
        sources=[source],
    )
    try:
        client = AuthenticatedApiClient(host, credential)
        first = UsersApi(client).get_users_me_sync()
    except AuthError as error:
        print(f"exchange refused, so there is nothing to refresh: {error}")
        sys.exit(1)

    exchanged = _stored_token(client)
    print(f"first call ok as {first.name}; token {exchanged[:12]}...")

    client.configuration.api_key[client.credential.scheme] = STALE_TOKEN
    print("token replaced with a stale one; the next call must 401 and recover")

    second = UsersApi(client).get_users_me_sync()
    refreshed = _stored_token(client)

    print(f"second call ok as {second.name}; token {refreshed[:12]}...")
    print(f"token was re-exchanged: {refreshed not in (STALE_TOKEN, exchanged)}")


def static_credential_does_not_retry() -> None:
    """An API key cannot be refreshed, so a 401 is reported, not retried."""
    host = resolve_host()
    client = AuthenticatedApiClient(host, StaticCredential("not-a-real-key", SecurityScheme.apikey))

    try:
        UsersApi(client).get_users_me_sync()
    except AuthError as error:
        print(f"\nstatic credential surfaced the 401 instead of looping: {error}")


def main() -> None:
    refreshable_credential_recovers()
    static_credential_does_not_retry()


if __name__ == "__main__":
    main()
