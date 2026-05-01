"""Workload-identity detectors.

Each source attempts to fetch a JWT/ID-token from a known location (a file,
a metadata server, an env-driven HTTP endpoint). If detection succeeds the
SDK exchanges the token for a Marmot session via RFC 8693.

None of the sources prompt the user; all are silent and return ``None`` when
their environment isn't present.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from marmot.auth.exchange import TOKEN_TYPE_ID_TOKEN


@dataclass(frozen=True)
class SubjectToken:
    """A workload-identity token to be presented as the RFC 8693 subject_token."""

    token: str
    token_type: str = TOKEN_TYPE_ID_TOKEN  # ID tokens are the common case


class WorkloadIdentitySource(Protocol):
    """A source that can produce a subject token without user interaction."""

    name: str

    def fetch(self) -> SubjectToken | None:
        """Return a token if this source's environment is present, else ``None``."""
        ...


def default_sources() -> list[WorkloadIdentitySource]:
    """Return the built-in sources in detection order."""
    from marmot.auth.workload.gcp import GCPWorkloadIdentitySource
    from marmot.auth.workload.github import GitHubActionsSource
    from marmot.auth.workload.kubernetes import KubernetesServiceAccountSource

    return [
        GitHubActionsSource(),
        GCPWorkloadIdentitySource(),
        KubernetesServiceAccountSource(),
    ]
