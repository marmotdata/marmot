---
sidebar_position: 3
---

# Claude Agent SDK

The Claude Agent SDK integration ships in the Python and TypeScript SDKs. It has two halves:

import { CalloutCard } from '@site/src/components/DocCard';
import { Tabs, TabPanel } from '@site/src/components/Steps';

1. The **Marmot MCP server** (built into your Marmot instance at `/api/v1/mcp`) gives the agent catalog-aware tools: `discover_data`, `find_ownership`, `lookup_term`. Point the SDK at it via `mcpServers` and it just shows up as a tool source.
2. **`MarmotAgentTracker`** plugs into the SDK's hook system. The first time any hook fires it registers the agent as an asset of type `Agent`, captures every tool output for `mrn://` references, and writes one batched lineage call when the session ends.

## Install

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```bash
pip install "marmot-sdk[claude-agent]"
```

The `claude-agent` extra adds `claude-agent-sdk`.

</TabPanel>

<TabPanel value="ts">

```bash
pnpm add @marmotdata/sdk @anthropic-ai/claude-agent-sdk
```

`@anthropic-ai/claude-agent-sdk` is loaded only when you import the tracker, so the SDK stays lean for non-agent users.

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
import asyncio
from marmot import Client, resolve
from marmot.integrations.claude_agent import MarmotAgentTracker
from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions

async def main():
    base_url, credential = resolve(base_url=None)
    client = Client(base_url=base_url, credential=credential)
    tracker = MarmotAgentTracker(
        client,
        name="catalog-explorer",
        model="claude-sonnet-4-5",
        owner="data-eng",
    )

    options = ClaudeAgentOptions(
        mcp_servers={
            "marmot": {
                "type": "http",
                "url": f"{base_url}/api/v1/mcp",
                "headers": {"Authorization": f"Bearer {credential.token}"},
            }
        },
        hooks=tracker.hooks(),
        permission_mode="bypassPermissions",
    )

    async with ClaudeSDKClient(options=options) as agent:
        await agent.query("Find a postgres table about orders and summarise it.")
        async for _ in agent.receive_response():
            pass

    print("agent registered as:", tracker.agent_mrn)

asyncio.run(main())
```

</TabPanel>

<TabPanel value="ts">

```ts
import { Client, resolve } from "@marmotdata/sdk";
import { MarmotAgentTracker } from "@marmotdata/sdk/claude-agent";
import { query } from "@anthropic-ai/claude-agent-sdk";

const { baseUrl, credential } = await resolve({});
const client = new Client({ baseUrl, credential });

const tracker = new MarmotAgentTracker(client, {
  name: "catalog-explorer",
  model: "claude-sonnet-4-5",
  owner: "data-eng",
});

for await (const msg of query({
  prompt: "Find a postgres table about orders and summarise it.",
  options: {
    mcpServers: {
      marmot: {
        type: "http",
        url: `${baseUrl}/api/v1/mcp`,
        headers: { Authorization: `Bearer ${credential.token}` },
      },
    },
    hooks: tracker.hooks(),
    permissionMode: "bypassPermissions",
  },
})) {
  // stream messages as you wish
}

console.log("agent registered as:", tracker.agentMrn);
```

</TabPanel>

</Tabs>

After the first run the agent appears in Marmot as `type=Agent`, `service=ClaudeAgent`, `name=catalog-explorer`, with lineage edges from every asset it touched.

## Catalog tools (via MCP)

The Marmot MCP server exposes the catalog as tools, namespaced `mcp__marmot__*`:

| Tool             | Purpose                                                       |
| ---------------- | ------------------------------------------------------------- |
| `discover_data`  | Find or browse assets — by name, type, provider, tags, or MRN |
| `find_ownership` | Resolve who owns an asset, or what a team/user owns           |
| `lookup_term`    | Search the business glossary for term definitions             |

Tool responses include MRNs (often inside markdown content blocks). The tracker walks both structured objects and free text for `mrn://` URIs, so lineage is captured automatically.

To restrict the agent to a subset:

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
options = ClaudeAgentOptions(
    mcp_servers={"marmot": {...}},
    hooks=tracker.hooks(),
    allowed_tools=["mcp__marmot__discover_data", "mcp__marmot__find_ownership"],
)
```

</TabPanel>

<TabPanel value="ts">

```ts
options: {
  mcpServers: { marmot: { ... } },
  hooks: tracker.hooks(),
  allowedTools: ["mcp__marmot__discover_data", "mcp__marmot__find_ownership"],
}
```

</TabPanel>

</Tabs>

## Custom tools

Two ways to attribute lineage from tools you ship as your own MCP server alongside Marmot's.

### MRNs in tool output

If your tool returns objects with `mrn` fields — `{ mrn, ... }` or `{ results: [{ mrn, ... }] }` — or text bodies that mention `mrn://...` URIs, the tracker picks them up on every `PostToolUse` hook. This is the same mechanism that captures MRNs from Marmot's MCP responses.

### Manual `record_source`

Use this when the upstream is only known at runtime — for example a tool that picks one of several tables:

<Tabs items={[
{ label: "Python", value: "py", icon: "mdi:language-python" },
{ label: "TypeScript", value: "ts", icon: "mdi:language-typescript" }
]} groupId="lang">

<TabPanel value="py">

```python
def query_table(table: str, sql: str) -> list[dict]:
    tracker.record_source(f"mrn://table/postgres/{table}")
    return run_sql(sql)
```

</TabPanel>

<TabPanel value="ts">

```ts
function queryTable(table: string, sql: string) {
  tracker.recordSource(`mrn://table/postgres/${table}`);
  return runSql(sql);
}
```

</TabPanel>

</Tabs>
