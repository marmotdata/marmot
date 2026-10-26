---
name: marmot
description: Interact with a Marmot data catalog instance. Use this skill when the user wants to search for data assets, view lineage, browse glossary terms, check pipeline runs, manage tags or owners, view metrics, query catalogued data, or do anything related to their data catalog. Covers the Marmot CLI, REST API and MCP server.
---

This skill helps you interact with a Marmot data catalog. Marmot catalogs data assets (databases, tables, topics, APIs, dashboards) across an organisation's data stack and tracks lineage between them.

## Setup

The user needs the Marmot CLI installed and authenticated. If not already set up:

```bash
curl -fsSL get.marmotdata.io | sh
marmot login https://marmot.example.com
```

`marmot login` opens a browser for OAuth 2.0 PKCE authentication and caches the token locally. The token expires after 24 hours and carries the user's roles and permissions as claims. Login also registers the token with the Docker credential store for the instance's host, so `oras push`, `crane push` and `docker push` can publish plugins to the registry the instance serves. This goes through the `docker-credential-marmot` helper, a symlink to the marmot binary that `install.sh` creates; if the helper is not on your PATH, login writes the token itself into `~/.docker/config.json` the way `docker login` does. Alternatively, use an API key:

```bash
export MARMOT_HOST=https://marmot.example.com
export MARMOT_API_KEY=<key>
```

Or pass `--host` and `--api-key` as flags on any command.

`marmot skills install` refreshes this skill to match the installed CLI.

## MCP Server

Marmot has a built-in MCP (Model Context Protocol) server. If the user has configured MCP, you can interact with the catalog directly using these tools instead of the CLI:

Reading the catalog:

- **discover_data**: unified data discovery. Supports natural language queries, lookups by ID or MRN (e.g. `postgres://db/schema/table`), filtering by type, provider and tags, and metadata-based queries.
- **find_ownership**: ownership in both directions. "Who owns this asset?", "What does this user own?", "Show all data owned by the data-eng team". Works for assets and glossary terms.
- **lookup_term**: business glossary lookups. Search for terms by name or retrieve definitions.
- **explore_data_products**: curated collections of assets that serve a business purpose, and what they contain.
- **explore_teams**: teams and their members.
- **trace_lineage**: what feeds an asset (upstream) and what it feeds (downstream).

Changing the catalog (only offered to callers allowed to make the change):

- **update_documentation**: write or clear the human-authored description on an asset.
- **manage_tags**: add or remove tags on an asset.
- **manage_owners**: add or remove a user or team as an owner of an asset.

Querying data (only offered to callers with `assets:query`):

- **list_queryable_sources**: the sources that answer queries, searchable by name.
- **query_data**: run one read-only SQL statement against a source. The answer carries the rows plus what the catalog knows about every table read, and the id the query was recorded under.

Ask before calling the tools that change the catalog, even when the user's request implies the change. Say what you are about to write and to which asset.

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

Prefer MCP tools when available. Fall back to the CLI or REST API for operations MCP doesn't cover (e.g. glossary writes, metrics, admin).

## Driving the CLI as an agent

```bash
marmot agent schema                       # every command, flag, example, exit code and setting as JSON
export MARMOT_READ_ONLY=true              # refuse every command that is not a read (or pass --read-only)
```

Load `marmot agent schema` once rather than walking `--help`. Each command carries `access`: `read`, `write` (changes the server) or `local` (changes local config or credentials). Unless the user wants you to change things, work with `MARMOT_READ_ONLY=true`; a blocked command exits with code 9 and changes nothing.

Branch on the exit code, not the message:

| Code | Meaning | What to do |
|---|---|---|
| 0 | Success | |
| 1 | Other failure | Read the message |
| 2 | Usage: wrong arguments or flags | Fix the command line |
| 3 | Not authenticated | Ask the user to run `marmot login` or set `MARMOT_API_KEY` |
| 4 | Forbidden, including a query you may not run | Don't retry or work round it; tell the user what access is missing |
| 5 | Not found | Check the id or name |
| 6 | Rejected as invalid, including SQL the source cannot run | Fix the request |
| 7 | Rate limited | Wait, then retry |
| 8 | Server failed or unreachable | Retry later |
| 9 | Blocked by read-only mode | Ask the user before turning it off |
| 10 | Needs confirmation and there is no terminal | Show the user the change; pass `--yes` only once they agree |
| 11 | Confirmation declined | Stop |

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

### Querying data

`marmot query` runs read-only SQL against catalogued sources as the caller, checks the caller may query every table the statement reads and records the query. It needs the `assets:query` permission.

```bash
marmot query sources [--search <text>]              # sources you can query
marmot query tables <source> [--search <text>]      # catalogued tables on a source, busiest first
marmot query run <source> --table <name>            # preview a table in the source's own dialect
marmot query run <source> "SELECT ..."              # run a statement
marmot query run <source> --file q.sql              # ... from a file
marmot query run <source> - < q.sql                 # ... from stdin
marmot query history [--decision denied] [--source <name>] [--mrn <mrn>]
marmot query show <query-id>                        # one recorded query, with the reason if denied
marmot query stats [--since 7d]                     # totals, busiest tables and callers
```

- `query run` prints rows on stdout and the row count, query id and catalog context on stderr, so the table pipes cleanly. `-o json` returns everything in one document, including `truncated` when the row limit was hit (raise it with `--max-rows`).
- A denial gives the reason. Don't rewrite the query to get round it; tell the user what access is missing.
- `--since` takes `30m`, `24h`, `7d`, `2w` or an RFC 3339 time.

### Authentication

```bash
marmot login                              # authenticate via browser (OAuth PKCE); reuses a valid cached token
marmot login https://marmot.example.com   # authenticate to a specific instance
marmot login <url> --force                # sign in again even if a token is cached
marmot login <url> --print-token          # print the access token on stdout (status goes to stderr)
marmot login <url> --no-launch-browser    # print the sign-in URL; paste the callback URL back to finish
marmot logout                             # remove cached token and registry credential
marmot context list                       # list contexts (* = active)
marmot context use <name>                 # switch active context
marmot context delete <name>              # remove context and token
```

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
marmot config list                        # list all config values (incl. active context)
```

## REST API

The Marmot API is available at `{host}/api/v1/`. Authenticate with either the `X-API-Key` header or `Authorization: Bearer <token>` (from `marmot login`).

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
| `/api/v1/query/sources` | GET | Sources you can query |
| `/api/v1/query/sources/{name}/tables` | GET | Catalogued tables on a source |
| `/api/v1/query/run` | POST | Run a read-only statement (`source`, `statement`, `max_rows`) |
| `/api/v1/query/history` | GET | Query log (`mine=true` for your own) |
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
- Destructive commands (`assets delete`, `apikeys delete`) ask for confirmation, and refuse when there is no terminal to ask on. Pass `--yes` only once the user has agreed to that specific change.
- To answer a question from the data itself, find the table first (`discover_data` or `marmot query tables`), check its description and tags, then query it. Quote the query id with the answer so the user can find it in the log.
- All list commands support `--limit` and `--offset` for pagination.
- When using the REST API directly, authenticate with either `X-API-Key` or `Authorization: Bearer <token>` header.
