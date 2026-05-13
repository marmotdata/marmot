"""Personal API key management for the authenticated user."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.users import (
    delete_users_apikeys_id,
    get_users_apikeys,
    post_users_apikeys,
)
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.api_key import APIKey
from marmot._gen.models.create_api_key_request import CreateAPIKeyRequest
from marmot._gen.types import UNSET, Unset


class APIKeysResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self) -> list[APIKey]:
        """Return all API keys for the authenticated user."""
        return cast(
            list[APIKey],
            unwrap(get_users_apikeys.sync_detailed(client=self._c)),
        )

    def create(self, *, name: str, expires_in_days: int = 0) -> APIKey:
        """Issue a new API key. The token field is only readable on this response."""
        expires: int | Unset = expires_in_days if expires_in_days > 0 else UNSET
        body = CreateAPIKeyRequest(name=name, expires_in_days=expires)
        return cast(
            APIKey,
            unwrap(post_users_apikeys.sync_detailed(client=self._c, body=body)),
        )

    def delete(self, key_id: str) -> None:
        """Revoke an API key."""
        unwrap(delete_users_apikeys_id.sync_detailed(id=key_id, client=self._c))
