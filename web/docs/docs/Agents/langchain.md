---
sidebar_position: 2
---

# LangChain

The LangChain integration ships in the Python and TypeScript SDKs. It has two halves:

import { CalloutCard } from '@site/src/components/DocCard';
import { Tabs, TabPanel } from '@site/src/components/Steps';

1. **`catalog_tools(client)`** returns a list of LangChain tools (`search_catalog`, `get_asset`, `lookup_asset`, `get_upstream_lineage`) bound to your Marmot client. Drop them into any agent.
2. **`MarmotCallbackHandler`** registers the agent on first run as an asset of type `Agent`, captures every tool call and writes one batched lineage edge per upstream when the run ends.

## Install

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```bash
pip install "marmot-sdk[langchain]"
```

The `langchain` extra adds `langchain-core`. The agent runtime and model providers are up to you.

</TabPanel>

<TabPanel value="ts">

```bash
pnpm add @marmotdata/sdk @langchain/core
```

`@langchain/core` is an optional peer dependency, so the SDK stays lean for non-agent users.

</TabPanel>

</Tabs>

## Quick start

A minimal agent that searches the catalog, registers itself and writes lineage:

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from langchain_core.messages import HumanMessage, SystemMessage
from langchain_core.runnables import RunnableConfig, RunnableLambda
from langchain_openai import ChatOpenAI

from marmot import AuthenticatedApiClient
from marmot.auth import resolve_credential, resolve_host
from marmot.integrations import MarmotCatalog
from marmot.integrations.langchain import MarmotCallbackHandler, catalog_tools

AGENT_NAME = "catalog-explorer"
MODEL = "gpt-4o-mini"

SYSTEM_PROMPT = (
    "You answer questions about an organisation's data using the Marmot catalog. "
    "Always consult the marmot tools; never guess from memory."
)

catalog = MarmotCatalog(AuthenticatedApiClient.connect())

tools = catalog_tools(catalog)
handler = MarmotCallbackHandler(
    catalog,
    name=AGENT_NAME,
    model=MODEL,
    owner="data-eng",
    tools=tools,
)

llm = ChatOpenAI(model=MODEL, temperature=0).bind_tools(tools)


# The handler tracks a *chain* run: it registers the agent when the root chain
# starts and flushes on its end, attributing every nested tool and model call to
# it. `config` must be forwarded so those nested runs inherit the root.
def pipeline(question: str, config: RunnableConfig) -> str:
    answer = llm.invoke(
        [SystemMessage(SYSTEM_PROMPT), HumanMessage(question)], config=config
    )
    return str(answer.content)


reply = RunnableLambda(pipeline).invoke(
    "Find a postgres table about orders and summarise it.",
    config=RunnableConfig(callbacks=[handler]),
)

print(reply)
print("agent registered as:", handler.agent_mrn)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { connect } from "@marmotdata/sdk";
import { MarmotCallbackHandler, catalogTools } from "@marmotdata/sdk/langchain";
import { ChatOpenAI } from "@langchain/openai";
import { createReactAgent } from "langchain";

const client = await connect();
const tools = catalogTools(client);

const handler = new MarmotCallbackHandler(client, {
  name: "catalog-explorer",
  model: "gpt-4o-mini",
  owner: "data-eng",
  tools,
});

const agent = createReactAgent({
  llm: new ChatOpenAI({ model: "gpt-4o-mini", temperature: 0 }),
  tools,
});

await agent.invoke(
  { messages: [{ role: "user", content: "Find a postgres table about orders." }] },
  { callbacks: [handler] },
);

console.log("agent registered as:", handler.agentMrn);
```

</TabPanel>

</Tabs>

After the first run the agent appears in Marmot as `type=Agent`, `service=LangChain`, `name=catalog-explorer`, with lineage edges from every asset it touched.

## Catalog tools

`catalog_tools(client)` returns four tools wrapped around the SDK:

| Tool | Purpose |
| --- | --- |
| `search_catalog` | Find assets by name, description or metadata |
| `get_asset` | Fetch full schema and metadata for one asset ID |
| `lookup_asset` | Resolve an asset by `(type, service, name)` |
| `get_upstream_lineage` | Trace ancestors up to N hops |

Their responses include `mrn` fields, so the callback handler picks them up automatically and records the upstreams.

## Custom tools

Three ways to attribute lineage from your own tools.

### `marmot_tool` helper

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
from marmot.integrations.langchain import marmot_tool

@marmot_tool(asset_mrn="mrn://table/postgres/orders")
def query_orders(sql: str) -> list[dict]:
    """Run a read-only SQL query against the orders table."""
    return run_sql(sql)
```

</TabPanel>

<TabPanel value="ts">

```ts
import { marmotTool } from "@marmotdata/sdk/langchain";

const queryOrders = marmotTool({
  name: "query_orders",
  description: "Run a SQL query against the orders table.",
  assetMrn: "mrn://table/postgres/orders",
  schema: {
    type: "object",
    properties: { sql: { type: "string" } },
    required: ["sql"],
  },
  func: async ({ sql }: { sql: string }) => runSql(sql),
});
```

</TabPanel>

</Tabs>

The MRN is stamped into tool metadata. The handler reads it on every call.

### Manual `record_source`

Use this when the upstream is only known at runtime, for example a tool that picks one of several tables:

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
def query_table(table: str, sql: str) -> list[dict]:
    handler.record_source(f"mrn://table/postgres/{table}")
    return run_sql(sql)
```

</TabPanel>

<TabPanel value="ts">

```ts
function queryTable(table: string, sql: string) {
  handler.recordSource(`mrn://table/postgres/${table}`);
  return runSql(sql);
}
```

</TabPanel>

</Tabs>

### MRNs in tool output

If your tool returns objects shaped like `{ mrn, ... }` or `{ results: [{ mrn, ... }] }`, the handler walks the output looking for them. This is how `catalog_tools` produces lineage automatically.

<CalloutCard
  title="Other frameworks"
  description="LlamaIndex, AutoGen and CrewAI work today against the Marmot SDK. First-class integrations follow demand."
  docId="Agents/index"
  buttonText="See all integrations"
  icon="mdi:robot"
/>
