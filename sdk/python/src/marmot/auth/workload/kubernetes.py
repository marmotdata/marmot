"""Kubernetes service-account token source.

When running inside a Kubernetes pod, kubelet projects a JWT signed by the
cluster's service-account issuer at a well-known path. The cluster's issuer
must be configured as a TokenExchanger provider in Marmot.
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from marmot.auth.workload import SubjectToken

DEFAULT_TOKEN_PATH = Path("/var/run/secrets/kubernetes.io/serviceaccount/token")


@dataclass
class KubernetesServiceAccountSource:
    name: str = "kubernetes"
    token_path: Path = DEFAULT_TOKEN_PATH

    def fetch(self) -> SubjectToken | None:
        try:
            raw = self.token_path.read_text().strip()
        except (FileNotFoundError, PermissionError, OSError):
            return None
        if not raw:
            return None
        return SubjectToken(token=raw)
