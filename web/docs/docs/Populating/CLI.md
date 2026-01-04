---
sidebar_position: 2
---

The Marmot CLI provides an easy way to populate your data catalog by ingesting metadata from various data sources. This document explains how to use the `ingest` command to bring your data assets into Marmot.

## Overview

The `ingest` command discovers metadata from configured data sources and catalogs them as assets in Marmot. It supports multiple data sources, can establish lineage relationships between assets, and can attach documentation to assets.

## Installation

### Automatic Installation

You can install Marmot with the automatic installation script, it's strongly recommended you inspect the contents of any script before piping it into bash.

```bash
curl -fsSL get.marmotdata.io | sh
```

### Manual Installation

If you prefer to install manually:

1. Download the latest Marmot binary for your platform from [GitHub Releases](https://github.com/marmotdata/marmot/releases)
2. Make the binary executable:
   ```bash
   chmod +x marmot
   ```
3. Move the binary to a location in your PATH:
   ```bash
   sudo mv marmot /usr/local/bin/
   ```

## Command Syntax

```bash
marmot ingest [flags]
```

### Flags

| Flag              | Description                                        |
| ----------------- | -------------------------------------------------- |
| `--config`, `-c`  | Path to ingestion config file                      |
| `--host`, `-H`    | Marmot API host (default: "http://localhost:8080") |
| `--api-key`, `-k` | API key for authentication                         |
| `--destroy`, `-d` | Delete all resources for this pipeline             |

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

Where `source_type` is one of the supported data source types. You can find all [available source types and their available configuration by looking at the Plugins documentation.](/docs/plugins)

** It's important to give your pipeline a unique name, this is used to track the state of the ingestion **

## Example: Ingesting Kafka Topics

Here's an example configuration for ingesting metadata from a Kafka cluster:

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

This configuration:

1. Connects to a Kafka broker at `kafka-broker:9092` with SASL PLAIN authentication
2. Integrates with a Schema Registry at `http://schema-registry:8081`

To run the ingestion:

```bash
marmot ingest -c /path/to/config.yaml -H http://marmot-server:8080 -k your-api-key
```

## Batch Processing

The ingest command processes and uploads assets in batches for efficiency. Lineage relationships are also established between assets where possible.
