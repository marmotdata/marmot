"""Quickstart for the Marmot Python SDK.

Run against a Marmot instance you've already authenticated with:

    marmot login http://localhost:5173
    uv run python examples/quickstart.py
"""

from __future__ import annotations

import marmot


def main() -> None:
    # Auth resolves automatically from ~/.config/marmot/credentials.json,
    # env vars, or workload identity. No interactive prompts.
    with marmot.connect() as client:
        print(f"Connected to {client.base_url}")

        # Search the catalog.
        results = client.search("postgres", limit=3)
        hits = results.get("results") or []
        print(f"\nSearch found {len(hits)} matches:")
        for hit in hits:
            print(f"  - {hit.get('name')} ({hit.get('type')}/{hit.get('asset_type')})")

        if not hits:
            return

        # Fetch one asset by ID.
        asset = client.assets.get(hits[0]["id"])
        print(f"\nFetched asset: {asset.get('name')}")
        print(f"  MRN: {asset.get('mrn')}")
        print(f"  Owner: {(asset.get('metadata') or {}).get('owner', '<none>')}")


if __name__ == "__main__":
    main()
