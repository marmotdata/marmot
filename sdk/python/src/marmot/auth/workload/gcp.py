"""GCP workload identity source.

On GCE / GKE / Cloud Run / Cloud Functions, Google's metadata server vends an
identity token for the active service account when queried with the right
audience. We pass the Marmot host as the audience so the token can only be
replayed against this server.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

import httpx

from marmot.auth.workload import SubjectToken

METADATA_HOST = "metadata.google.internal"
IDENTITY_PATH = "/computeMetadata/v1/instance/service-accounts/default/identity"
METADATA_HEADER = {"Metadata-Flavor": "Google"}


@dataclass
class GCPWorkloadIdentitySource:
    name: str = "gcp"
    audience: str | None = None  # defaults to the Marmot host at fetch time
    timeout: float = 2.0

    def fetch(self) -> SubjectToken | None:
        # Cheap presence check — avoids a multi-second hang on machines without
        # the metadata server. The env var is set on every Google compute env
        # by the runtime; if it's absent we skip immediately.
        if not _looks_like_gcp():
            return None

        audience = self.audience or os.environ.get("MARMOT_HOST")
        if not audience:
            return None

        try:
            resp = httpx.get(
                f"http://{METADATA_HOST}{IDENTITY_PATH}",
                params={"audience": audience, "format": "full"},
                headers=METADATA_HEADER,
                timeout=self.timeout,
            )
        except httpx.HTTPError:
            return None

        if resp.status_code != 200:
            return None

        token = resp.text.strip()
        if not token:
            return None
        return SubjectToken(token=token)


def _looks_like_gcp() -> bool:
    """Quick env signal — true on every Google managed runtime."""
    return any(
        var in os.environ
        for var in (
            "GOOGLE_CLOUD_PROJECT",
            "GCLOUD_PROJECT",
            "K_SERVICE",  # Cloud Run / Cloud Functions
            "FUNCTION_TARGET",
        )
    )
