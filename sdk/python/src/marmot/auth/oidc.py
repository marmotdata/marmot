"""RFC 8693 token exchange for workload identity.

A subject token from any :class:`WorkloadIdentitySource` is exchanged for a
Marmot session token. The exchange is retried on demand, so a token rejected
before its stated expiry (revoked, rotated signing key, clock skew) still
recovers.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING

from marmot.auth.credential import SecurityScheme
from marmot.auth.workload import SubjectToken, default_sources
from marmot.errors import AuthError
from marmot.generated import ApiClient, ApiResponse, AuthApi, TokenExchangeResponse
from marmot.generated.sync_helper import run_sync

if TYPE_CHECKING:
    from collections.abc import Sequence

    from marmot.auth.workload import WorkloadIdentitySource

GRANT_TYPE_TOKEN_EXCHANGE = "urn:ietf:params:oauth:grant-type:token-exchange"  # noqa: S105 — RFC 8693 URN, not a credential
_EXPIRY_LEEWAY = timedelta(seconds=30)


class OIDCCredential:
    """A bearer credential obtained by exchanging a workload-identity token."""

    scheme = SecurityScheme.bearer

    def __init__(
        self,
        api_client: ApiClient,
        audience: str | None = None,
        sources: Sequence[WorkloadIdentitySource] | None = None,
    ) -> None:
        self.auth_api = AuthApi(api_client=api_client)
        self.audience = audience
        self.sources = list(sources) if sources is not None else default_sources()
        self.source = "workload identity"
        self._token: str | None = None
        self._expires_at: datetime | None = None

    @property
    def is_stale(self) -> bool:
        return self._token is None or self._is_expired()

    def get_token(self) -> str:
        """Exchange a subject token, reusing the last one until it expires.

        Blocks the calling thread on the same shared loop the generated ``*_sync``
        methods use; from async code ``await refresh()`` instead.
        """
        if not self.is_stale:
            return self._token  # type: ignore[return-value]  # is_stale covers None
        return run_sync(self.refresh())

    async def refresh(self) -> str:
        """Exchange a new subject token, discarding any token held."""

        subject = self._subject_token()
        try:
            response = await self.auth_api.oauth_token_post_with_http_info(
                grant_type=GRANT_TYPE_TOKEN_EXCHANGE,
                subject_token=subject.token,
                subject_token_type=subject.token_type,
            )
        except Exception as e:
            raise AuthError("Error exchanging token with API") from e
        return self._store(response, subject)

    def _subject_token(self) -> SubjectToken:
        for source in self.sources:
            if (subject := source.fetch(self.audience)) is not None:
                return subject
        raise AuthError(
            f"No subject token was found to exchange (tried: "
            f"{', '.join(source.name for source in self.sources) or 'no sources registered'})"
        )

    def _store(self, response: ApiResponse[TokenExchangeResponse], subject: SubjectToken) -> str:
        if not response.data.access_token:
            raise AuthError("Did not receive token after API exchange")

        self._token = response.data.access_token
        self._expires_at = None
        if response.data.expires_in and response.data.expires_in > 0:
            self._expires_at = self._now() + timedelta(seconds=response.data.expires_in)
        self.source = f"workload identity ({subject.source})"
        return self._token

    def _is_expired(self) -> bool:
        if self._expires_at is None:
            return False  # no expiry advertised; refresh-on-401 covers rejection
        return self._now() >= self._expires_at - _EXPIRY_LEEWAY

    @staticmethod
    def _now() -> datetime:
        return datetime.now(timezone.utc)
