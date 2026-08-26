"""RFC 8693 token exchange for workload identity.

A subject token from any :class:`WorkloadIdentitySource` is exchanged for a
Marmot session token. The exchange is retried on demand, so a token rejected
before its stated expiry (revoked, rotated signing key, clock skew) still
recovers.
"""

from __future__ import annotations

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

    def get_token(self) -> str:
        """Return the token held, exchanging one if there is none yet.

        Blocks the calling thread on the same shared loop the generated ``*_sync``
        methods use; from async code ``await refresh()`` instead. A token the API
        later rejects is replaced by the refresh-on-401 path, not here.
        """
        if self._token is not None:
            return self._token
        return run_sync(self.refresh())

    async def refresh(self) -> str:
        """Exchange a new subject token, discarding any token held.

        Concurrent rejected requests each exchange their own token. Marmot issues
        stateless JWTs, so the tokens are independent and the last one stored wins.
        """
        subject = self._subject_token()
        try:
            response = await self.auth_api.post_oauth_token_with_http_info(
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
        self.source = f"workload identity ({subject.source})"
        return self._token
