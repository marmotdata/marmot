"""Python SDK for Marmot."""

from marmot.auth import ENVIRONMENT_API_KEY, ENVIRONMENT_BEARER_TOKEN, Credential, SecurityScheme
from marmot.client import AuthenticatedApiClient, mcp_url
from marmot.config import ENVIRONMENT_HOST, ENVIRONMENT_MARMOT_CONTEXT
from marmot.errors import (
    AuthError,
    MarmotError,
    NotFoundError,
    RateLimitError,
    ServerError,
    ValidationError,
)
from marmot.generated import (
    AdminApi,
    AgentsApi,
    ApiClient,
    ApiException,
    AssetRulesApi,
    AssetsApi,
    AuthApi,
    Configuration,
    GlossaryApi,
    IngestionApi,
    LineageApi,
    MetricsApi,
    OwnersApi,
    PipelinesApi,
    PluginsApi,
    ProductsApi,
    RolesApi,
    RunsApi,
    SearchApi,
    ServiceAccountsApi,
    SsoApi,
    TeamsApi,
    UiApi,
    UsersApi,
)

__all__ = [  # noqa: RUF022 — grouped by origin, not alphabetised
    # Wrappers/helpers
    "ENVIRONMENT_API_KEY",
    "ENVIRONMENT_BEARER_TOKEN",
    "ENVIRONMENT_HOST",
    "ENVIRONMENT_MARMOT_CONTEXT",
    "AuthenticatedApiClient",
    "mcp_url",
    "SecurityScheme",
    # Generated
    "AdminApi",
    "AgentsApi",
    "ApiClient",
    "ApiException",
    "AssetRulesApi",
    "AssetsApi",
    "AuthApi",
    "AuthError",
    "Configuration",
    "Credential",
    "GlossaryApi",
    "IngestionApi",
    "LineageApi",
    "MarmotError",
    "MetricsApi",
    "NotFoundError",
    "OwnersApi",
    "PipelinesApi",
    "PluginsApi",
    "ProductsApi",
    "RateLimitError",
    "RolesApi",
    "RunsApi",
    "SearchApi",
    "ServerError",
    "ServiceAccountsApi",
    "SsoApi",
    "TeamsApi",
    "UiApi",
    "UsersApi",
    "ValidationError",
]

__version__ = "0.4.0"
