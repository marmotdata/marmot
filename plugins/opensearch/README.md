# marmot-plugin-opensearch

Marmot plugin for [OpenSearch](https://opensearch.org/). Discovers:

- **Indices** as `Table` assets with health, status, shard/replica counts, document counts, store size, and field mappings flattened into a column schema. System indices (prefixed with `.`) are excluded by default.
- **Data streams** as `Data Stream` assets with timestamp field, generation, and backing index count.
- **Aliases** as `Alias` assets with their target indices, write-index flag, and filter state.
- **Lineage**: CONTAINS edges from data streams to their backing indices, and REFERENCES edges from aliases to the indices they point at.
- **Statistics**: a `docs_count` metric per index.

## Example Configuration

```yaml
addresses:
  - "https://opensearch.company.com:9200"
username: "admin"
password: "${OPENSEARCH_PASSWORD}"
tags:
  - "opensearch"
  - "search"
```

Use `tls_skip_verify` for self-signed clusters or `ca_cert_path` to trust a custom CA.

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
