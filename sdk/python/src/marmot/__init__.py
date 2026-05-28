"""Python SDK for Marmot."""

from marmot._gen.models.activity_response import ActivityResponse
from marmot._gen.models.agent_run import AgentRun
from marmot._gen.models.api_key import APIKey
from marmot._gen.models.asset import Asset
from marmot._gen.models.asset_count import AssetCount
from marmot._gen.models.asset_search_response import AssetSearchResponse
from marmot._gen.models.asset_summary_response import AssetSummaryResponse
from marmot._gen.models.assets_by_provider_response import AssetsByProviderResponse
from marmot._gen.models.assets_by_type_response import AssetsByTypeResponse
from marmot._gen.models.batch_lineage_result import BatchLineageResult
from marmot._gen.models.create_api_key_request import CreateAPIKeyRequest
from marmot._gen.models.create_asset_request import CreateAssetRequest
from marmot._gen.models.create_term_request import CreateTermRequest
from marmot._gen.models.get_runs_response_200 import GetRunsResponse200 as ListRunsResponse
from marmot._gen.models.glossary_list_result import GlossaryListResult
from marmot._gen.models.glossary_term import GlossaryTerm
from marmot._gen.models.lineage_edge import LineageEdge
from marmot._gen.models.lineage_response import LineageResponse
from marmot._gen.models.list_members_response import ListMembersResponse
from marmot._gen.models.list_teams_response import ListTeamsResponse
from marmot._gen.models.list_users_response import ListUsersResponse
from marmot._gen.models.plugin_run import PluginRun as Run
from marmot._gen.models.query_count import QueryCount
from marmot._gen.models.record_run_request import RecordRunRequest
from marmot._gen.models.reindex_accepted_response import ReindexAcceptedResponse
from marmot._gen.models.reindex_status_response import ReindexStatusResponse
from marmot._gen.models.run_entities_response import RunEntitiesResponse
from marmot._gen.models.run_entity import RunEntity
from marmot._gen.models.runs_response import RunsResponse
from marmot._gen.models.search_owners_response import SearchOwnersResponse
from marmot._gen.models.search_response import SearchResponse
from marmot._gen.models.stats import Stats
from marmot._gen.models.tag_request import TagRequest
from marmot._gen.models.team import Team
from marmot._gen.models.tool_call_payload import ToolCallPayload
from marmot._gen.models.total_assets_response import TotalAssetsResponse
from marmot._gen.models.update_asset_request import UpdateAssetRequest
from marmot._gen.models.update_term_request import UpdateTermRequest
from marmot._gen.models.user import User
from marmot._gen.types import UNSET, Unset
from marmot.auth import Credential, resolve
from marmot.client import Client, connect
from marmot.errors import (
    AuthError,
    MarmotError,
    NotFoundError,
    RateLimitError,
    ServerError,
    ValidationError,
    is_not_found,
    is_rate_limit,
)

__all__ = [
    "UNSET",
    "APIKey",
    "ActivityResponse",
    "AgentRun",
    "Asset",
    "AssetCount",
    "AssetSearchResponse",
    "AssetSummaryResponse",
    "AssetsByProviderResponse",
    "AssetsByTypeResponse",
    "AuthError",
    "BatchLineageResult",
    "Client",
    "CreateAPIKeyRequest",
    "CreateAssetRequest",
    "CreateTermRequest",
    "Credential",
    "GlossaryListResult",
    "GlossaryTerm",
    "LineageEdge",
    "LineageResponse",
    "ListMembersResponse",
    "ListRunsResponse",
    "ListTeamsResponse",
    "ListUsersResponse",
    "MarmotError",
    "NotFoundError",
    "QueryCount",
    "RateLimitError",
    "RecordRunRequest",
    "ReindexAcceptedResponse",
    "ReindexStatusResponse",
    "Run",
    "RunEntitiesResponse",
    "RunEntity",
    "RunsResponse",
    "SearchOwnersResponse",
    "SearchResponse",
    "ServerError",
    "Stats",
    "TagRequest",
    "Team",
    "ToolCallPayload",
    "TotalAssetsResponse",
    "Unset",
    "UpdateAssetRequest",
    "UpdateTermRequest",
    "User",
    "ValidationError",
    "connect",
    "is_not_found",
    "is_rate_limit",
    "resolve",
]

__version__ = "0.3.0"
