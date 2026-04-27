---
sidebar_position: 3
title: CLI Reference
---

# CLI Reference

The Marmot CLI lets you interact with your data catalog directly from the terminal.

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { Steps, Step, Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

<CalloutCard
  title="Looking for Ingestion?"
  description="The CLI also supports populating your catalog from data sources via the ingest command."
  href="/docs/Populating/CLI"
  buttonText="View Ingestion Docs"
  icon="mdi:database-plus"
/>

## Installation

<Tabs items={[
{ label: "Automatic", value: "auto", icon: "mdi:download" },
{ label: "Manual", value: "manual", icon: "mdi:folder-download" }
]}>
<TabPanel>

```bash
curl -fsSL get.marmotdata.io | sh
```

</TabPanel>
<TabPanel>

Download the latest binary for your platform from [GitHub Releases](https://github.com/marmotdata/marmot/releases), then:

```bash
chmod +x marmot && sudo mv marmot /usr/local/bin/
```

</TabPanel>
</Tabs>

---

## Configuration

Before using CLI commands you need to tell Marmot where your server is and how to authenticate. There are three ways to do this, listed in order of precedence: CLI flags, environment variables and config file.

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

---

## Output Formats

All commands support `--output` / `-o` with `table` (default), `json` or `yaml`.

```bash
marmot assets list -o json | jq '.assets[].name'
```

---

## Commands

All list commands support `--limit` and `--offset` for pagination. Destructive commands prompt for confirmation unless `--yes` is passed. Run `marmot <command> --help` for full flag details.

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
    href="/docs/Populating"
    icon="mdi:database-plus"
  />
  <DocCard
    title="Query Language"
    description="Use advanced search queries to find assets"
    href="/docs/queries"
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
    href="/docs/Deploy"
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
