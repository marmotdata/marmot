"""Runnable example: teaching the SDK a workload identity it doesn't know about.

The built-in sources cover Kubernetes, GCP and GitHub Actions. Anywhere else —
Vault, a CI runner with its own OIDC endpoint, a sidecar that drops a token on
disk — you supply a `WorkloadIdentitySource`: an object with a `name` and a
`fetch()` that returns a `SubjectToken` when its environment is present, or
`None` so the next source gets a turn.

`register_source` puts it in the registry that `connect()` consults, so callers
need not pass it explicitly.

This one reads an OIDC ID token from a file, which is also the easiest way to
exercise the exchange on a laptop — see examples/README.md for the local issuer
recipe.

Usage:
    pip install marmot-sdk
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_SUBJECT_TOKEN_FILE=/tmp/marmot-subject.jwt
    python examples/custom_workload_identity/custom_workload_identity.py
"""

from __future__ import annotations

import os
import pathlib

from marmot import ApiClient, AuthenticatedApiClient, Configuration, UsersApi
from marmot.auth import OIDCCredential, resolve_host
from marmot.auth.workload import (
    TOKEN_TYPE_ID_TOKEN,
    SubjectToken,
    default_sources,
    register_source,
)
from marmot.errors import AuthError

ENVIRONMENT_TOKEN_FILE = "MARMOT_SUBJECT_TOKEN_FILE"  # noqa: S105 — a variable name
DEFAULT_TOKEN_FILE = "/tmp/marmot-subject.jwt"  # noqa: S105, S108 — a path, dev-only


@register_source
class FileSubjectTokenSource:
    """An OIDC ID token dropped on disk by whatever attests this workload.

    `name` is how the source is reported in `credential.source` and keyed in the
    registry, so keep it stable. Registered sources are constructed with no
    arguments, hence the environment variable rather than a constructor
    argument.
    """

    name = "file"

    def __init__(self, path: str | None = None) -> None:
        self.path = pathlib.Path(path or os.environ.get(ENVIRONMENT_TOKEN_FILE, DEFAULT_TOKEN_FILE))

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        """Return the token, or None if this environment has no such file.

        Returning None rather than raising is what lets the credential chain
        move on: a source is a claim about the environment, not a demand.
        """
        try:
            token = self.path.read_text().strip()
        except OSError:
            return None
        if not token:
            return None
        return SubjectToken(token=token, source=self.name, token_type=TOKEN_TYPE_ID_TOKEN)


def exchange_with_the_custom_source() -> None:
    """Use the source directly, which reports failure where it happens."""
    host = resolve_host()
    source = FileSubjectTokenSource()

    if source.fetch(host) is None:
        print(f"no subject token at {source.path}; nothing to exchange")
        return

    credential = OIDCCredential(
        api_client=ApiClient(Configuration(host=host)),
        audience=host,
        sources=[source],
    )

    try:
        credential.get_token()
    except AuthError as error:
        # The server rejects a token no configured OIDC provider can verify.
        print(f"exchange refused: {error}")
        return

    me = UsersApi(AuthenticatedApiClient(host, credential)).get_users_me_sync()
    print(f"exchanged via {credential.source}; acting as {me.name}")


def registered_sources_are_found_automatically() -> None:
    """`connect()` tries every registered source, in registration order."""
    print("\nregistered workload sources:")
    for source in default_sources():
        print(f"  {source.name}")

    try:
        client = AuthenticatedApiClient.connect()
    except AuthError as error:
        print(f"no credential resolved: {error}")
        return
    print(f"connect() chose: {client.credential.source}")


def main() -> None:
    exchange_with_the_custom_source()
    registered_sources_are_found_automatically()


if __name__ == "__main__":
    main()
