---
sidebar_position: 1
---

# SDK

The Marmot SDK is a typed client for the REST API, available in **Python**, **Go** and **TypeScript**. Authentication resolves automatically from environment variables, cached OAuth tokens or workload identity.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

<DocCardGrid>
  <DocCard
    title="Marmot for Agents"
    description="Plug your LLM agents into the catalog with ready-made tools and automatic lineage."
    docId="Agents/index"
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
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```bash
pip install marmot-sdk
```

Requires Python 3.10+. Package name is `marmot-sdk`, import name is `marmot`.

</TabPanel>

<TabPanel value="go">

```bash
go get github.com/marmotdata/marmot/sdk/go
```

Requires Go 1.24+. Import path is `github.com/marmotdata/marmot/sdk/go`, package name is `marmot`.

</TabPanel>

<TabPanel value="ts">

```bash
pnpm add @marmotdata/sdk
```

Requires Node 18+. ESM and CJS builds ship together with bundled types.

</TabPanel>

</Tabs>

## Authenticate

Every SDK resolves credentials from the same priority chain, so the same code runs locally, in CI and in production without branching on environment:

1. **Explicit arguments.** `api_key` / `token` passed to `connect()` or `NewClient()`.
2. **Environment variables.** `MARMOT_API_KEY`, `MARMOT_TOKEN`, `MARMOT_HOST`, `MARMOT_CONTEXT`.
3. **Cached OAuth token.** Written to `~/.config/marmot/credentials.json` by `marmot login`.
4. **Workload identity.** GitHub Actions OIDC, GCP metadata or a Kubernetes service-account token. No API key needed.

If no credential resolves, the SDK raises an `AuthError` so misconfiguration fails fast.

Log in for local use:

```bash
marmot login http://localhost:5173
```

Then construct a client:

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, UsersApi

# Resolves host and credential from the chain
client = AuthenticatedApiClient.connect()

# Or supply them explicitly
client = AuthenticatedApiClient.connect(
    host="https://marmot.example.com", api_key="..."
)

me = UsersApi(client).get_users_me_sync()
print(me.name, "via", client.credential.source)
```

</TabPanel>

<TabPanel value="go">

```go
package main

import (
    "context"
    "log"
    "os"

    marmot "github.com/marmotdata/marmot/sdk/go"
)

func main() {
    ctx := context.Background()

    // Resolves from the chain
    client, err := marmot.NewClient(marmot.ClientOptions{})
    if err != nil {
        log.Fatal(err)
    }

    // Or pass credentials explicitly
    client, err = marmot.NewClient(marmot.ClientOptions{
        Host:   "https://marmot.example.com",
        APIKey: os.Getenv("MARMOT_API_KEY"),
    })
}
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

// Resolves from the chain
const client = await connect();

// Or pass an API key explicitly
const explicit = await connect({
  baseUrl: "https://marmot.example.com",
  apiKey: "...",
});
```

</TabPanel>

</Tabs>

The following sections all assume `client` (and `ctx` for Go) is already constructed as shown above.

<TipBox>

**Python:** one `AuthenticatedApiClient` is shared by every generated `*Api` class, which
you construct around it — `UsersApi(client)`, `AssetsApi(client)`, and so on. Method names
follow the operation: `get_assets_id`, `post_lineage_batch`. Each exists twice, as a
coroutine and with a `_sync` suffix that runs it on a shared event loop, so the snippets
below stay synchronous. Request bodies are pydantic models from `marmot.generated.models`,
and failures raise `marmot.errors` types (`NotFoundError`, `AuthError`, `ValidationError`,
`RateLimitError`, `ServerError`) rather than returning a status code.

</TipBox>

## Search

One unified search across assets, glossary terms, teams and data products. Returns a typed `SearchResponse` with facets, results and pagination.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, SearchApi

client = AuthenticatedApiClient.connect()

results = SearchApi(client).get_search_sync(q="orders", types=["Table", "Topic"], limit=20)
print(f"{results.total} matches")
for hit in results.results or []:
    print(hit.name, "-", hit.type.value if hit.type else "unknown")
```

</TabPanel>

<TabPanel value="go">

```go
import (
    "fmt"

    marmot "github.com/marmotdata/marmot/sdk/go"
)

results, err := client.Search.Query(ctx, "orders", marmot.SearchOptions{
    Types: []string{"Table", "Topic"},
    Limit: 20,
})
if err != nil {
    return err
}
for _, hit := range results.Results {
    fmt.Println(hit.Name, hit.Metadata["mrn"])
}
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const results = await client.search("orders", {
  types: ["Table", "Topic"],
  limit: 20,
});
for (const hit of results.results ?? []) {
  console.log(hit.name, hit.metadata?.mrn);
}
```

</TabPanel>

</Tabs>

Marmot accepts both free-text queries and a structured query language (`@type: "Table" AND @provider: "postgres"`). See the [Query Language guide](/docs/queries) for the full grammar.

## Assets

Every catalog entry is an Asset. The Assets resource covers CRUD, lookup by natural key, search, summary aggregates and tag management.

### Fetch by ID

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AssetsApi, AuthenticatedApiClient

client = AuthenticatedApiClient.connect()

asset = AssetsApi(client).get_assets_id_sync(id="01HX...")
print(asset.name, asset.mrn)
```

</TabPanel>

<TabPanel value="go">

```go
import "fmt"

asset, err := client.Assets.Get(ctx, "01HX...")
if err != nil {
    return err
}
fmt.Println(asset.Name, asset.Mrn)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const asset = await client.assets.get("01HX...");
console.log(asset.name, asset.mrn);
```

</TabPanel>

</Tabs>

### Lookup by natural key

When you know an asset by `(type, service, name)` but not its ID, `lookup` resolves it. `find` does the same but returns `nil` / `None` instead of raising on a miss.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AssetsApi, AuthenticatedApiClient
from marmot.errors import NotFoundError

assets = AssetsApi(AuthenticatedApiClient.connect())

asset = assets.get_assets_lookup_type_service_name_sync(
    type="Table", service="postgres", name="orders"
)

# A missing asset raises rather than returning None
try:
    assets.get_assets_lookup_type_service_name_sync(
        type="Table", service="postgres", name="nope"
    )
except NotFoundError:
    asset = None
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

asset, err := client.Assets.Lookup(ctx, marmot.LookupInput{
    Type:    "Table",
    Service: "postgres",
    Name:    "orders",
})
if err != nil {
    return err
}

// nil, nil on 404 instead of an error
maybe, err := client.Assets.Find(ctx, marmot.LookupInput{
    Type:    "Table",
    Service: "postgres",
    Name:    "orders",
})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const asset = await client.assets.lookup({
  type: "Table",
  service: "postgres",
  name: "orders",
});
// null on 404 instead of throwing
const maybe = await client.assets.find({
  type: "Table",
  service: "postgres",
  name: "orders",
});
```

</TabPanel>

</Tabs>

### Search and summary

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AssetsApi, AuthenticatedApiClient

assets = AssetsApi(AuthenticatedApiClient.connect())

hits = assets.get_assets_search_sync(
    q="customer",
    types=["Table"],
    services=["postgres"],
    tags=["pii"],
    limit=50,
)
summary = assets.get_assets_summary_sync()  # totals by type, provider, tag
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

hits, err := client.Assets.Search(ctx, marmot.AssetSearchOptions{
    Query:     "customer",
    Types:     []string{"Table"},
    Providers: []string{"postgres"},
    Tags:      []string{"pii"},
    Limit:     50,
})
summary, err := client.Assets.Summary(ctx)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const hits = await client.assets.search({
  query: "customer",
  types: ["Table"],
  providers: ["postgres"],
  tags: ["pii"],
  limit: 50,
});
const summary = await client.assets.summary();
```

</TabPanel>

</Tabs>

### Create, update, delete

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AssetsApi, AuthenticatedApiClient
from marmot.generated.models import CreateAssetRequest, UpdateAssetRequest

assets = AssetsApi(AuthenticatedApiClient.connect())

created = assets.post_assets_sync(
    create_asset_request=CreateAssetRequest(
        name="orders",
        type="Table",
        providers=["postgres"],
        metadata={"owner": "data-eng"},
    )
)

updated = assets.put_assets_id_sync(
    id=created.id,
    update_asset_request=UpdateAssetRequest(description="Customer orders"),
)
assets.delete_assets_id_sync(id=created.id)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

created, err := client.Assets.Create(ctx, marmot.CreateAssetInput{
    Name:      "orders",
    Type:      "Table",
    Providers: []string{"postgres"},
    Tags:      []string{"pii"},
})
if err != nil {
    return err
}

_, err = client.Assets.Update(ctx, *created.ID, marmot.UpdateAssetInput{
    Description: "Customer orders",
})
err = client.Assets.Delete(ctx, *created.ID)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const created = await client.assets.create({
  name: "orders",
  type: "Table",
  providers: ["postgres"],
  metadata: { owner: "data-eng" },
});

const updated = await client.assets.update(created.id!, {
  description: "Customer orders",
});
await client.assets.delete(created.id!);
```

</TabPanel>

</Tabs>

### Tag management

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AssetsApi, AuthenticatedApiClient
from marmot.generated.models import TagRequest

assets = AssetsApi(AuthenticatedApiClient.connect())

assets.post_assets_tags_id_sync(id=asset_id, tag_request=TagRequest(tag="pii"))
assets.delete_assets_tags_id_sync(id=asset_id, tag_request=TagRequest(tag="pii"))
```

</TabPanel>

<TabPanel value="go">

```go
err := client.Assets.AddTag(ctx, assetID, "pii")
if err != nil {
    return err
}
err = client.Assets.RemoveTag(ctx, assetID, "pii")
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

await client.assets.addTag(assetId, "pii");
await client.assets.removeTag(assetId, "pii");
```

</TabPanel>

</Tabs>

## Lineage

Lineage edges identify endpoints by MRN (`<type>://<service>/<name>`). Read the graph from any node; write one edge or many at a time.

### Read the graph

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, LineageApi

lineage = LineageApi(AuthenticatedApiClient.connect())

graph = lineage.get_lineage_assets_id_sync(id=asset_id, direction="both", limit=50)
upstream = lineage.get_lineage_assets_id_sync(id=asset_id, direction="upstream", limit=10)

# Leave out edge types you don't want, e.g. structural CONTAINS edges
flow = lineage.get_lineage_assets_id_sync(id=asset_id, exclude_types="CONTAINS")
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

graph, err := client.Lineage.Get(ctx, assetID, marmot.LineageOptions{
    Direction: "both",
    Limit:     50,
})
if err != nil {
    return err
}

upstream, err := client.Lineage.Upstream(ctx, assetID, marmot.LineageOptions{Limit: 10})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const graph = await client.lineage.get(assetId, {
  direction: "both",
  depth: 3,
});
const upstream = await client.lineage.upstream(assetId, { depth: 2 });
const downstream = await client.lineage.downstream(assetId, { depth: 2 });
```

</TabPanel>

</Tabs>

### Write edges

Prefer `/lineage/direct` and `/lineage/batch` for new integrations. They accept simple `(source, target)` pairs and de-duplicate server-side.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, LineageApi
from marmot.generated.models import LineageEdge

lineage = LineageApi(AuthenticatedApiClient.connect())

# Single edge
lineage.post_lineage_direct_sync(
    lineage_edge=LineageEdge(
        source="postgres://prod/sales/orders",
        target="kafka://prod/orders.events",
    )
)

# Batched: one HTTP call, many edges
lineage.post_lineage_batch_sync(
    lineage_edge=[
        LineageEdge(
            source="postgres://prod/sales/orders",
            target="kafka://prod/orders.events",
        ),
        LineageEdge(
            source="kafka://prod/orders.events",
            target="s3://prod/orders-archive",
        ),
    ]
)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

// Single edge
_, err := client.Lineage.Write(ctx, marmot.WriteEdgeInput{
    Source: "postgres://prod/sales/orders",
    Target: "kafka://prod/orders.events",
})
if err != nil {
    return err
}

// Batched: one HTTP call, many edges
_, err = client.Lineage.Batch(ctx, []marmot.WriteEdgeInput{
    {Source: "postgres://prod/sales/orders", Target: "kafka://prod/orders.events"},
    {Source: "kafka://prod/orders.events", Target: "s3://prod/orders-archive"},
})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

await client.lineage.write({
  source: "postgres://prod/sales/orders",
  target: "kafka://prod/orders.events",
});

await client.lineage.batch([
  ["postgres://prod/sales/orders", "kafka://prod/orders.events"],
  ["kafka://prod/orders.events", "s3://prod/orders-archive"],
]);
```

</TabPanel>

</Tabs>

Leave `Type` empty (`DIRECT` is the default) for code-derived edges; set it explicitly (`"writes"`, `"AGENT_LOOKUP"`, …) when you want to distinguish causes in the lineage graph.

## Glossary

Business glossary terms with definitions, descriptions and hierarchies via `parent_term_id`.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, GlossaryApi
from marmot.generated.models import CreateTermRequest, UpdateTermRequest

glossary = GlossaryApi(AuthenticatedApiClient.connect())

page = glossary.get_glossary_list_sync(limit=50)
print(f"{len(page.terms or [])} of {page.total} terms")

hits = glossary.get_glossary_search_sync(q="customer")

term = glossary.post_glossary_sync(
    create_term_request=CreateTermRequest(
        name="PII",
        definition="Personally Identifiable Information",
        description="Data that can identify an individual.",
    )
)

glossary.put_glossary_id_sync(
    id=term.id,
    update_term_request=UpdateTermRequest(name="Personally Identifiable Information"),
)
glossary.delete_glossary_id_sync(id=term.id)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

page, err := client.Glossary.List(ctx, marmot.GlossaryListOptions{Limit: 50})
if err != nil {
    return err
}
hits, err := client.Glossary.Search(ctx, marmot.GlossarySearchOptions{Query: "customer"})

term, err := client.Glossary.Create(ctx, marmot.CreateTermInput{
    Name:        "PII",
    Definition:  "Personally Identifiable Information",
    Description: "Data that can identify an individual.",
})

_, err = client.Glossary.Update(ctx, *term.ID, marmot.UpdateTermInput{
    Name: "Personally Identifiable Information",
})
err = client.Glossary.Delete(ctx, *term.ID)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const page = await client.glossary.list({ limit: 50 });
const hits = await client.glossary.search({ query: "customer" });

const term = await client.glossary.create({
  name: "PII",
  definition: "Personally Identifiable Information",
  description: "Data that can identify an individual.",
});

await client.glossary.update(term.id!, {
  name: "Personally Identifiable Information",
});
await client.glossary.delete(term.id!);
```

</TabPanel>

</Tabs>

## Users & Teams

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, TeamsApi, UsersApi

client = AuthenticatedApiClient.connect()
users, teams = UsersApi(client), TeamsApi(client)

me = users.get_users_me_sync()
user = users.get_users_id_sync(id=user_id)
page = users.get_users_sync(active=True, limit=100)

all_teams = teams.get_teams_sync()
team = teams.get_teams_id_sync(id=team_id)
members = teams.get_teams_id_members_sync(id=team_id)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

me, err := client.Users.Me(ctx)
if err != nil {
    return err
}
user, err := client.Users.Get(ctx, userID)
active := true
users, err := client.Users.List(ctx, marmot.UsersListOptions{Active: &active, Limit: 100})

teams, err := client.Teams.List(ctx, marmot.TeamsListOptions{})
team, err := client.Teams.Get(ctx, teamID)
members, err := client.Teams.Members(ctx, teamID)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const me = await client.users.me();
const user = await client.users.get(userId);
const users = await client.users.list({ active: true, limit: 100 });

const teams = await client.teams.list();
const team = await client.teams.get(teamId);
const members = await client.teams.members(teamId);
```

</TabPanel>

</Tabs>

## API Keys

Manage personal API keys for the authenticated user. The full key token is only readable from the `create` response, so store it immediately.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, UsersApi
from marmot.generated.models import CreateAPIKeyRequest

users = UsersApi(AuthenticatedApiClient.connect())

keys = users.get_users_apikeys_sync()
created = users.post_users_apikeys_sync(
    create_api_key_request=CreateAPIKeyRequest(name="ci-deploy", expires_in_days=30)
)
print(created.key)  # only readable here
users.delete_users_apikeys_id_sync(id=created.id)
```

</TabPanel>

<TabPanel value="go">

```go
import (
    "fmt"

    marmot "github.com/marmotdata/marmot/sdk/go"
)

keys, err := client.APIKeys.List(ctx)
if err != nil {
    return err
}
created, err := client.APIKeys.Create(ctx, marmot.CreateAPIKeyInput{
    Name:          "ci-deploy",
    ExpiresInDays: 30,
})
fmt.Println(created.Key) // only readable here
err = client.APIKeys.Delete(ctx, *created.ID)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const keys = await client.apiKeys.list();
const created = await client.apiKeys.create({
  name: "ci-deploy",
  expiresInDays: 30,
});
console.log(created.key); // only readable here
await client.apiKeys.delete(created.id!);
```

</TabPanel>

</Tabs>

## Runs

Read pipeline-ingestion run history. Useful when wiring up alerts on failed ingests or audit dashboards.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, RunsApi

runs = RunsApi(AuthenticatedApiClient.connect())

recent = runs.get_runs_sync(statuses="failed,running", limit=20)
run = runs.get_runs_id_sync(id=run_id)
entities = runs.get_runs_id_entities_sync(id=run_id, status="failed")
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

recent, err := client.Runs.List(ctx, marmot.RunsListOptions{
    Statuses: "failed,running",
    Limit:    20,
})
if err != nil {
    return err
}
run, err := client.Runs.Get(ctx, runID)
entities, err := client.Runs.Entities(ctx, runID, marmot.RunEntitiesOptions{
    Status: "failed",
})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const recent = await client.runs.list({
  statuses: "failed,running",
  limit: 20,
});
const run = await client.runs.get(runId);
const entities = await client.runs.entities(runId, { status: "failed" });
```

</TabPanel>

</Tabs>

## Metrics

Catalog usage and breakdown metrics. `top_assets` and `top_queries` take an inclusive `[start, end]` window of RFC3339 timestamps.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, MetricsApi

metrics = MetricsApi(AuthenticatedApiClient.connect())

total = metrics.get_metrics_assets_total_sync()
print(total.count)

by_type = metrics.get_metrics_assets_by_type_sync()
by_provider = metrics.get_metrics_assets_by_provider_sync()

top = metrics.get_metrics_top_assets_sync(
    start="2025-01-01T00:00:00Z",
    end="2025-02-01T00:00:00Z",
    limit=10,
)
queries = metrics.get_metrics_top_queries_sync(
    start="2025-01-01T00:00:00Z",
    end="2025-02-01T00:00:00Z",
    limit=10,
)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

total, err := client.Metrics.TotalAssets(ctx)
if err != nil {
    return err
}
byType, err := client.Metrics.AssetsByType(ctx)
byProvider, err := client.Metrics.AssetsByProvider(ctx)

top, err := client.Metrics.TopAssets(ctx, marmot.TopOptions{
    Start: "2025-01-01T00:00:00Z",
    End:   "2025-02-01T00:00:00Z",
    Limit: 10,
})
queries, err := client.Metrics.TopQueries(ctx, marmot.TopOptions{
    Start: "2025-01-01T00:00:00Z",
    End:   "2025-02-01T00:00:00Z",
    Limit: 10,
})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const total = await client.metrics.totalAssets();
const byType = await client.metrics.assetsByType();
const byProvider = await client.metrics.assetsByProvider();

const top = await client.metrics.topAssets({
  start: "2025-01-01T00:00:00Z",
  end: "2025-02-01T00:00:00Z",
  limit: 10,
});
const queries = await client.metrics.topQueries({
  start: "2025-01-01T00:00:00Z",
  end: "2025-02-01T00:00:00Z",
  limit: 10,
});
```

</TabPanel>

</Tabs>

## Owners

Search the catalog for asset owners (users and teams).

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AuthenticatedApiClient, OwnersApi

owners = OwnersApi(AuthenticatedApiClient.connect())

hits = owners.get_owners_search_sync(q="alice", limit=10)
for owner in hits.owners or []:
    print(owner)
```

</TabPanel>

<TabPanel value="go">

```go
import marmot "github.com/marmotdata/marmot/sdk/go"

hits, err := client.Owners.Search(ctx, "alice", marmot.OwnerSearchOptions{Limit: 10})
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const hits = await client.owners.search("alice", { limit: 10 });
```

</TabPanel>

</Tabs>

## Admin

Trigger or poll a full search reindex. Requires admin permissions.

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "Go", value: "go", icon: "mdi:language-go" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot import AdminApi, AuthenticatedApiClient

admin = AdminApi(AuthenticatedApiClient.connect())

accepted = admin.post_admin_search_reindex_sync()
status = admin.get_admin_search_reindex_sync()
print(status.running, status.es_configured)
```

</TabPanel>

<TabPanel value="go">

```go
import "fmt"

_, err := client.Admin.Reindex(ctx)
if err != nil {
    return err
}
status, err := client.Admin.ReindexStatus(ctx)
fmt.Println(status.Running, status.EsConfigured)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";

const client = await connect();

const accepted = await client.admin.reindex();
const status = await client.admin.reindexStatus();
console.log(status.running, status.es_configured);
```

</TabPanel>

</Tabs>

<CalloutCard
  title="Building an agent?"
  description="Marmot for Agents builds on the SDK to give LLM agents the catalog as tools and writes their lineage automatically."
  docId="Agents/index"
  buttonText="Read the guide"
  icon="mdi:robot"
/>

<DocCardGrid>
  <DocCard
    title="REST API"
    description="Every endpoint the SDKs wrap, with request/response schemas."
    href="/api"
    icon="mdi:api"
  />
  <DocCard
    title="Authentication"
    description="The full credential resolution chain, including OIDC workload identity."
    docId="Configure/Authentication/index"
    icon="mdi:lock"
  />
</DocCardGrid>
