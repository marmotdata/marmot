"""Bridge between the public auth surface and the generated client.

- Two auth schemes (X-API-Key for keys, Bearer for OAuth/workload tokens)
- Refresh-on-401

Both are handled by customizing the generated ``ApiClient``
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, Final

from typing_extensions import Self

from marmot.auth import (
    Credential,
    Refreshable,
    SecurityScheme,
    resolve_credential,
    resolve_host,
)
from marmot.errors import get_exception_type
from marmot.generated import ApiClient, ApiException, ApiResponse, Configuration, McpApi, rest
from marmot.generated.api_response import T as ApiResponseT

if TYPE_CHECKING:
    from collections.abc import Sequence

    from marmot.auth.workload import WorkloadIdentitySource

_SCHEME_PREFIXES: Final[dict[SecurityScheme, str]] = {
    SecurityScheme.apikey: "",
    SecurityScheme.bearer: "Bearer",
}


class AuthenticatedApiClient(ApiClient):
    def __init__(
        self,
        host: str,
        credential: Credential,
        configuration: Configuration | None = None,
        **kwargs: Any,
    ) -> None:
        if configuration is None:
            configuration = Configuration(host=host.rstrip("/"))
        configuration.api_key_prefix[credential.scheme] = _SCHEME_PREFIXES[credential.scheme]

        super().__init__(configuration, **kwargs)
        self.credential = credential
        self._store_token(credential.get_token())

    @classmethod
    def connect(
        cls,
        host: str | None = None,
        api_key: str | None = None,
        token: str | None = None,
        context_name: str | None = None,
        api_client: ApiClient | None = None,
        sources: Sequence[WorkloadIdentitySource] | None = None,
        **kwargs: Any,
    ) -> Self:
        """Build a client from the first credential that resolves. See :func:`resolve_credential`."""

        resolved_host = resolve_host(host, context_name)
        credential = resolve_credential(
            resolved_host,
            api_key=api_key,
            token=token,
            context_name=context_name,
            api_client=api_client,
            sources=sources,
        )
        return cls(resolved_host, credential, **kwargs)

    async def call_api(
        self,
        method: str,
        url: str,
        header_params: dict[str, str] | None = None,
        *args: Any,
        **kwargs: Any,
    ) -> rest.RESTResponse:
        response = await super().call_api(method, url, header_params, *args, **kwargs)
        if response.status != 401 or not isinstance(self.credential, Refreshable):
            return response

        await response.read()  # type: ignore[no-untyped-call]  # drain before discarding
        self._store_token(await self.credential.refresh())
        self._reapply_auth_headers(header_params)
        return await super().call_api(method, url, header_params, *args, **kwargs)

    def response_deserialize(
        self,
        response_data: rest.RESTResponse,
        response_types_map: dict[str, ApiResponseT] | None = None,
    ) -> ApiResponse[ApiResponseT]:
        try:
            return super().response_deserialize(response_data, response_types_map)
        except ApiException as e:
            raise get_exception_type(e.status)(
                message=e.reason or str(e), status_code=e.status
            ) from e

    def _store_token(self, token: str) -> None:
        self.configuration.api_key[self.credential.scheme] = token

    def _reapply_auth_headers(self, header_params: dict[str, str] | None) -> None:
        """Replace the token ``param_serialize`` applied before we refreshed it.

        Only headers already present are updated: whether an operation sends a
        credential at all is decided from its ``security`` requirement, upstream.
        """
        if header_params is None:
            return
        for setting in self.configuration.auth_settings().values():
            if setting["in"] == "header" and setting["value"] and setting["key"] in header_params:
                header_params[setting["key"]] = setting["value"]


def mcp_url(api_client: ApiClient) -> str:
    """The URL of Marmot's MCP endpoint, for an MCP client's own configuration.

    Read from the generated client so the path stays written down in one place,
    the spec. The generated code keeps each operation's path inside a private
    serializer, hence serializing a request here rather than sending one.
    """
    _, url, *_ = McpApi(api_client)._post_mcp_serialize(
        request_body=None,
        _request_auth=None,
        _content_type=None,
        _headers=None,
        _host_index=0,
    )
    return url
