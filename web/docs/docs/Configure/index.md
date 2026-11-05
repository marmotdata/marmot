---
title: Configuration reference
description: Every setting for a self-hosted Marmot server, its environment variable and its default.
---

# Configuration reference

Configure a self-hosted Marmot with a YAML file or environment variables. Every setting is optional apart from the database, and each table lists its default.

## Configuration file

By default, Marmot looks for `config.yaml` in the current directory. Use the `--config` flag to specify a different path.

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: <database-password>
  name: marmot
server:
  host: 0.0.0.0
  port: 8080
logging:
  level: info
  format: json
```

## Environment variables

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

See [TLS](./tls.md) to set up TLS and the [Helm deployment guide](../Deploy/Helm.md#encryption-key) for managing the encryption key in Kubernetes.

Set `server.root_url` to the address people use, such as `https://marmot.example.com`, before you turn on single sign-on or use `marmot login`. Marmot builds its sign-in callback URLs from it:

```yaml
server:
  root_url: https://marmot.example.com
```

## Database

Marmot needs PostgreSQL 14 or later, and a database user that can create tables and indexes.

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

Set the format to `console` for readable output while you develop.

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

See [anonymous access](./anonymous-access.md) for anonymous browsing and [MCP authentication](../MCP/index.md#hosted-clients) for allowing hosted MCP clients such as claude.ai to sign in with OAuth.

### Single sign-on providers

Single sign-on providers are configured under `auth.<provider>`, where `<provider>` is one of `google`, `github`, `gitlab`, `keycloak`, `okta`, `slack` or `auth0`, or `generic_oidc` for any other OIDC-compliant provider. The common keys are:

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

Each provider has its own options and setup steps. See [single sign-on](./Authentication/index.md) for each provider's guide.

## Search

| Key              | Description                     | Default |
| ---------------- | ------------------------------- | ------- |
| `search.timeout` | Search query timeout in seconds | `10`    |

The optional Elasticsearch backend is configured under `search.elasticsearch`. See [search backend](./elasticsearch.md) for its options.

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

## Kubernetes operator

| Key                        | Description                                              | Default         |
| -------------------------- | --------------------------------------------------------- | --------------- |
| `operator.enabled`         | Synchronise scheduled pipelines with the Marmot operator  | `false`         |
| `operator.namespace`       | Namespace the operator manages pipeline resources in      | -               |
| `operator.service_account` | Service account used for pipeline resources               | `marmot-ingest` |

## Data queries

| Key                                     | Description                                                                 | Default |
| --------------------------------------- | --------------------------------------------------------------------------- | ------- |
| `query.enabled`                         | Let people and agents query sources set up to answer queries               | `true`  |
| `query.query_timeout`                   | Seconds before a query is cancelled                                         | `60`    |
| `query.max_rows_default`                | Rows returned when a query doesn't ask for a limit                          | `1000`  |
| `query.max_rows_cap`                    | Most rows any query can return                                              | `10000` |
| `query.instance_idle_ttl`               | Seconds an unused plugin process stays alive between queries                | `300`   |
| `query.ingest_on_query_source_interval` | Minimum seconds between query-triggered refreshes of one source             | `3600`  |
| `query.ingest_on_query_global_interval` | Minimum seconds between any two query-triggered refreshes                   | `60`    |

## Serving plugins

| Key                           | Description                                                                         | Default |
| ----------------------------- | ----------------------------------------------------------------------------------- | ------- |
| `query.serve.enabled`         | Accept plugins that serve queries from outside Marmot                                | `false` |
| `query.serve.listen_address`  | Address the serve listener binds                                                     | `:8443` |
| `query.serve.public_address`  | `host:port` serving plugins dial, when not the Marmot host on the listen port        | -       |
| `query.serve.replica_address` | Host, or `host:port`, other replicas forward queries to; only needed with several    | -       |
| `query.serve.ca_key_uri`      | KMS URI of the serving CA key (ECDSA P-256); stored in the database when unset        | -       |
| `query.serve.signing_key_uri` | KMS URI of the query signing key (ECDSA P-256)                                        | -       |

See [serve queries from your network](./serving-plugins.md).

## OpenLineage

| Key                        | Description                                         | Default |
| -------------------------- | ---------------------------------------------------- | ------- |
| `openlineage.auth.enabled` | Require authentication for the OpenLineage endpoint  | `true`  |

## Rate limiting

| Key                  | Description                        | Default |
| -------------------- | ----------------------------------- | ------- |
| `rate_limit.enabled` | Enable in-memory API rate limiting  | `false` |

## Banner

| Key                     | Description                              | Default    |
| ----------------------- | ----------------------------------------- | ---------- |
| `ui.banner.enabled`     | Show the banner                           | `false`    |
| `ui.banner.dismissible` | Allow users to dismiss the banner         | `true`     |
| `ui.banner.variant`     | Style (info, warning, error, success)     | `info`     |
| `ui.banner.message`     | Banner text                               | -          |
| `ui.banner.id`          | Identifier used to remember dismissals    | `banner-1` |

See [banner](./banner.md) for examples.

## Interface language

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

## Plugins

| Key                   | Description                                                          | Default |
| --------------------- | --------------------------------------------------------------------- | ------- |
| `plugins.registry`    | OCI registry namespace to install core plugins from, e.g. an internal mirror | -       |
| `plugins.autoinstall` | Pull missing core plugins from the registry at startup                | `true`  |
| `plugins.updates.enabled` | Check the signed plugin index for new plugin versions and advisories | `true` |
| `plugins.updates.auto_update` | Plugin updates Marmot applies by itself: `off`, `security` or `patch` | `security` |
| `plugins.updates.check_interval` | Seconds between plugin index checks | `43200` |
| `plugins.updates.index` | OCI reference or `https://` URL of the plugin index | `<registry>/index:latest` |

See [plugin updates](./plugin-updates.md) for how plugin versions are chosen and changed.
