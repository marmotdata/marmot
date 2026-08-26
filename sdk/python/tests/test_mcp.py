"""The MCP URL comes from the spec, via the generated client."""

from __future__ import annotations

from marmot import AuthenticatedApiClient, mcp_url
from marmot.auth import SecurityScheme, StaticCredential

HOST = "http://x"


def test_mcp_url_is_read_from_the_generated_client() -> None:
    client = AuthenticatedApiClient(HOST, StaticCredential("key", SecurityScheme.apikey))

    assert mcp_url(client) == f"{HOST}/api/v1/mcp"


def test_mcp_url_does_not_double_a_host_that_ends_in_a_slash() -> None:
    client = AuthenticatedApiClient(f"{HOST}/", StaticCredential("key", SecurityScheme.apikey))

    assert mcp_url(client) == f"{HOST}/api/v1/mcp"
