"""Runnable example: authenticating a workload with no API key to hand out.

In a pod, a GitHub Actions job or on a GCP instance, the platform already
attests the workload's identity. `OIDCCredential` presents that attestation as
an RFC 8693 subject token, exchanges it at `/oauth/token` for a Marmot JWT, and
exchanges again when the client sees a 401 — so a short-lived token needs no
handling by the caller.

Nothing here reads `$MARMOT_API_KEY`: it shows the last link of the credential
chain on its own.

Usage:
    pip install marmot-sdk
    export MARMOT_HOST=http://localhost:8080
    python examples/workload_identity.py                       # run inside a pod / CI job / GCP VM
"""

from __future__ import annotations

from marmot import ApiClient, AuthenticatedApiClient, Configuration, SearchApi, UsersApi
from marmot.auth import OIDCCredential, resolve_host
from marmot.auth.workload.github import GitHubActionsSource
from marmot.auth.workload.kubernetes import KubernetesServiceAccountSource
from marmot.errors import AuthError


def exchange_explicitly() -> None:
    """Name the sources to try, in order, and exchange up front.

    `resolve_credential` already does this as the chain's last step; doing it by
    hand is clearer when a workload has no other credential and a failure should
    be reported as such.
    """
    host = resolve_host()

    credential = OIDCCredential(
        api_client=ApiClient(Configuration(host=host)),
        audience=host,
        sources=[KubernetesServiceAccountSource(), GitHubActionsSource()],
    )

    try:
        token = credential.get_token()
    except AuthError as error:
        print(f"no workload identity available here: {error}")
        return

    print(f"exchanged a subject token for a Marmot JWT ({len(token)} chars)")

    client = AuthenticatedApiClient(host, credential)
    me = UsersApi(client).get_users_me_sync()
    print(f"acting as: {me.name}")

    # A 401 mid-session triggers a fresh exchange and the request is retried
    # once, so a token expiring between calls is invisible here.
    hits = SearchApi(client).get_search_sync(q="orders", limit=1)
    print(f"search still works after any refresh: {hits.total} hits")


def let_the_chain_decide() -> None:
    """The same thing via `connect`, restricted to workload identity.

    Passing `sources` narrows which attestations are attempted; the earlier
    links of the chain (explicit arguments, environment, cached login) still win
    if they resolve.
    """
    client = AuthenticatedApiClient.connect(sources=[KubernetesServiceAccountSource()])
    print(f"credential in use: {client.credential.source}")


def main() -> None:
    exchange_explicitly()
    let_the_chain_decide()


if __name__ == "__main__":
    main()
