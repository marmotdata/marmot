"""Public client surface for the Marmot SDK."""

from __future__ import annotations

from typing import Any

import httpx

from marmot._http import Transport
from marmot.auth import Credential, resolve
from marmot.auth.workload import WorkloadIdentitySource
from marmot.resources.assets import AssetsResource
from marmot.resources.lineage import LineageResource
from marmot.resources.search import SearchResource


class Client:
    """Marmot client.

    Construct via :func:`connect`. The client exposes resource namespaces
    (``client.assets``, ``client.lineage``) plus a top-level :meth:`search`.
    """

    def __init__(
        self,
        *,
        base_url: str,
        credential: Credential,
        timeout: float = 30.0,
        http_client: httpx.Client | None = None,
    ) -> None:
        self._transport = Transport(
            base_url=base_url,
            credential=credential,
            timeout=timeout,
            client=http_client,
        )
        self.assets = AssetsResource(self._transport)
        self.lineage = LineageResource(self._transport)
        self._search = SearchResource(self._transport)

    @property
    def base_url(self) -> str:
        return self._transport.base_url

    def search(self, query: str, **kwargs: Any) -> dict[str, Any]:
        """Run a catalog search. See :class:`SearchResource` for kwargs."""
        return self._search(query, **kwargs)

    def close(self) -> None:
        self._transport.close()

    def __enter__(self) -> Client:
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()


def connect(
    *,
    base_url: str | None = None,
    token: str | None = None,
    api_key: str | None = None,
    context: str | None = None,
    timeout: float = 30.0,
    workload_sources: list[WorkloadIdentitySource] | None = None,
) -> Client:
    """Construct a Marmot client with credentials resolved from the standard chain.

    Resolution order:

    1. Explicit ``api_key`` / ``token`` kwargs
    2. Env vars: ``MARMOT_API_KEY``, ``MARMOT_TOKEN``, ``MARMOT_HOST``, ``MARMOT_CONTEXT``
    3. Cached OAuth token in ``~/.config/marmot/credentials.json`` (written by ``marmot login``)
    4. Workload identity → RFC 8693 token exchange

    Raises :class:`marmot.AuthError` if no credential resolves.
    """
    resolved_url, cred = resolve(
        base_url=base_url,
        api_key=api_key,
        token=token,
        context=context,
        workload_sources=workload_sources,
    )
    return Client(base_url=resolved_url, credential=cred, timeout=timeout)
