"""Asset CRUD operations."""

from __future__ import annotations

from typing import Any

from marmot._http import Transport
from marmot.errors import NotFoundError
from marmot.resources import API_PREFIX


class AssetsResource:
    def __init__(self, transport: Transport) -> None:
        self._t = transport

    def get(self, id: str) -> dict[str, Any]:
        """Fetch an asset by its ID."""
        return self._t.get(f"{API_PREFIX}/assets/{id}")

    def lookup(self, *, type: str, service: str, name: str) -> dict[str, Any]:
        """Fetch an asset by its (type, service, name) triple."""
        return self._t.get(f"{API_PREFIX}/assets/lookup/{type}/{service}/{name}")

    def find(self, *, type: str, service: str, name: str) -> dict[str, Any] | None:
        """Like :meth:`lookup` but returns ``None`` instead of raising on 404."""
        try:
            return self.lookup(type=type, service=service, name=name)
        except NotFoundError:
            return None

    def create(self, asset: dict[str, Any]) -> dict[str, Any]:
        """Create a new asset. ``asset`` must include name, type, providers, etc."""
        return self._t.post(f"{API_PREFIX}/assets", json=asset)

    def update(self, id: str, asset: dict[str, Any]) -> dict[str, Any]:
        """Update an existing asset by ID."""
        return self._t.put(f"{API_PREFIX}/assets/{id}", json=asset)

    def delete(self, id: str) -> None:
        """Delete an asset by ID."""
        self._t.delete(f"{API_PREFIX}/assets/{id}")
