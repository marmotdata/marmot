# marmot-plugin-elasticsearch

Marmot plugin for [Elasticsearch](https://www.elastic.co/elasticsearch). Discovers indices, data streams, and aliases from an Elasticsearch cluster and produces:

- **Indices** as `Table` assets with health/status, shard/replica counts, doc counts, store size, creation date, and a JSON Schema derived from the index mapping (nested fields flattened as dotted paths).
- **Data streams** as `Data Stream` assets with the timestamp field, backing-index count, generation, status, ILM policy, and template — plus `CONTAINS` lineage edges to each backing index.
- **Aliases** as `Alias` assets listing target indices and write-index/filter flags — plus `REFERENCES` lineage edges to each target index.
- **Statistics**: `docs_count` per index (when `include_index_stats` is enabled).

Supports self-hosted clusters via `addresses` and Elastic Cloud via `cloud_id`. Auth is either API key or basic auth (mutually exclusive). TLS verification can be skipped or backed by a custom CA.

## Example Configurations

### Self-hosted with basic auth

```yaml
addresses:
  - "https://elasticsearch.company.com:9200"
username: "elastic"
password: "${ES_PASSWORD}"
tags:
  - "elasticsearch"
  - "search"
```

### Elastic Cloud with API key

```yaml
cloud_id: "my-deployment:dXMtY2VudHJhbC0xLmdjcC5jbG91ZC5lcy5pbzo0NDMkMGNmNQ=="
api_key: "${ES_API_KEY}"
include_index_stats: true
```

### Custom CA and system indices

```yaml
addresses:
  - "https://elasticsearch.internal:9200"
username: "elastic"
password: "${ES_PASSWORD}"
ca_cert_path: "/etc/ssl/certs/company-ca.pem"
include_system_indices: true
filter:
  exclude:
    - "^\\.security-.*"
```

## Development

Build and test:

```sh
make build
make test
```

To run a local build inside Marmot:

```sh
make install
```

This copies the binary to `~/.marmot/plugins/`, the directory Marmot scans for local plugins. A local plugin shadows the released core plugin with the same name: Marmot skips downloading it and loads your build instead. Delete the binary from `~/.marmot/plugins/` to fall back to the released version.

If your Marmot runs with a custom plugins directory (`MARMOT_PLUGINS_DIR`), set the same value for `make install` so both point at the same place.
