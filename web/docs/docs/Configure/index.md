# Configure

Marmot is configured using a YAML file or environment variables. All settings have sensible defaults so you only need to specify what you want to change.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

## Configuration Topics

<DocCardGrid>
  <DocCard
    title="Authentication"
    description="Set up SSO with Google, GitHub, GitLab, Okta, Slack or Auth0"
    href="/docs/Configure/Authentication"
    icon="mdi:shield-account"
  />
  <DocCard
    title="Anonymous Access"
    description="Allow users to browse the Catalog without logging in"
    href="/docs/Configure/anonymous-access"
    icon="mdi:incognito"
  />
  <DocCard
    title="Customisable Banner"
    description="Display announcements and notices to users"
    href="/docs/Configure/banner"
    icon="mdi:message-alert"
  />
</DocCardGrid>

## Configuration File

By default, Marmot looks for `config.yaml` in the current directory. Use the `--config` flag to specify a different path.

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: secret
  name: marmot
server:
  host: 0.0.0.0
  port: 8080
logging:
  level: info
  format: json
```

## Environment Variables

All configuration options can be set via environment variables using the `MARMOT_` prefix with underscores separating nested keys. For example, `database.host` becomes `MARMOT_DATABASE_HOST`.

## Database

Marmot requires PostgreSQL 14 or later. Ensure the database user has privileges to create tables and indexes.

| Key | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `database.host` | PostgreSQL host | `localhost` | `MARMOT_DATABASE_HOST` |
| `database.port` | PostgreSQL port | `5432` | `MARMOT_DATABASE_PORT` |
| `database.user` | Database username | `postgres` | `MARMOT_DATABASE_USER` |
| `database.password` | Database password | - | `MARMOT_DATABASE_PASSWORD` |
| `database.name` | Database name | `marmot` | `MARMOT_DATABASE_NAME` |
| `database.sslmode` | SSL mode (disable, require, verify-full) | `disable` | `MARMOT_DATABASE_SSLMODE` |
| `database.maxConns` | Maximum open connections | `10` | `MARMOT_DATABASE_MAX_CONNS` |
| `database.idleConns` | Minimum idle connections | `5` | `MARMOT_DATABASE_IDLE_CONNS` |
| `database.connLifetime` | Connection lifetime in minutes | `30` | `MARMOT_DATABASE_CONN_LIFETIME` |

## Server

| Key | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `server.host` | Bind address | `0.0.0.0` | `MARMOT_SERVER_HOST` |
| `server.port` | Port number | `8080` | `MARMOT_SERVER_PORT` |

## Logging

Marmot uses structured logging. Set the format to `console` for human-readable output during development.

| Key | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `logging.level` | Log level (debug, info, warn, error) | `info` | `MARMOT_LOGGING_LEVEL` |
| `logging.format` | Output format (json, console) | `json` | `MARMOT_LOGGING_FORMAT` |

## OpenLineage

| Key | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `openlineage.auth.enabled` | Require authentication for the OpenLineage endpoint | `true` | `MARMOT_OPENLINEAGE_AUTH_ENABLED` |
