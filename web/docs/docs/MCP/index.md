---
sidebar_position: 3
title: MCP server
description: Connect Claude Code, Claude Desktop, Cursor, Cline or LibreChat to Marmot's built-in MCP server so an assistant can search the catalog, trace lineage and look up owners and glossary terms.
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# MCP server

Marmot has a built-in [Model Context Protocol](https://modelcontextprotocol.io) server at `https://<your-marmot-server>/api/v1/mcp`. Point an AI assistant at it and the assistant can search your catalog, trace lineage and look up owners and glossary terms, with the same permissions as the identity it signs in with.

## Before you start

- You have a Marmot API key. Create one under **Account → API keys**, or use a [service account's key](#use-a-service-account) for an agent that runs on its own.
- You know your Marmot server's address. On [Marmot Cloud](../Cloud/instances.md) it's `https://<instance>.marmotdata.cloud`.

## Connect a client

Add Marmot to your client's MCP configuration:

<Tabs groupId="mcp-client" queryString="client">
<TabItem value="claude-code" label="Claude Code" default>

Add this to `~/.claude.json` for every project, or to `.mcp.json` in a project's root for that project only:

```json
{
  "mcpServers": {
    "marmot": {
      "type": "http",
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<api-key>"
      }
    }
  }
}
```

If you've signed in with `marmot login`, you can send `"Authorization": "Bearer <token>"` instead of `X-API-Key`. Claude Code asks you to approve a project's servers the first time it uses them.

</TabItem>
<TabItem value="claude-desktop" label="Claude Desktop">

Edit the configuration file for your platform:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "marmot": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "https://<your-marmot-server>/api/v1/mcp",
        "--header",
        "X-API-Key:<api-key>"
      ]
    }
  }
}
```

For a server on plain HTTP, such as a local one, add `--allow-http` to `args`.

</TabItem>
<TabItem value="cursor" label="Cursor">

Add this to `~/.cursor/mcp.json` for every project, or to `.cursor/mcp.json` in a project's root for that project only:

```json
{
  "mcpServers": {
    "marmot": {
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<api-key>"
      }
    }
  }
}
```

To keep the key out of the file, use `${env:MARMOT_API_KEY}` in its place.

</TabItem>
<TabItem value="cline" label="Cline">

In VS Code, open Cline, select the **MCP Servers** icon, then **Configure MCP Servers**, and add Marmot to `cline_mcp_settings.json`:

```json
{
  "mcpServers": {
    "marmot": {
      "url": "https://<your-marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<api-key>"
      }
    }
  }
}
```

</TabItem>
<TabItem value="librechat" label="LibreChat">

Add Marmot to `librechat.yaml`:

```yaml
mcpServers:
  marmot:
    type: streamable-http
    url: https://<your-marmot-server>/api/v1/mcp
    headers:
      X-API-Key: <api-key>
    timeout: 30000
```

To keep the key out of the file, use `${MARMOT_API_KEY}` in its place.

</TabItem>
</Tabs>

Then ask the assistant something only your catalog knows, such as "who owns the orders table?". It should answer with a real owner and the asset it found.

## Use a service account

An assistant that signs in with your API key has your permissions. An agent that runs on its own should have its own identity instead, so you can limit what it reaches and see what it did:

1. Under **Settings → Service accounts**, select **Create service account**.
2. Give it a role with `assets:view`, `glossary:view` and `teams:view`.
3. Create an API key for it and put that key in the client configuration.

## Hosted clients

Clients can also sign in with OAuth instead of an API key. The client registers itself with Marmot and completes a browser sign-in, so there's nothing to set up for clients running on your own machine.

A hosted client such as claude.ai calls back to an `https` address, so add its host to the server configuration:

```yaml
auth:
  dcr:
    allowed_redirect_hosts:
      - claude.ai
```

The environment variable `MARMOT_AUTH_DCR_ALLOWED_REDIRECT_HOSTS` takes the same list, separated by commas. Hosts on this list receive sign-in codes for your Marmot server, so only add hosts you trust.

Each person sees the clients they've allowed under **Account → Connected apps**. **Revoke** signs one out straight away.

## Tools

The server gives an assistant these tools:

| Tool | What it answers |
| --- | --- |
| `discover_data` | Finds assets by name, type, provider, tags or metadata |
| `find_ownership` | Who owns an asset, and everything a person or team owns |
| `lookup_term` | Glossary definitions and the assets linked to a term |
| `explore_data_products` | Which data products exist and the assets in each |
| `explore_teams` | Teams and their members |
| `trace_lineage` | What feeds an asset and what depends on it |
| `update_documentation` | Writes a plain-language description on an asset, without touching the one a plugin ingested |
| `manage_tags`, `manage_owners` | Adds or removes tags and owners on an asset |
| `list_queryable_sources`, `query_data` | Runs a read-only SQL statement against a source whose pipeline is set up to answer queries; see [data queries](../data-queries.md) |

The tools that change the catalog need `assets:manage`. An identity without it can still use every read tool.

## Next steps

- [Skills](skills.md): teach an agent the Marmot CLI and API as well.
- [Marmot for agents](../Agents/index.md): build your own agent on the catalog with LangChain or the Claude Agent SDK.
