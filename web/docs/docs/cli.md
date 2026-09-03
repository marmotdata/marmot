---
sidebar_position: 3
title: CLI Reference
---

# CLI Reference

The Marmot CLI lets you interact with your data catalog directly from the terminal.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { CliInstall } from '@site/src/components/CliInstall';

<CalloutCard
  title="Looking for Ingestion?"
  description="The CLI also supports populating your catalog from data sources via the ingest command."
  docId="Populating/CLI"
  buttonText="View Ingestion Docs"
  icon="mdi:database-plus"
/>

## Installation

<CliInstall />

---

## Authentication

Sign in once with `marmot login`. It opens your browser, you sign in the way you always do, and the CLI keeps a token for you.

```bash
marmot login https://marmot.example.com
```

That is all most people need. Every other command uses the token from then on. It lasts 24 hours; run `marmot login` again when it has expired.

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

The CLI opens the Marmot sign-in page in your browser and waits on a local port for the browser to come back with a code. It swaps that code for a token and saves it in `~/.config/marmot/credentials.json`. The token is a signed JWT with your user id, your roles and their permissions, and an expiry. This is plain OAuth 2.0 with PKCE. There is no client secret and nothing to configure.

### Pushing to a registry on your Marmot host

Login also makes the token available to `docker`, `crane` and `oras` for the Marmot host, so those tools can push to a registry served there without a `docker login`.

This needs `docker-credential-marmot` on your `PATH`. It is just the marmot binary under another name, and the install script sets it up. If you installed marmot some other way:

```bash
ln -s "$(command -v marmot)" "$(dirname "$(command -v marmot)")/docker-credential-marmot"
```

Without it, login writes the token straight into `~/.docker/config.json`. That works until the token expires, and not at all if Docker Desktop manages your credentials. Login tells you when that is the case.

### API Keys

API keys can still be used and always take priority over cached login tokens.

```bash
# Via flag
marmot assets list --api-key mrmot_abc123

# Via environment variable
export MARMOT_API_KEY=mrmot_abc123
```

### Auth Priority

When multiple credentials are available, the CLI uses the first one found:

1. `--api-key` flag or `MARMOT_API_KEY` environment variable
2. Cached OAuth token from `marmot login` (for the active context)
3. Kubernetes service account token (auto-detected in-cluster)

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

### CLI Flags

```bash
marmot assets list --host https://marmot.example.com --api-key my-key
```

### Environment Variables

```bash
export MARMOT_HOST=https://marmot.example.com
export MARMOT_API_KEY=my-key
```

### Config File

```bash
marmot config init
```

This creates `~/.config/marmot/config.yaml` interactively. You can also use `marmot config set <key> <value>` to set individual values.

| Key | Description | Default |
| --- | --- | --- |
| `host` | Marmot server URL | `http://localhost:8080` |
| `api_key` | API key for authentication | (none) |
| `output` | Default output format (`table`, `json`, `yaml`) | `table` |
| `current_context` | Active context name | (none) |

---

## Output Formats

All commands support `--output` / `-o` with `table` (default), `json` or `yaml`.

```bash
marmot assets list -o json | jq '.assets[].name'
```

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
```

Browse, search and manage assets in your catalog. Use `list` and `search` with `--types`, `--providers` and `--tags` to filter results.

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

### marmot lineage

```
marmot lineage get <asset-id> [flags]
```

View the upstream and downstream lineage graph for an asset. Control traversal depth with `--depth`.

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
marmot admin <reindex | reindex-status>
```

Administrative operations. `reindex` triggers a full search reindex and `reindex-status` checks its progress.

### marmot config

```
marmot config <init | set | get | list>
```

Manage CLI configuration. See [Configuration](#configuration) above for details.

---

## Tab Completion

Generate shell completions with `marmot completion <shell>`. Supported shells are `bash`, `zsh`, `fish` and `powershell`.

```bash
source <(marmot completion bash)
```

---

## Next Steps

<DocCardGrid>
  <DocCard
    title="Populating Your Catalog"
    description="Learn about all the ways to add assets to Marmot"
    docId="Populating/index"
    icon="mdi:database-plus"
  />
  <DocCard
    title="Query Language"
    description="Use advanced search queries to find assets"
    docId="queries"
    icon="mdi:code-tags"
  />
  <DocCard
    title="REST API"
    description="View the full API documentation for custom integrations"
    href="/api"
    icon="mdi:api"
  />
  <DocCard
    title="Deployment Options"
    description="Deploy Marmot to production with Docker, Helm or the CLI"
    docId="Deploy/index"
    icon="mdi:cloud-upload"
  />
</DocCardGrid>

<CalloutCard
  title="Need Help?"
  description="Join the Discord community to ask questions and connect with other Marmot users."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
