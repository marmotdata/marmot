"""RFC 8693 token exchange against Marmot's /oauth/token endpoint."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING

import httpx

from marmot.errors import AuthError, ServerError

if TYPE_CHECKING:
    from marmot.auth import Credential

GRANT_TYPE = "urn:ietf:params:oauth:grant-type:token-exchange"
TOKEN_TYPE_ID_TOKEN = "urn:ietf:params:oauth:token-type:id_token"
TOKEN_TYPE_ACCESS_TOKEN = "urn:ietf:params:oauth:token-type:access_token"

_DEFAULT_TIMEOUT = 10.0


def exchange(
    *,
    base_url: str,
    subject_token: str,
    subject_token_type: str,
    source_name: str,
    timeout: float = _DEFAULT_TIMEOUT,
    client: httpx.Client | None = None,
) -> Credential:
    """Exchange a workload-identity token for a Marmot session token.

    Returns a :class:`marmot.auth.Credential` whose ``refresh`` callback
    re-runs the exchange — useful for long-lived agents.
    """
    from marmot.auth import Credential  # avoid circular import

    def _do_exchange() -> Credential:
        url = base_url.rstrip("/") + "/oauth/token"
        data = {
            "grant_type": GRANT_TYPE,
            "subject_token": subject_token,
            "subject_token_type": subject_token_type,
        }

        owns_client = client is None
        c = client or httpx.Client(timeout=timeout)
        try:
            resp = c.post(url, data=data)
        finally:
            if owns_client:
                c.close()

        if resp.status_code == 400:
            raise AuthError(_oauth_error(resp))
        if resp.status_code == 401:
            raise AuthError(
                f"server rejected workload-identity token from {source_name}: {_oauth_error(resp)}"
            )
        if resp.status_code >= 500:
            raise ServerError(
                f"token exchange failed: HTTP {resp.status_code}", status_code=resp.status_code
            )
        if resp.status_code != 200:
            raise ServerError(
                f"unexpected response from /oauth/token: HTTP {resp.status_code}",
                status_code=resp.status_code,
            )

        body = resp.json()
        access = body.get("access_token")
        if not isinstance(access, str) or not access:
            raise ServerError("token exchange response missing access_token")

        expires_in = body.get("expires_in")
        expires_at: datetime | None = None
        if isinstance(expires_in, int) and expires_in > 0:
            expires_at = datetime.now(timezone.utc) + timedelta(seconds=expires_in)

        return Credential(
            token=access,
            scheme="Bearer",
            expires_at=expires_at,
            refresh=_do_exchange,
            source=f"token exchange via {source_name}",
        )

    return _do_exchange()


def _oauth_error(resp: httpx.Response) -> str:
    """Format an RFC 6749 error response."""
    try:
        body = resp.json()
    except ValueError:
        return resp.text or f"HTTP {resp.status_code}"

    err = body.get("error", "")
    desc = body.get("error_description", "")
    if err and desc:
        return f"{err}: {desc}"
    return err or desc or resp.text or f"HTTP {resp.status_code}"
