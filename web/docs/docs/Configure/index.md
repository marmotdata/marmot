# Configure

Marmot is configured using a YAML file or environment variables. All settings have sensible defaults so you only need to specify what you want to change. Every option is listed below with its default value.

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

## Configuration Topics

<DocCardGrid>
  <DocCard
    title="Authentication"
    description="Set up SSO with Google, GitHub, GitLab, Okta, Slack or Auth0"
    docId="Configure/Authentication/index"
    icon="mdi:shield-account"
  />
  <DocCard
    title="Anonymous Access"
    description="Allow users to browse the Catalog without logging in"
    docId="Configure/anonymous-access"
    icon="mdi:incognito"
  />
  <DocCard
    title="TLS"
    description="Set up custom TLS configuration"
    docId="Configure/tls"
    icon="mdi:lock"
  />
  <DocCard
    title="Customisable Banner"
    description="Display announcements and notices to users"
    docId="Configure/banner"
    icon="mdi:message-alert"
  />
  <DocCard
    title="Elasticsearch"
    description="Enhance search with deep fuzzy matching across all fields"
    docId="Configure/elasticsearch"
    icon="mdi:magnify"
  />
  <DocCard
    title="Translations"
    description="Set the interface language and contribute new translations"
    docId="Develop/translations"
    icon="mdi:translate"
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

All configuration options can be set via environment variables using the `MARMOT_` prefix with underscores separating nested keys. For example, `database.host` becomes `MARMOT_DATABASE_HOST`. List values can be given as comma-separated environment variables.

## Server

| Key                              | Description                                                                        | Default   |
| -------------------------------- | ---------------------------------------------------------------------------------- | --------- |
| `server.host`                    | Bind address                                                                        | `0.0.0.0` |
| `server.port`                    | HTTP port                                                                           | `8080`    |
| `server.root_url`                | Public URL of this Marmot instance                                                  | -         |
| `server.custom_response_headers` | Extra HTTP headers added to every response (config file only)                       | -         |
| `server.encryption_key`          | Key used to encrypt stored credentials at rest, generate one with `marmot generate-encryption-key` | -         |
| `server.allow_unencrypted`       | Allow running without an encryption key, storing credentials unencrypted            | `false`   |
| `server.tls.cert_path`           | Path to server TLS certificate                                                      | -         |
| `server.tls.key_path`            | Path to server TLS private key                                                      | -         |
| `server.tls.ca_cert_path`        | Path to CA cert for client verification (mTLS)                                      | -         |

See [TLS](./tls.md) for setting up TLS and the [Helm deployment guide](../Deploy/Helm.md#encryption-key) for managing the encryption key in Kubernetes.

:::info Root URL Required for Authentication
`server.root_url` must be set when using OAuth/OIDC authentication or CLI login (`marmot login`). It is the URL that users access Marmot from (e.g. `https://marmot.example.com`). This is used to generate OAuth callback URLs and redirect users after authentication.

```yaml
server:
  root_url: https://marmot.example.com
```

Or via environment variable:

```bash
export MARMOT_SERVER_ROOT_URL=https://marmot.example.com
```
:::

## Database

Marmot requires PostgreSQL 14 or later. Ensure the database user has privileges to create tables and indexes.

| Key                      | Description                              | Default     |
| ------------------------ | ---------------------------------------- | ----------- |
| `database.host`          | PostgreSQL host                          | `localhost` |
| `database.port`          | PostgreSQL port                          | `5432`      |
| `database.user`          | Database username                        | `postgres`  |
| `database.password`      | Database password                        | `postgres`  |
| `database.name`          | Database name                            | `marmot`    |
| `database.sslmode`       | SSL mode (disable, require, verify-full) | `disable`   |
| `database.max_conns`     | Maximum open connections                 | `50`        |
| `database.idle_conns`    | Minimum idle connections                 | `25`        |
| `database.conn_lifetime` | Connection lifetime in minutes           | `5`         |

## Logging

Marmot uses structured logging. Set the format to `console` for human-readable output during development.

| Key              | Description                                                | Default |
| ---------------- | ---------------------------------------------------------- | ------- |
| `logging.level`  | Log level (trace, debug, info, warn, error, fatal, panic)  | `info`  |
| `logging.format` | Output format (json, console)                              | `json`  |

## Authentication

| Key                                | Description                                                                 | Default |
| ---------------------------------- | --------------------------------------------------------------------------- | ------- |
| `auth.anonymous.enabled`           | Allow browsing without logging in                                            | `false` |
| `auth.anonymous.role`              | Role assigned to anonymous users                                             | `user`  |
| `auth.dcr.allowed_redirect_hosts`  | Hosts allowed in https redirect URIs for dynamically registered OAuth clients; empty keeps registration loopback-only | -       |

See [Anonymous Access](./anonymous-access.md) for anonymous browsing and [MCP authentication](../MCP/index.md#hosted-clients-oauth) for allowing hosted MCP clients such as claude.ai to sign in with OAuth.

### SSO providers

Single Sign-On providers are configured under `auth.<provider>`, where `<provider>` is one of `google`, `github`, `gitlab`, `keycloak`, `okta`, `slack` or `auth0`, or `generic_oidc` for any other OIDC-compliant provider. The common keys are:

| Key                              | Description                                        | Default            |
| -------------------------------- | -------------------------------------------------- | ------------------ |
| `auth.<provider>.enabled`        | Enable this provider                               | `false`            |
| `auth.<provider>.client_id`      | OAuth client ID                                    | -                  |
| `auth.<provider>.client_secret`  | OAuth client secret                                | -                  |
| `auth.<provider>.url`            | Provider URL, where applicable                     | provider-specific  |
| `auth.<provider>.redirect_url`   | OAuth callback URL for this Marmot instance        | -                  |
| `auth.<provider>.scopes`         | OAuth scopes to request                            | provider-specific  |
| `auth.<provider>.allow_signup`   | Create Marmot users on first sign-in               | `true`             |
| `auth.<provider>.team_sync`      | Synchronise team membership from provider groups   | disabled           |
| `auth.<provider>.group_mapping`  | Map provider groups to Marmot roles                | -                  |

Each provider has its own options and setup steps — see [Authentication](./Authentication/index.md) for the per-provider guides.

## Search

| Key              | Description                     | Default |
| ---------------- | ------------------------------- | ------- |
| `search.timeout` | Search query timeout in seconds | `10`    |

The optional Elasticsearch backend is configured under `search.elasticsearch` — see [Elasticsearch](./elasticsearch.md) for the full list of options.

## Metrics

| Key                                     | Description                                                       | Default                          |
| --------------------------------------- | ------------------------------------------------------------------ | -------------------------------- |
| `metrics.enabled`                       | Serve Prometheus metrics on a separate port at `/metrics`          | `false`                          |
| `metrics.port`                          | Port for the metrics server                                        | `9090`                           |
| `metrics.owner_metadata_fields`         | Metadata fields used to attribute asset ownership in catalog metrics | `owner, ownedBy, owningTeam`     |
| `metrics.schemas.excluded_asset_types`  | Asset types excluded from schema coverage metrics                   | `Service`                        |
| `metrics.schemas.excluded_providers`    | Providers excluded from schema coverage metrics                     | -                                |

## Pipelines

| Key                            | Description                                                | Default |
| ------------------------------ | ----------------------------------------------------------- | ------- |
| `pipelines.max_workers`        | Maximum concurrent pipeline runs processed by this instance | `10`    |
| `pipelines.scheduler_interval` | Seconds between scheduler checks for due pipeline runs      | `60`    |
| `pipelines.lease_expiry`       | Seconds before an abandoned run lease is reclaimed          | `300`   |
| `pipelines.claim_expiry`       | Seconds before an unstarted run claim expires               | `30`    |

## Kubernetes Operator

| Key                        | Description                                              | Default         |
| -------------------------- | --------------------------------------------------------- | --------------- |
| `operator.enabled`         | Synchronise scheduled pipelines with the Marmot operator  | `false`         |
| `operator.namespace`       | Namespace the operator manages pipeline resources in      | -               |
| `operator.service_account` | Service account used for pipeline resources               | `marmot-ingest` |

## OpenLineage

| Key                        | Description                                         | Default |
| -------------------------- | ---------------------------------------------------- | ------- |
| `openlineage.auth.enabled` | Require authentication for the OpenLineage endpoint  | `true`  |

## Rate Limiting

| Key                  | Description                        | Default |
| -------------------- | ----------------------------------- | ------- |
| `rate_limit.enabled` | Enable in-memory API rate limiting  | `false` |

## UI Banner

| Key                     | Description                              | Default    |
| ----------------------- | ----------------------------------------- | ---------- |
| `ui.banner.enabled`     | Show the banner                           | `false`    |
| `ui.banner.dismissible` | Allow users to dismiss the banner         | `true`     |
| `ui.banner.variant`     | Style (info, warning, error, success)     | `info`     |
| `ui.banner.message`     | Banner text                               | -          |
| `ui.banner.id`          | Identifier used to remember dismissals    | `banner-1` |

See [Customisable Banner](./banner.md) for examples.

## UI Language

| Key                   | Description                                                             | Default |
| --------------------- | ----------------------------------------------------------------------- | ------- |
| `ui.default_language` | Language tag for visitors with no preference, empty detects the browser | -       |

See [Translations](../Develop/translations.md) for the available languages and how to contribute one.

## Telemetry

| Key                  | Description                          | Default                                    |
| -------------------- | ------------------------------------- | ------------------------------------------ |
| `telemetry.enabled`  | Send anonymous usage telemetry        | `true`                                     |
| `telemetry.endpoint` | Telemetry ingest endpoint             | `https://telemetry.marmotdata.io/v1/ingest` |
| `telemetry.interval` | Seconds between telemetry reports     | `86400`                                    |

See [Telemetry](./telemetry.md) for what is collected and how to opt out.

## Experimental

| Key                          | Description                                          | Default |
| ---------------------------- | ----------------------------------------------------- | ------- |
| `experimental.table_preview` | Show sample rows on asset detail pages                | `false` |

See [Table Preview](./table-preview.md) before enabling this.

## Plugins

| Key                   | Description                                                          | Default |
| --------------------- | --------------------------------------------------------------------- | ------- |
| `plugins.registry`    | OCI registry namespace to install core plugins from, e.g. an internal mirror | -       |
| `plugins.autoinstall` | Pull missing core plugins from the registry at startup                | `true`  |
