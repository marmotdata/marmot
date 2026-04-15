---
sidebar_position: 2
---

import { Tabs, TabPanel, TipBox } from '@site/src/components/Steps';

# CLI

The `ingest` command discovers metadata from configured data sources and catalogs them as assets in Marmot. It supports multiple data sources, can establish lineage relationships between assets and can attach documentation to assets.

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
