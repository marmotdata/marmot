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

from marmot.auth.workload import SubjectToken, register_source
from marmot.config import ENVIRONMENT_HOST

METADATA_HOST = "metadata.google.internal"
IDENTITY_PATH = "/computeMetadata/v1/instance/service-accounts/default/identity"
METADATA_HEADER = {"Metadata-Flavor": "Google"}


@register_source
@dataclass
class GCPWorkloadIdentitySource:
    name: str = "gcp"
    audience: str | None = None  # defaults to the Marmot host at fetch time
    timeout: float = 2.0

    def fetch(self, audience: str | None = None) -> SubjectToken | None:
        if not _looks_like_gcp():
            return None

        audience = self.audience or audience or os.environ.get(ENVIRONMENT_HOST)
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
        return SubjectToken(token=token, source=self.name)


def _looks_like_gcp() -> bool:
    return any(
        var in os.environ
        for var in (
            "GOOGLE_CLOUD_PROJECT",
            "GCLOUD_PROJECT",
            "K_SERVICE",  # Cloud Run / Cloud Functions
            "FUNCTION_TARGET",
        )
    )
