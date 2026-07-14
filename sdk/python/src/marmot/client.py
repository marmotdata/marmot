"""Public client surface for the Marmot SDK."""

from __future__ import annotations

from typing import Any

import httpx

from marmot._adapter import make_gen_client, make_marmot_auth
from marmot.auth import Credential, resolve
from marmot.auth.workload import WorkloadIdentitySource
from marmot.resources.admin import AdminResource
from marmot.resources.agent_runs import AgentRunsResource
from marmot.resources.api_keys import APIKeysResource
from marmot.resources.assets import AssetsResource
from marmot.resources.glossary import GlossaryResource
from marmot.resources.lineage import LineageResource
from marmot.resources.metrics import MetricsResource
from marmot.resources.owners import OwnersResource
from marmot.resources.runs import RunsResource
from marmot.resources.search import SearchResource
from marmot.resources.teams import TeamsResource
from marmot.resources.users import UsersResource

_USER_AGENT = "marmot-sdk-py"


class Client:
    """Marmot client.

    Construct via :func:`connect`. Each resource is exposed as an attribute
    (``client.assets``, ``client.glossary``, ...). ``client.search`` is a
    callable shortcut for ``client.search.query``.
    """

    def __init__(
        self,
        *,
        base_url: str,
        credential: Credential,
        timeout: float = 30.0,
        http_client: httpx.Client | None = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._http = http_client if http_client is not None else httpx.Client(timeout=timeout)
        self._owns_http = http_client is None
        self._http.auth = make_marmot_auth(credential)
        self._http.base_url = httpx.URL(f"{self._base_url}/api/v1")
        self._http.headers.setdefault("User-Agent", _USER_AGENT)
        self._gen = make_gen_client(self._base_url, self._http)
        self.admin = AdminResource(self._gen)
        self.agent_runs = AgentRunsResource(self._gen)
        self.api_keys = APIKeysResource(self._gen)
        self.assets = AssetsResource(self._gen)
        self.glossary = GlossaryResource(self._gen)
        self.lineage = LineageResource(self._gen)
        self.metrics = MetricsResource(self._gen)
        self.owners = OwnersResource(self._gen)
        self.runs = RunsResource(self._gen)
        self.search = SearchResource(self._gen)
        self.teams = TeamsResource(self._gen)
        self.users = UsersResource(self._gen)

    @property
    def base_url(self) -> str:
        return self._base_url

    def close(self) -> None:
        if self._owns_http:
            self._http.close()

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
