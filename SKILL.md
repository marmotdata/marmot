---
name: marmot
description: Interact with a Marmot data catalog instance. Use this skill when the user wants to search for data assets, view lineage, browse glossary terms, check pipeline runs, manage tags or owners, view metrics, or do anything related to their data catalog. Covers the Marmot CLI, REST API and MCP server.
---

This skill helps you interact with a Marmot data catalog. Marmot catalogs data assets (databases, tables, topics, APIs, dashboards) across an organisation's data stack and tracks lineage between them.

## Setup

The user needs the Marmot CLI installed and configured. If not already set up:

```bash
curl -fsSL get.marmotdata.io | sh
marmot config init
```

`config init` prompts for the host URL and API key. Alternatively, set environment variables:

```bash
export MARMOT_HOST=https://marmot.example.com
export MARMOT_API_KEY=<key>
```

Or pass `--host` and `--api-key` as flags on any command.

## MCP Server

Marmot has a built-in MCP (Model Context Protocol) server. If the user has configured MCP, you can interact with the catalog directly using these tools instead of the CLI:

- **discover_data** — unified data discovery. Supports natural language queries, lookups by ID or MRN (e.g. `postgres://db/schema/table`), filtering by type/provider/tags and metadata-based queries.
- **find_ownership** — bidirectional ownership queries. "Who owns this asset?", "What does this user own?", "Show all data owned by the data-eng team". Works for assets and glossary terms.
- **lookup_term** — business glossary lookups. Search for terms by name or retrieve definitions.

MCP configuration lives in `~/.claude.json` (user-level) or `.mcp.json` (project-level):

```json
{
  "mcpServers": {
    "marmot": {
      "type": "http",
      "url": "https://<marmot-server>/api/v1/mcp",
      "headers": {
        "X-API-Key": "<api-key>"
      }
    }
  }
}
```

Prefer MCP tools when available. Fall back to the CLI or REST API for operations MCP doesn't cover (e.g. writes, metrics, admin).

## CLI Reference

All commands follow the pattern `marmot <resource> <action> [args] [flags]`. Use `--output json` (or `-o json`) to get machine-readable output. Supported output formats: `table` (default), `json`, `yaml`.

### Assets

```bash
marmot assets list                        # list assets (paginated)
marmot assets search <query>              # search assets
marmot assets get <id>                    # get asset details
marmot assets summary                     # counts by type, provider, tag
marmot assets tags add <id> <tag>         # add a tag
marmot assets tags remove <id> <tag>      # remove a tag
marmot assets owners <id>                 # list owners
marmot assets delete <id>                 # delete (prompts for confirmation)
```

Filters: `--types`, `--providers`, `--tags`. Pagination: `--limit`, `--offset`.

### Search

```bash
marmot search <query>                     # unified search (assets, glossary, teams, users)
```

Filter by type with `--types asset,glossary`.

### Glossary

```bash
marmot glossary list
marmot glossary get <id>
marmot glossary create --name "Term" --definition "What it means"
marmot glossary update <id> --definition "New definition"
marmot glossary delete <id>
marmot glossary search <query>
```

### Lineage

```bash
marmot lineage get <asset-id>             # view upstream/downstream graph
```

Use `--depth` to control traversal depth.

### Runs

```bash
marmot runs list                          # list pipeline runs
marmot runs get <id>                      # run details and summary
marmot runs entities <id>                 # entities processed in a run
```

Filter with `--pipelines` and `--statuses`.

### Users and API Keys

```bash
marmot users me                           # current authenticated user
marmot users list
marmot apikeys list
marmot apikeys create <name>              # key shown once at creation
marmot apikeys delete <id>
```

### Teams

```bash
marmot teams list
marmot teams get <id>
marmot teams members <id>
```

### Metrics

```bash
marmot metrics summary                    # total assets + by-type breakdown
marmot metrics by-type                    # asset counts by type
marmot metrics by-provider                # asset counts by provider
marmot metrics top-assets --start <RFC3339> --end <RFC3339>
marmot metrics top-queries --start <RFC3339> --end <RFC3339>
```

`top-assets` and `top-queries` require a time range. `--start` and `--end` accept RFC3339 timestamps (e.g. `2026-01-01T00:00:00Z`). If omitted, defaults to the last 30 days. Use `--limit` to control how many results are returned (default 10).

`summary`, `by-type` and `by-provider` do not require a time range.

### Admin

```bash
marmot admin reindex                      # trigger search reindex
marmot admin reindex-status               # check progress
```

### Config

```bash
marmot config init                        # interactive setup
marmot config set <key> <value>           # set a config value
marmot config get <key>                   # get a config value
marmot config list                        # list all config values
```

## REST API

The Marmot API is available at `{host}/api/v1/`. Authenticate with the `X-API-Key` header.

### Key Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/api/v1/assets/search?q=...` | GET | Search assets |
| `/api/v1/assets/{id}` | GET | Get asset by ID |
| `/api/v1/assets/{id}` | DELETE | Delete asset |
| `/api/v1/assets/summary` | GET | Asset summary stats |
| `/api/v1/lineage/assets/{id}` | GET | Asset lineage graph |
| `/api/v1/glossary/list` | GET | List glossary terms |
| `/api/v1/glossary/{id}` | GET | Get glossary term |
| `/api/v1/glossary/` | POST | Create glossary term |
| `/api/v1/glossary/{id}` | PUT | Update glossary term |
| `/api/v1/glossary/{id}` | DELETE | Delete glossary term |
| `/api/v1/runs` | GET | List pipeline runs |
| `/api/v1/runs/{id}` | GET | Get run details |
| `/api/v1/search?q=...` | GET | Unified search |
| `/api/v1/metrics` | GET | Aggregated metrics (requires `start` and `end` query params, RFC3339) |
| `/api/v1/metrics/assets/total` | GET | Total asset count |
| `/api/v1/metrics/assets/by-type` | GET | Assets by type |
| `/api/v1/metrics/assets/by-provider` | GET | Assets by provider |
| `/api/v1/metrics/top-queries` | GET | Top search queries (requires `start`, `end`) |
| `/api/v1/metrics/top-assets` | GET | Top viewed assets (requires `start`, `end`) |
| `/api/v1/users/me` | GET | Current user |
| `/api/v1/users/apikeys` | GET | List API keys |
| `/api/v1/users/apikeys` | POST | Create API key |
| `/api/v1/teams` | GET | List teams |
| `/api/v1/teams/{id}` | GET | Get team |
| `/api/v1/teams/{id}/members` | GET | Team members |
| `/api/v1/admin/search/reindex` | POST | Trigger reindex |
| `/api/v1/admin/search/reindex` | GET | Reindex status |

### Time-range endpoints

`/api/v1/metrics`, `/api/v1/metrics/top-queries` and `/api/v1/metrics/top-assets` require `start` and `end` query parameters in RFC3339 format. The maximum range is 30 days. Example:

```
GET /api/v1/metrics/top-queries?start=2026-03-01T00:00:00Z&end=2026-03-26T00:00:00Z&limit=10
```

## Tips

- Prefer MCP tools when the user has MCP configured. Fall back to the CLI for operations MCP doesn't cover.
- Prefer the CLI over raw API calls when the user has it installed.
- Use `-o json` and pipe through `jq` when the user needs to extract or transform data.
- Asset IDs are UUIDs. Asset MRNs (Marmot Resource Names) look like `mrn://type/provider/name`.
- Destructive commands (`delete`) prompt for confirmation. Pass `--yes` to skip in scripts.
- All list commands support `--limit` and `--offset` for pagination.
- When using the REST API directly, always include the `X-API-Key` header.
