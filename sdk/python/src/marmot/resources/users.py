"""User listing and identity queries."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.users import get_users, get_users_id, get_users_me
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.list_users_response import ListUsersResponse
from marmot._gen.models.user import User
from marmot._gen.types import UNSET, Unset


class UsersResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(
        self,
        *,
        query: str | None = None,
        active: bool | None = None,
        role_ids: list[str] | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> ListUsersResponse:
        """Return paginated users."""
        query_arg: str | Unset = query if query is not None else UNSET
        active_arg: bool | Unset = active if active is not None else UNSET
        role_ids_arg: list[str] | Unset = role_ids if role_ids else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            ListUsersResponse,
            unwrap(
                get_users.sync_detailed(
                    client=self._c,
                    query=query_arg,
                    active=active_arg,
                    role_ids=role_ids_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )

    def get(self, user_id: str) -> User:
        """Fetch a user by ID."""
        return cast(User, unwrap(get_users_id.sync_detailed(id=user_id, client=self._c)))

    def me(self) -> User:
        """Return the currently authenticated user."""
        return cast(User, unwrap(get_users_me.sync_detailed(client=self._c)))
