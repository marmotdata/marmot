---
sidebar_position: 3
title: CLI reference
---

# CLI reference

The Marmot CLI works with your catalog from the terminal. It also runs plugins on your own machine and pushes what they find to Marmot, which [populating with the CLI](Populating/CLI.md) covers.

import { CliInstall } from '@site/src/components/CliInstall';

## Installation

<CliInstall />

---

## Authentication

Sign in once with `marmot login`. It opens your browser, you sign in the way you always do and the CLI keeps a token for you.

```bash
marmot login https://marmot.example.com
```

Every other command uses the token from then on. It lasts 24 hours; run `marmot login` again when it has expired.

### Useful options

```bash
# Skip the browser and print the sign-in link instead.
# Open it anywhere, then paste the address the browser ends up on back here.
marmot login https://marmot.example.com --no-launch-browser

# Sign in again even if you still have a valid token
marmot login https://marmot.example.com --force

# Print the token, for scripts
marmot login https://marmot.example.com --print-token

# Forget the token
marmot logout
```

### What happens behind the scenes

The CLI opens the Marmot sign-in page in your browser and waits on a local port for the browser to come back with a code. It swaps that code for a token and saves it in `~/.config/marmot/credentials.json`. The token is a signed JWT with your user id, your roles, their permissions and an expiry. This is plain OAuth 2.0 with PKCE. There is no client secret and nothing to configure.

### Pushing to a registry on your Marmot host

Login also makes the token available to `docker`, `crane` and `oras` for the Marmot host, so those tools can push to a registry served there without a `docker login`.

This needs `docker-credential-marmot` on your `PATH`. It is just the marmot binary under another name, and the install script sets it up. If you installed marmot some other way:

```bash
ln -s "$(command -v marmot)" "$(dirname "$(command -v marmot)")/docker-credential-marmot"
```

Without it, login writes the token straight into `~/.docker/config.json`. That works until the token expires, and not at all if Docker Desktop manages your credentials. Login tells you when that is the case.

### API keys

API keys can still be used and always take priority over cached login tokens.

```bash
# Via flag
marmot assets list --api-key mrmot_abc123

# Via environment variable
export MARMOT_API_KEY=mrmot_abc123
```

### Which credential is used

When multiple credentials are available, the CLI uses the first one found:

1. `--api-key` flag or `MARMOT_API_KEY` environment variable
2. Cached OAuth token from `marmot login` (for the active context)
3. Kubernetes service account token (auto-detected in-cluster)

A Kubernetes service account token is only accepted from the operator's ingestion ServiceAccount, projected for the `marmot` audience at `/var/run/secrets/marmot/token`, and it can only push ingestion results and serve queries. That covers starting and completing runs, writing assets, lineage and documentation and enrolling a serving plugin.

---

## Contexts

Contexts let you work with multiple Marmot instances (e.g. staging and production). A context is created automatically when you run `marmot login`.

```bash
# Login creates a context named after the hostname
marmot login https://marmot.example.com
# → Context "marmot.example.com" created and activated.

marmot login https://staging.marmot.dev
# → Context "staging.marmot.dev" created and activated.

# List all contexts (* = active)
marmot context list
#   marmot.example.com   https://marmot.example.com   (token valid)
# * staging.marmot.dev   https://staging.marmot.dev    (token valid)

# Switch active context
marmot context use marmot.example.com

# Remove a context and its cached token
marmot context delete staging.marmot.dev
```

---

## Configuration

You can also configure the CLI with flags, environment variables or a config file. These are checked in order of precedence.

### Flags

```bash
marmot assets list --host https://marmot.example.com --api-key my-key
```

### Environment variables

```bash
export MARMOT_HOST=https://marmot.example.com
export MARMOT_API_KEY=my-key
export MARMOT_READ_ONLY=true
```

### Config file

```bash
marmot config init
```

This creates `~/.config/marmot/config.yaml` interactively. You can also use `marmot config set <key> <value>` to set individual values.

| Key | Description | Default |
| --- | --- | --- |
| `host` | Marmot server URL | `http://localhost:8080` |
| `api_key` | API key for authentication | (none) |
| `ca_file` | PEM file of a CA to trust for the Marmot API in addition to the system's, also `--ca-file` or `MARMOT_CA_FILE` | (none) |
| `output` | Default output format (`table`, `json`, `yaml`) | `table` |
| `current_context` | Active context name | (none) |
| `read_only` | Refuse every command that changes something, also `--read-only` or `MARMOT_READ_ONLY`. See [read-only mode](#read-only-mode-and-exit-codes) | `false` |

---

## Output formats

All commands support `--output` / `-o` with `table` (default), `json` or `yaml`.

```bash
marmot assets list -o json | jq '.assets[].name'
```

## Read-only mode and exit codes

For an agent or script driving the CLI, turn on read-only mode unless it's meant to change things: `--read-only`, `MARMOT_READ_ONLY=true` or `read_only: true` in the config file. Every command that writes to the server or changes local configuration then exits with code 9 instead of running. It's a guardrail, not a security boundary: the API key's permissions decide what's allowed.

Branch on the exit code rather than the error message:

| Code | Name | Meaning |
| --- | --- | --- |
| 0 | `ok` | The command succeeded |
| 1 | `error` | Any failure not covered below |
| 2 | `usage` | Wrong arguments or flags |
| 3 | `unauthenticated` | No valid credentials; run `marmot login` or set `MARMOT_API_KEY` |
| 4 | `forbidden` | Signed in but not allowed, including a data query you may not run |
| 5 | `not_found` | The resource doesn't exist or isn't visible to you |
| 6 | `invalid` | The server rejected the request as invalid |
| 7 | `rate_limited` | Too many requests; wait and retry |
| 8 | `server` | The server failed or couldn't be reached; retrying later may help |
| 9 | `read_only` | The command changes something and read-only mode is on |
| 10 | `confirmation_required` | The command needs confirmation and there's no terminal; pass `--yes` once the change is agreed |
| 11 | `cancelled` | The confirmation prompt was declined |

`marmot agent schema` prints all of this as JSON, along with every command, flag and whether it reads or writes, so an agent can load it once instead of walking `--help`.

---

## Commands

All list commands support `--limit` and `--offset` for pagination. Destructive commands prompt for confirmation unless `--yes` is passed. Run `marmot <command> --help` for full flag details.

### marmot login

```
marmot login [url] [flags]
```

Authenticate with a Marmot instance via browser using OAuth 2.0 PKCE. A valid cached token is reused. If no URL is provided and no context is active, prompts for one. Creates a context automatically and registers the token with the Docker credential store for the instance's host.

| Flag | Description |
| --- | --- |
| `--force` | Sign in again even if a valid token is cached |
| `--print-token` | Print the access token on stdout; status messages go to stderr |
| `--no-launch-browser` | Print the sign-in URL instead of opening a browser; the callback URL can be pasted on stdin |

### marmot logout

```
marmot logout
```

Remove the cached authentication token for the active context, and the registry credential login registered for its host.

### marmot context

```
marmot context <list | use | delete>
```

Manage named contexts for switching between Marmot instances. Contexts are created automatically by `marmot login`.

| Subcommand | Description |
| --- | --- |
| `list` | Show all contexts with token status |
| `use <name>` | Switch active context |
| `delete <name>` | Remove context and its cached token |

### marmot assets

```
marmot assets <list | get | search | delete | summary | tags | owners> [flags]
marmot assets <identity | merge | upsert> [flags]
```

Browse, search and manage assets in your catalog. Use `list` and `search` with `--types`, `--providers` and `--tags` to filter results. `tags add` and `tags remove` take an asset ID and a tag.

`identity <id>` shows an asset's [MRN](MRNs/index.md), zone and previous MRNs, and each writer with its role and when it last saw the asset. `merge <id> --into <id>` folds an asset into another that turns out to be the same thing: owners, glossary terms, data products, subscriptions, lineage and documentation move across, and the merged asset's MRN keeps working as an alias. `upsert <mrn>` creates the asset an MRN names, or updates what you say about it if it exists.

```bash
marmot assets upsert \
  mrn://asset/table/postgresql@prod-eu/analytics.public.orders \
  --description "One row per order, from payment authorisation."

marmot assets merge <old-asset-id> --into <new-asset-id>
```

| Flag | On | Description |
| --- | --- | --- |
| `--into` | `merge` | ID of the asset to keep. Required. |
| `--yes`, `-y` | `merge` | Skip the confirmation prompt. |
| `--name` | `upsert` | Asset name. Defaults to the MRN's name. |
| `--type` | `upsert` | Asset type. Defaults to the MRN's type. |
| `--providers` | `upsert` | Comma-separated providers. Defaults to the MRN's provider. |
| `--description` | `upsert` | Description. |
| `--tags` | `upsert` | Comma-separated tags. |

### marmot search

```
marmot search <query> [flags]
```

Unified search across assets, glossary terms, teams and users. Filter by result type with `--types`.

### marmot glossary

```
marmot glossary <list | get | search | create | update | delete> [flags]
```

Manage glossary terms. Create terms with `--name` and `--definition`, optionally nesting them under a parent with `--parent-id`.

### marmot runs

```
marmot runs <list | get | entities> [flags]
```

View pipeline ingestion runs. Filter with `--pipelines` and `--statuses`.

### marmot query

```
marmot query <sources | tables | run | history | show | stats> [flags]
```

Run read-only SQL against catalogued sources as yourself, for example `marmot query run shop-pg --table orders`. It needs the `assets:query` permission, which admins hold by default, and every query appears in **Query audit**.

### marmot lineage

```
marmot lineage get <asset-id> [flags]
```

View the upstream and downstream lineage graph for an asset. Control traversal depth with `--depth`.

### marmot mrn

```
marmot mrn resolve <mrn>
```

Show what an [MRN](MRNs/index.md) names: its kind, ID and current MRN. It accepts legacy MRNs and MRNs something used to have, and says when the MRN you gave has moved.

### marmot zones

```
marmot zones <list | bind | unbind>
```

Manage which [zone](MRNs/zones.md) each host belongs to. Plugins report the hosts they connect to; bind a host yourself when lineage from Kafka Connect, OpenLineage or another tool that only knows an address should land on a particular system's assets. A binding you make wins over what plugins report.

| Subcommand | Description |
| --- | --- |
| `list` | Show every binding with its provider, host, zone and origin (`discovered` or `manual`) |
| `bind <provider> <host:port> <zone>` | Bind a host to a zone |
| `unbind <provider> <host:port>` | Remove a host's binding |

### marmot users

```
marmot users <me | list | get> [flags]
```

View user information. `me` shows the currently authenticated user.

### marmot apikeys

```
marmot apikeys <list | create | delete> [flags]
```

Manage API keys for authentication. The full key is only shown once at creation time.

### marmot teams

```
marmot teams <list | get | members> [flags]
```

View teams and their members.

### marmot metrics

```
marmot metrics <summary | by-type | by-provider | top-assets | top-queries> [flags]
```

View catalog metrics and usage statistics. `top-assets` and `top-queries` require a time range via `--start` and `--end` (RFC3339 format, defaults to the last 30 days).

### marmot admin

```
marmot admin <reindex | reindex-status | serve-keys | rotate-serve-keys>
```

Administrative operations. `reindex` triggers a full search reindex and `reindex-status` checks its progress. `serve-keys` lists the key generations that secure [serving plugins](Configure/serving-plugins.md) and `rotate-serve-keys` rotates them: gracefully by default, or with `--immediate` after a suspected compromise. Pass `--ca-key-uri` and `--signing-key-uri` to rotate to keys held in a KMS.

### marmot service-accounts

```
marmot service-accounts <list | get | create | delete> [flags]
marmot service-accounts apikeys <list | create | delete> [flags]
```

Manage service accounts and their API keys. A service account is a principal with no person behind it: CI, an ingestion job, an agent. Create one with `--name` and an optional `--roles` list of role IDs, then issue it a key. An account holds up to five keys and the key itself is printed once, at creation.

```bash
marmot service-accounts create --name catalog-copilot \
  --description "Answers questions in #data-help. Owned by the platform team."

marmot service-accounts apikeys create \
  --service-account <id> --name production --expires-in-days 90
```

| Flag | On | Description |
| --- | --- | --- |
| `--name` | `create` | Account or key name. Required. |
| `--description` | `create` | What the account is for and who owns it. |
| `--roles` | `create` | Comma-separated role IDs to assign. |
| `--service-account` | `apikeys *` | The account to act on. Required. |
| `--expires-in-days` | `apikeys create` | Key expiry. `0` never expires. |

See [users, roles and access](Configure/access-control.md) for what to grant.

### marmot agent

```
marmot agent schema
```

Print every command, flag, exit code and setting as one JSON document, with whether each command reads, writes to the server or changes local configuration. See [read-only mode and exit codes](#read-only-mode-and-exit-codes).

### marmot plugins

```
marmot plugins <list | outdated | update | pin | unpin | rollback | reset | check>
```

Manage the versions of the core plugins your server runs, separately from Marmot upgrades. `outdated` lists plugins with an update or an open security advisory, `update <id>[@version]` moves one forward and `rollback` returns it to the version it ran before and pins it there. See [plugin updates](Configure/plugin-updates.md).

### marmot ingest

```
marmot ingest --config <file> [flags]
```

Run plugins on this machine and push what they find to Marmot, rather than having the server run them. Use it in CI, or when the source is not reachable from the Marmot server. The configuration file lists the plugins to run and their settings; see [populating with the CLI](Populating/CLI.md). `marmot plugin` is an alias and takes the same flags.

| Flag | Description |
| --- | --- |
| `--config`, `-c` | Path to the ingestion config file. Required. |
| `--quiet`, `-q` | Hide info logs and show errors only. On by default. |
| `--destroy`, `-d` | Delete every asset this pipeline created. Prompts for confirmation. |
| `--serve` | After ingesting, stay running and answer Marmot's queries for the run marked `queryable: true`. See [serving queries from your network](Configure/serving-plugins.md). |

### marmot generate-encryption-key

```
marmot generate-encryption-key
```

Print a new 32-byte key for `server.encryption_key`, which Marmot uses to encrypt pipeline credentials at rest. Losing the key means losing the credentials encrypted with it, so store it wherever you keep your other secrets before starting the server.

### marmot skills

```
marmot skills install [--project | --dir <path>]
```

Install the [Marmot skill](/docs/MCP/skills) for your AI agent, matching this version of the CLI.

### marmot credential-helper

```
marmot credential-helper <get | store | erase | list>
```

The Docker credential helper protocol, so `docker`, `crane` and `oras` can push to a registry served by your Marmot host using your `marmot login` session. You do not call this yourself: Docker does, through the `docker-credential-marmot` symlink the install script creates. See [authentication](#authentication) above.

### marmot run

```
marmot run [--config <file>]
```

Start a Marmot server from the same binary as the CLI. This is what the container image runs. See [deployment](Deploy/index.md) for running it in production.

### marmot config

```
marmot config <init | set | get | list>
```

Manage CLI configuration. See [Configuration](#configuration) above for details.

---

## Tab completion

Generate shell completions with `marmot completion <shell>`. Supported shells are `bash`, `zsh`, `fish` and `powershell`.

```bash
source <(marmot completion bash)
```

---

## Next steps

- [Populating with the CLI](Populating/CLI.md): run plugins on your own machine and push what they find.
- [Search and the query language](queries.md): the syntax for precise searches.
- [REST API](/api): everything the CLI does, for your own integrations.
- [Discord](https://discord.gg/TWCk7hVFN4): ask questions and talk to other Marmot users.
