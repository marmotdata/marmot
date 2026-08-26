"""Workload-identity detectors.

Each source attempts to fetch a JWT/ID-token from a known location (a file,
a metadata server, an env-driven HTTP endpoint). If detection succeeds the
SDK exchanges the token for a Marmot session via RFC 8693.

None of the sources prompt the user; all are silent and return ``None`` when
their environment isn't present.

This package is a leaf: it produces subject tokens and knows nothing about
credentials or the exchange itself (see :mod:`marmot.auth.oidc`).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol, TypeVar, runtime_checkable

TOKEN_TYPE_ID_TOKEN = "urn:ietf:params:oauth:token-type:id_token"  # noqa: S105 — RFC 8693 URN, not a credential

T = TypeVar("T", bound="WorkloadIdentitySource")

WORKLOAD_REGISTRY: dict[str, type[WorkloadIdentitySource]] = {}


@dataclass(frozen=True)
class SubjectToken:
    """A workload-identity token to be presented as the RFC 8693 subject_token."""

    token: str
    source: str = ""
    token_type: str = TOKEN_TYPE_ID_TOKEN


@runtime_checkable
class WorkloadIdentitySource(Protocol):
    """A source that can produce a subject token without user interaction."""

    name: str

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        """Return a token if this source's environment is present, else ``None``.

        ``audience`` is the resolved Marmot host; sources that need one may fall
        back to their own configuration when it is ``None``.
        """
        ...


def register_source(cls: type[T]) -> type[T]:
    WORKLOAD_REGISTRY[cls.name] = cls
    return cls


def unregister_source(name: str) -> bool:
    """Remove a source from the registry. True if it was registered."""

    return WORKLOAD_REGISTRY.pop(name, None) is not None


def default_sources() -> list[WorkloadIdentitySource]:
    return [cls() for cls in WORKLOAD_REGISTRY.values()]


from marmot.auth.workload import gcp, github, kubernetes  # noqa: E402, F401, I001 — registration side effect
