"""GitHub Actions OIDC source.

When a workflow has ``id-token: write`` permission, GitHub injects two env
vars and a runtime endpoint that vends an OIDC token. The token's audience
must match what Marmot is configured to accept.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

import httpx

from marmot.auth.workload import SubjectToken


@dataclass
class GitHubActionsSource:
    name: str = "github-actions"
    audience: str | None = None  # defaults to the Marmot host at fetch time
    timeout: float = 5.0

    def fetch(self) -> SubjectToken | None:
        url = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_URL")
        bearer = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
        if not url or not bearer:
            return None

        audience = self.audience or os.environ.get("MARMOT_HOST")
        params = {"audience": audience} if audience else {}

        try:
            resp = httpx.get(
                url,
                params=params,
                headers={"Authorization": f"Bearer {bearer}"},
                timeout=self.timeout,
            )
        except httpx.HTTPError:
            return None

        if resp.status_code != 200:
            return None

        try:
            token = resp.json().get("value")
        except ValueError:
            return None

        if not isinstance(token, str) or not token:
            return None
        return SubjectToken(token=token)
