"""List and inspect pipeline ingestion runs."""

from __future__ import annotations

from typing import cast

from marmot._adapter import unwrap
from marmot._gen.api.runs import get_runs, get_runs_id, get_runs_id_entities
from marmot._gen.client import AuthenticatedClient
from marmot._gen.models.get_runs_response_200 import GetRunsResponse200
from marmot._gen.models.plugin_run import PluginRun
from marmot._gen.models.run_entities_response import RunEntitiesResponse
from marmot._gen.types import UNSET, Unset


class RunsResource:
    def __init__(self, client: AuthenticatedClient) -> None:
        self._c = client

    def list(
        self,
        *,
        pipelines: str | None = None,
        statuses: str | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> GetRunsResponse200:
        """Return paginated runs. ``pipelines`` and ``statuses`` are comma-separated."""
        pipelines_arg: str | Unset = pipelines if pipelines is not None else UNSET
        statuses_arg: str | Unset = statuses if statuses is not None else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            GetRunsResponse200,
            unwrap(
                get_runs.sync_detailed(
                    client=self._c,
                    pipelines=pipelines_arg,
                    statuses=statuses_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )

    def get(self, run_id: str) -> PluginRun:
        """Fetch a single run by its database ID."""
        return cast(PluginRun, unwrap(get_runs_id.sync_detailed(id=run_id, client=self._c)))

    def entities(
        self,
        run_id: str,
        *,
        entity_type: str | None = None,
        status: str | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> RunEntitiesResponse:
        """List the entities processed in a run."""
        et_arg: str | Unset = entity_type if entity_type is not None else UNSET
        status_arg: str | Unset = status if status is not None else UNSET
        limit_arg: int | Unset = limit if limit is not None else UNSET
        offset_arg: int | Unset = offset if offset is not None else UNSET
        return cast(
            RunEntitiesResponse,
            unwrap(
                get_runs_id_entities.sync_detailed(
                    id=run_id,
                    client=self._c,
                    entity_type=et_arg,
                    status=status_arg,
                    limit=limit_arg,
                    offset=offset_arg,
                )
            ),
        )
