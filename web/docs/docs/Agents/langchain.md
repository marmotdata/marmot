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
import marmot
from marmot.integrations.langchain import MarmotCallbackHandler, catalog_tools
from langchain.agents import AgentExecutor, create_tool_calling_agent
from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI

with marmot.connect() as client:
    tools = catalog_tools(client)

    prompt = ChatPromptTemplate.from_messages([
        ("system", "You are a data analyst with access to the Marmot catalog."),
        ("human", "{input}"),
        ("placeholder", "{agent_scratchpad}"),
    ])
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
    agent = create_tool_calling_agent(llm, tools, prompt)
    executor = AgentExecutor(agent=agent, tools=tools)

    handler = MarmotCallbackHandler(
        client,
        name="catalog-explorer",
        model="gpt-4o-mini",
        owner="data-eng",
        tools=tools,
    )

    executor.invoke(
        {"input": "Find a postgres table about orders and summarise it."},
        config={"callbacks": [handler]},
    )

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

## What ends up in Marmot

The agent asset:

```json
{
  "name": "catalog-explorer",
  "type": "Agent",
  "providers": ["LangChain"],
  "metadata": {
    "framework": "LangChain",
    "model": "gpt-4o-mini",
    "owner": "data-eng",
    "tool_names": ["search_catalog", "get_asset", "lookup_asset", "get_upstream_lineage"],
    "system_prompt_sha256_16": "a1b2c3d4e5f60718"
  }
}
```

Each run also writes one batched lineage call (one edge per upstream MRN to the agent) and one run record (model, tokens, tool calls, status) visible on the agent's Runs tab.

<CalloutCard
  title="Other frameworks"
  description="LlamaIndex, AutoGen and CrewAI work today against the Marmot SDK. First-class integrations follow demand."
  href="/docs/Agents"
  buttonText="See all integrations"
  icon="mdi:robot"
/>
