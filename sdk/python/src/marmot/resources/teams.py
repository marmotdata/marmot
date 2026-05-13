"""List teams and their members."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.teams import get_teams, get_teams_id, get_teams_id_members
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.list_members_response import ListMembersResponse
from marmot._gen.models.list_teams_response import ListTeamsResponse
from marmot._gen.models.team import Team
from marmot._gen.types import UNSET, Unset


class TeamsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(self, *, limit: int | None = None, offset: int | None = None) -> ListTeamsResponse:
        """Return paginated teams."""
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            ListTeamsResponse,
            unwrap(get_teams.sync_detailed(client=self._c, limit=limit_arg, offset=offset_arg)),
        )

    def get(self, team_id: str) -> Team:
        """Fetch a team by ID."""
        return cast(Team, unwrap(get_teams_id.sync_detailed(id=team_id, client=self._c)))

    def members(self, team_id: str) -> ListMembersResponse:
        """Return the members of a team."""
        return cast(
            ListMembersResponse,
            unwrap(get_teams_id_members.sync_detailed(id=team_id, client=self._c)),
        )
