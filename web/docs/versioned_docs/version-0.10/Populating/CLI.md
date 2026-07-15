---
sidebar_position: 2
---

import { TipBox } from '@site/src/components/Steps';
import { CliInstall } from '@site/src/components/CliInstall';

# CLI

The `ingest` command discovers metadata from configured data sources and catalogs them as assets in Marmot. It supports multiple data sources, can establish lineage relationships between assets and can attach documentation to assets.

## Installation

<CliInstall />

See the [CLI Reference](/docs/cli) for configuring the host, API key and other global options.

## Configuration File

The ingest command requires a YAML configuration file that defines the data sources to ingest. The configuration follows this structure:

```yaml
name: my_pipeline_name
runs:
  - source_type1:
      # source-specific configuration
  - source_type2:
      # source-specific configuration
```

Where `source_type` is one of the supported data source types. You can find all [available source types and their configuration in the Plugins documentation.](/docs/Plugins)

<TipBox variant="info" title="Pipeline Names">
Give your pipeline a unique name. This is used to track the state of the ingestion.
</TipBox>

## Example: Ingesting Kafka Topics

```yaml
runs:
  - kafka:
      bootstrap_servers: "kafka-broker:9092"
      client_id: "marmot-kafka-plugin"
      client_timeout_seconds: 60
      authentication:
        type: "sasl_plaintext"
        username: "username"
        password: "password"
        mechanism: "PLAIN"
      schema_registry:
        url: "http://schema-registry:8081"
        enabled: true
        config:
          basic.auth.user.info: "username:password"
```

This configuration connects to a Kafka broker at `kafka-broker:9092` with SASL PLAIN authentication and integrates with a Schema Registry at `http://schema-registry:8081`.

```bash
marmot ingest -c config.yaml
```

## Where Plugins Run

Discovery runs wherever the CLI runs, not on the Marmot server. The CLI connects to your data sources directly and pushes the discovered assets to the Marmot API. This means the machine running `marmot ingest` needs network access to the data sources, while the Marmot server does not: it only receives the results.

On the first ingest, the CLI downloads the core plugins from `ghcr.io/marmotdata/plugins` and caches them under `~/.marmot/plugins/cache`. Later runs load them straight from the cache. Two environment variables control this:

- `MARMOT_PLUGINS_AUTOINSTALL=false` disables the download, for example on air-gapped runners with pre-installed plugins
- `MARMOT_PLUGINS_REGISTRY` installs from a registry mirror instead of GHCR

## Running in CI

Because plugins run local to the CLI, ingestion works anywhere the CLI can run, such as a GitHub Actions workflow on a schedule:

```yaml
jobs:
  ingest:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5

      - name: Install Marmot CLI
        run: curl -fsSL get.marmotdata.io | sh

      - name: Cache Marmot plugins
        uses: actions/cache@v4
        with:
          path: ~/.marmot/plugins/cache
          key: marmot-plugins-${{ runner.os }}

      - name: Ingest
        run: marmot ingest -c config.yaml
        env:
          MARMOT_HOST: https://marmot.example.com
          MARMOT_API_KEY: ${{ secrets.MARMOT_API_KEY }}
```

Caching `~/.marmot/plugins/cache` is optional; without it the CLI re-downloads the plugins on each run.
