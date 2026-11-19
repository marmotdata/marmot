---
title: Data queries
description: How Marmot runs read-only SQL against your sources for people and agents, what it checks first, what it keeps and which sources can answer.
---

# Data queries

Data queries let people and agents run read-only SQL against a catalogued source through Marmot. Marmot checks who is asking before anything runs, returns the rows without storing them and records every query in the [query audit](query-audit.md).

## How a query runs

1. A person in [Query](query-data.md), an agent over [MCP](MCP/index.md) or a script with the [CLI](cli.md) sends a statement and names a source.
2. Marmot checks the caller holds `assets:query`. A caller without it is refused before the statement reaches the source.
3. The source's plugin plans the statement and refuses anything that isn't a single read. PostgreSQL runs it inside a read-only transaction that's always rolled back; DuckDB opens its file read-only with external access turned off.
4. The plugin runs the statement with the source's own credentials and returns the rows, up to the row limit.
5. Marmot returns the rows alongside what the catalog knows about each table read: its owners, description and which columns hold personal data.

Marmot doesn't keep the rows. The query audit records the statement, who ran it, the tables it read, how many rows came back and the outcome.

The source's own permissions are still the hard boundary. Give the pipeline a database user that can only read what people should be able to query.

## Which sources can answer

A source answers queries when its plugin supports querying and its pipeline has **Let agents and people query this source** turned on under **Answer queries**. Plugins that support it are marked **Supports querying** when you add a source, such as PostgreSQL, MySQL, Google BigQuery, ClickHouse, Trino and DuckDB.

Turning querying on for a pipeline needs `access:manage`. Under the same section, **Refresh the catalog when queried** updates a table's details when a query reads it, at most hourly.

A source Marmot can't reach, such as a database on a private network, can still answer queries: run its plugin inside that network and it [serves queries](Configure/serving-plugins.md) over a connection it opens to Marmot.

## Permissions

| Permission | Lets you | Held by default by |
| --- | --- | --- |
| `assets:query` | Run queries against every queryable source | `admin` |
| `access:view` | Read everyone's queries in the query audit | `admin` |
| `access:manage` | Choose which pipelines answer queries, and cancel anyone's running query | `admin` |

Everyone can see and cancel their own queries. To let analysts query, create a role with `assets:query` and assign it; see [users and access](Configure/access-control.md).

When a query is refused, **Request access** sends the request to everyone who can manage users or service accounts.

## Limits

These are set under `query` in the [configuration](Configure/index.md#data-queries):

| Setting | Default | What it does |
| --- | --- | --- |
| `query_timeout` | 60 seconds | How long a query may run before it's cancelled |
| `max_rows_default` | 1,000 | Rows returned when the caller doesn't ask for a limit |
| `max_rows_cap` | 10,000 | The most rows any query can return |

A result cut off at the limit is marked as truncated. Set `query.enabled: false` to turn data queries off.

## Limitations

- Statements must be a single read. Writes, DDL and multiple statements are refused.
- A query reads one source. To join across sources, join in a warehouse source that already holds both.
- In open source Marmot, `assets:query` covers every queryable source. Per-table and per-source access is part of [Marmot Cloud](Cloud/access-control.md).

## Next steps

- [Query your data](query-data.md): run, save and export a query.
- [Query audit](query-audit.md): see who queried what.
- [Serve queries from your network](Configure/serving-plugins.md): answer queries from a source Marmot can't reach.
