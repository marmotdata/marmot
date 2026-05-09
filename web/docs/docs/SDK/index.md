---
sidebar_position: 1
---

# SDK

Official SDKs for **Python** and **TypeScript** wrap the REST API with credential resolution, typed resources for assets and lineage and a search helper.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Tabs, TabPanel } from '@site/src/components/Steps';

<DocCardGrid>
  <DocCard
    title="Marmot for Agents"
    description="Plug your LLM agents into the catalog with ready-made tools and automatic lineage."
    href="/docs/Agents"
    icon="mdi:robot"
  />
  <DocCard
    title="REST API"
    description="The HTTP API the SDKs wrap. Use it directly when no SDK exists for your language."
    href="/api"
    icon="mdi:api"
  />
</DocCardGrid>

## Install

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```bash
pip install marmot-sdk
```

Requires Python 3.10+. The package name is `marmot-sdk`; the import name is `marmot`.

</TabPanel>

<TabPanel value="ts">

```bash
pnpm add @marmotdata/sdk
```

Requires Node 18+. ESM and CJS builds ship together with bundled types.

</TabPanel>

</Tabs>

## Authenticate

Credentials resolve in priority order:

1. **Explicit arguments.** `api_key` or `token` passed to `connect()`.
2. **Environment variables.** `MARMOT_API_KEY`, `MARMOT_TOKEN`, `MARMOT_HOST`, `MARMOT_CONTEXT`.
3. **Cached OAuth token.** Written to `~/.config/marmot/credentials.json` by `marmot login`.
4. **Workload identity.** Use GitHub Actions, GCP or Kubernetes credentials directly. No API key needed.

Log in for local use:

```bash
marmot login http://localhost:5173
```

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
import marmot

# Resolves from the chain
client = marmot.connect()

# Or pass an API key explicitly
client = marmot.connect(base_url="https://marmot.example.com", api_key="...")
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

// Resolves from the chain
const client = await connect();

// Or pass an API key explicitly
const client = await connect({
  baseUrl: "https://marmot.example.com",
  apiKey: "...",
});
```

</TabPanel>

</Tabs>

If no credential resolves, `connect()` raises `AuthError`.

## Search

```python
results = client.search("orders", types=["Table", "Topic"], limit=20)
for hit in results["results"]:
    print(hit["name"], hit["mrn"])
```

## Assets

```python
asset = client.assets.get("01HX...")
asset = client.assets.lookup(type="Table", service="postgres", name="orders")
asset = client.assets.find(type="Table", service="postgres", name="orders")  # None on miss

created = client.assets.create({
    "name": "orders",
    "type": "Table",
    "providers": ["postgres"],
    "services": ["sales-db"],
    "metadata": {"owner": "data-eng"},
})
client.assets.update(created["id"], {**created, "metadata": {"owner": "platform"}})
client.assets.delete(created["id"])
```

## Lineage

Edges identify endpoints by MRN (`mrn://<type>/<service>/<name>`).

```python
client.lineage.write(
    source="mrn://table/postgres/orders",
    target="mrn://topic/kafka/orders.events",
)

# Batched: one HTTP call for many edges
client.lineage.batch([
    ("mrn://table/postgres/orders", "mrn://topic/kafka/orders.events"),
    ("mrn://topic/kafka/orders.events", "mrn://bucket/s3/orders"),
])

graph = client.lineage.upstream(asset_id="01HX...", depth=3)
```

<CalloutCard
  title="Building an agent?"
  description="Marmot for Agents builds on this SDK to give LLM agents the catalog as tools and write their lineage automatically."
  href="/docs/Agents"
  buttonText="Read the guide"
  icon="mdi:robot"
/>
