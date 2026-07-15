# Elasticsearch

Marmot can optionally use Elasticsearch to enhance search with deep fuzzy matching across all record fields including metadata, descriptions, documentation and schemas.

## Configuration

### YAML

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    index: "marmot"
```

### Environment Variables

```
MARMOT_SEARCH_ELASTICSEARCH_ENABLED=true
MARMOT_SEARCH_ELASTICSEARCH_ADDRESSES=http://localhost:9200
MARMOT_SEARCH_ELASTICSEARCH_INDEX=marmot
```

## Options

| Option                                  | Description                                                    | Default         | Environment Variable                           |
| --------------------------------------- | -------------------------------------------------------------- | --------------- | ---------------------------------------------- |
| `search.elasticsearch.enabled`          | Enable Elasticsearch for text search                           | `false`         | `MARMOT_SEARCH_ELASTICSEARCH_ENABLED`          |
| `search.elasticsearch.addresses`        | List of Elasticsearch node URLs                                | -               | `MARMOT_SEARCH_ELASTICSEARCH_ADDRESSES`        |
| `search.elasticsearch.username`         | HTTP Basic Auth username                                       | -               | `MARMOT_SEARCH_ELASTICSEARCH_USERNAME`         |
| `search.elasticsearch.password`         | HTTP Basic Auth password                                       | -               | `MARMOT_SEARCH_ELASTICSEARCH_PASSWORD`         |
| `search.elasticsearch.index`            | Name of the Elasticsearch index                                | `marmot`        | `MARMOT_SEARCH_ELASTICSEARCH_INDEX`            |
| `search.elasticsearch.bulk_size`        | Number of documents per bulk indexing batch                    | `500`           | `MARMOT_SEARCH_ELASTICSEARCH_BULK_SIZE`        |
| `search.elasticsearch.flush_interval`   | Interval between bulk flushes in milliseconds                  | `1000`          | `MARMOT_SEARCH_ELASTICSEARCH_FLUSH_INTERVAL`   |
| `search.elasticsearch.reindex_on_start` | Run a full reindex from PostgreSQL to Elasticsearch on startup | `false`         | `MARMOT_SEARCH_ELASTICSEARCH_REINDEX_ON_START` |
| `search.elasticsearch.shards`           | Number of primary shards for the index                         | cluster default | `MARMOT_SEARCH_ELASTICSEARCH_SHARDS`           |
| `search.elasticsearch.replicas`         | Number of replicas for the index                               | cluster default | `MARMOT_SEARCH_ELASTICSEARCH_REPLICAS`         |

## TLS

To connect to an Elasticsearch cluster over TLS:

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "https://es.example.com:9200"
    tls:
      ca_cert_path: "/etc/ssl/certs/es-ca.pem"
      cert_path: "/etc/ssl/certs/es-client.pem"
      key_path: "/etc/ssl/private/es-client-key.pem"
```

| Option                                          | Description                                            | Default | Environment Variable                                   |
| ----------------------------------------------- | ------------------------------------------------------ | ------- | ------------------------------------------------------ |
| `search.elasticsearch.tls.ca_cert_path`         | Path to CA certificate for verifying the ES server     | -       | `MARMOT_SEARCH_ELASTICSEARCH_TLS_CA_CERT_PATH`         |
| `search.elasticsearch.tls.cert_path`            | Path to client certificate for mutual TLS              | -       | `MARMOT_SEARCH_ELASTICSEARCH_TLS_CERT_PATH`            |
| `search.elasticsearch.tls.key_path`             | Path to client private key for mutual TLS              | -       | `MARMOT_SEARCH_ELASTICSEARCH_TLS_KEY_PATH`             |
| `search.elasticsearch.tls.insecure_skip_verify` | Skip server certificate verification (not recommended) | `false` | `MARMOT_SEARCH_ELASTICSEARCH_TLS_INSECURE_SKIP_VERIFY` |

## Startup Behaviour

At startup, Marmot checks whether Elasticsearch is reachable. If the cluster is not available, Marmot falls back to PostgreSQL-only search and logs an error. It will not retry connecting to Elasticsearch after startup.

## Shards and Replicas

By default Marmot defers to the Elasticsearch cluster settings for shard and replica counts. You can override these per-index if needed:

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    shards: 3
    replicas: 1
```

These values are only applied when Marmot creates the index for the first time. Changing them after the index already exists has no effect. To apply new shard counts to an existing cluster you must delete the index and let Marmot recreate it, or use the Elasticsearch split/shrink APIs directly.

## Indexing Existing Data

If you enable Elasticsearch on an instance that already has data, set `reindex_on_start: true` to populate the index from the existing `search_index` table:

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    reindex_on_start: true
```

You can also manually trigger a reindex from the admin UI under **Admin > System > Start Reindex**.
