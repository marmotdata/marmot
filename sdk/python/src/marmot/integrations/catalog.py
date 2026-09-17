"""The slice of Marmot that integrations use.

Both the LangChain and Claude Agent SDK integrations register an agent asset,
record runs and write lineage edges. They depend on the protocols here rather
than on the generated client, so integration code carries no endpoint knowledge
and can be driven by a fake in tests.
"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING, Any, Protocol, runtime_checkable
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field

from marmot.errors import NotFoundError
from marmot.generated import (
    AgentsApi,
    Asset,
    AssetsApi,
    CreateAssetRequest,
    LineageApi,
    LineageEdge,
    LineageResponse,
    RecordRunRequest,
    SearchApi,
    SearchResponse,
    ToolCallPayload,
    UpdateAssetRequest,
)

if TYPE_CHECKING:
    from collections.abc import Sequence

    from marmot.generated import ApiClient

AGENT_ASSET_TYPE = "Agent"
AGENT_INVOKES = "AGENT_INVOKES"


def _isoformat(moment: datetime) -> str:
    """The API expects RFC 3339; naive datetimes are assumed to be UTC."""

    return f"{moment.isoformat()}Z" if moment.tzinfo is None else moment.isoformat()


class ToolCall(BaseModel):
    """One tool invocation observed during an agent run."""

    model_config = ConfigDict(frozen=True)

    tool_name: str
    started_at: datetime
    duration_ms: int | None = Field(default=None, ge=0)  # None when the start was not observed
    status: str = "success"
    target_mrn: str | None = None

    def to_payload(self) -> ToolCallPayload:
        return ToolCallPayload(
            tool_name=self.tool_name,
            started_at=_isoformat(self.started_at),
            duration_ms=self.duration_ms,
            status=self.status,
            target_mrn=self.target_mrn,
        )


class AgentRunRecord(BaseModel):
    """A completed agent run, as reported to ``POST /agents/runs``."""

    model_config = ConfigDict(frozen=True)

    agent_mrn: str
    run_id: str
    started_at: datetime
    status: str
    ended_at: datetime | None = None
    model: str | None = None
    tokens_in: int = 0
    tokens_out: int = 0
    error: str | None = None
    tool_calls: list[ToolCall] = Field(default_factory=list)
    observed_assets: list[str] = Field(default_factory=list)

    def to_request(self) -> RecordRunRequest:
        return RecordRunRequest(
            agent_mrn=self.agent_mrn,
            run_id=self.run_id,
            started_at=_isoformat(self.started_at),
            ended_at=_isoformat(self.ended_at) if self.ended_at else None,
            status=self.status,
            model=self.model,
            tokens_in=self.tokens_in,
            tokens_out=self.tokens_out,
            error=self.error,
            tool_calls=[call.to_payload() for call in self.tool_calls] or None,
            observed_assets=list(self.observed_assets) or None,
        )


class AgentSpec(BaseModel):
    """An agent asset: what identifies it, and what we describe it with.

    ``(service, name)`` is the natural key the catalog is looked up by, so it
    must stay stable across runs; everything else is metadata.
    """

    model_config = ConfigDict(frozen=True)

    name: str = Field(min_length=1)
    service: str = Field(min_length=1)
    framework: str
    model: str | None = None
    version: str | None = None
    owner: str | None = None
    tool_names: list[str] | None = None
    system_prompt_hash: str | None = None
    extra_metadata: dict[str, Any] = Field(default_factory=dict)

    def metadata(self) -> dict[str, Any]:
        described = {
            "framework": self.framework,
            "model": self.model,
            "version": self.version,
            "owner": self.owner,
            "tool_names": self.tool_names,
            "system_prompt_sha256_16": self.system_prompt_hash,
        }
        return {**{k: v for k, v in described.items() if v}, **self.extra_metadata}

    def to_create_request(self) -> CreateAssetRequest:
        return CreateAssetRequest(
            name=self.name,
            type=AGENT_ASSET_TYPE,
            providers=[self.service],
            metadata=self.metadata(),
        )

    def to_update_request(self) -> UpdateAssetRequest:
        return UpdateAssetRequest(metadata=self.metadata())


@runtime_checkable
class CatalogReader(Protocol):
    """Catalog read access, as exposed to agent tools."""

    async def asearch(self, query: str, *, limit: int = 20) -> SearchResponse: ...

    async def aget_asset(self, asset_id: str) -> Asset: ...

    async def alookup_asset(self, *, asset_type: str, service: str, name: str) -> Asset | None: ...

    async def aget_upstream_lineage(self, asset_id: str, *, depth: int = 2) -> LineageResponse: ...


@runtime_checkable
class AgentRegistry(Protocol):
    """Catalog write access for agent telemetry."""

    async def aregister_agent(self, spec: AgentSpec) -> Asset:
        """Create or update the agent's asset and return it."""
        ...

    async def arecord_run(self, run: AgentRunRecord) -> None: ...

    async def awrite_edges(self, edges: Sequence[LineageEdge]) -> None: ...


class MarmotCatalog:
    """Adapts the generated client to the protocols above.

    Hand one to any integration::

        client = AuthenticatedApiClient.connect()
        catalog = MarmotCatalog(client)
    """

    def __init__(self, client: ApiClient) -> None:
        self._assets = AssetsApi(api_client=client)
        self._agents = AgentsApi(api_client=client)
        self._lineage = LineageApi(api_client=client)
        self._search = SearchApi(api_client=client)

    async def asearch(self, query: str, *, limit: int = 20) -> SearchResponse:
        return await self._search.get_search(q=query, limit=limit)

    async def aget_asset(self, asset_id: str) -> Asset:
        return await self._assets.get_assets_id(id=asset_id)

    async def alookup_asset(self, *, asset_type: str, service: str, name: str) -> Asset | None:
        try:
            return await self._assets.get_assets_lookup_type_service_name(
                type=asset_type, service=service, name=name
            )
        except NotFoundError:
            return None

    async def aget_upstream_lineage(self, asset_id: str, *, depth: int = 2) -> LineageResponse:
        return await self._lineage.get_lineage_assets_id(id=UUID(asset_id), limit=depth)

    async def aregister_agent(self, spec: AgentSpec) -> Asset:
        try:
            existing = await self._assets.get_assets_lookup_type_service_name(
                type=AGENT_ASSET_TYPE, service=spec.service, name=spec.name
            )
        except NotFoundError:
            existing = None
        if existing is None:
            return await self._assets.post_assets(create_asset_request=spec.to_create_request())
        if existing.id:
            return await self._assets.put_assets_id(
                id=existing.id, update_asset_request=spec.to_update_request()
            )
        return existing

    async def arecord_run(self, run: AgentRunRecord) -> None:
        await self._agents.post_agents_runs(record_run_request=run.to_request())

    async def awrite_edges(self, edges: Sequence[LineageEdge]) -> None:
        if edges:
            await self._lineage.post_lineage_batch(lineage_edge=list(edges))
