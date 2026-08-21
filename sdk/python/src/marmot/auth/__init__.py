"""Authentication: credential protocols, the resolution chain and workload identity."""

from marmot.auth.credential import (
    ENVIRONMENT_API_KEY,
    ENVIRONMENT_BEARER_TOKEN,
    Credential,
    Refreshable,
    SecurityScheme,
    StaticCredential,
)
from marmot.auth.oidc import OIDCCredential
from marmot.auth.resolver import (
    CredentialChain,
    resolve_credential,
    resolve_host,
)

__all__ = [
    "ENVIRONMENT_API_KEY",
    "ENVIRONMENT_BEARER_TOKEN",
    "Credential",
    "CredentialChain",
    "OIDCCredential",
    "Refreshable",
    "SecurityScheme",
    "StaticCredential",
    "resolve_credential",
    "resolve_host",
]
