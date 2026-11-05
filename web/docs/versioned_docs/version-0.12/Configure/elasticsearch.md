---
title: Search backend
sidebar_label: Search backend
description: Use Elasticsearch for fuzzy search across every field of the catalog.
---

# Search backend

Marmot searches with PostgreSQL out of the box. Add Elasticsearch for fuzzy matching across every field of a record, including metadata, descriptions, documentation and schemas.

## Connect Elasticsearch

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    index: "marmot"
```

Or with environment variables:

```bash
MARMOT_SEARCH_ELASTICSEARCH_ENABLED=true
MARMOT_SEARCH_ELASTICSEARCH_ADDRESSES=http://localhost:9200
MARMOT_SEARCH_ELASTICSEARCH_INDEX=marmot
```

## Options

Each key has an environment variable: `MARMOT_` followed by the key in capitals with dots as underscores, such as `MARMOT_SEARCH_ELASTICSEARCH_ADDRESSES`.

| Key                                     | Description                                                    | Default         |
| --------------------------------------- | -------------------------------------------------------------- | --------------- |
| `search.elasticsearch.enabled`          | Enable Elasticsearch for text search                           | `false`         |
| `search.elasticsearch.addresses`        | List of Elasticsearch node URLs                                | -               |
| `search.elasticsearch.username`         | HTTP Basic Auth username                                       | -               |
| `search.elasticsearch.password`         | HTTP Basic Auth password                                       | -               |
| `search.elasticsearch.index`            | Name of the Elasticsearch index                                | `marmot`        |
| `search.elasticsearch.bulk_size`        | Number of documents per bulk indexing batch                    | `500`           |
| `search.elasticsearch.flush_interval`   | Interval between bulk flushes in milliseconds                  | `1000`          |
| `search.elasticsearch.reindex_on_start` | Run a full reindex from PostgreSQL to Elasticsearch on startup | `false`         |
| `search.elasticsearch.shards`           | Number of primary shards for the index                         | cluster default |
| `search.elasticsearch.replicas`         | Number of replicas for the index                               | cluster default |

## TLS

Connect to a cluster over TLS with a CA certificate, and optionally a client certificate:

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

| Key                                             | Description                                            | Default |
| ----------------------------------------------- | ------------------------------------------------------ | ------- |
| `search.elasticsearch.tls.ca_cert_path`         | Path to CA certificate for verifying the ES server     | -       |
| `search.elasticsearch.tls.cert_path`            | Path to client certificate for mutual TLS              | -       |
| `search.elasticsearch.tls.key_path`             | Path to client private key for mutual TLS              | -       |
| `search.elasticsearch.tls.insecure_skip_verify` | Skip server certificate verification; development only | `false` |

## If Elasticsearch is down at startup

Marmot checks Elasticsearch once, when it starts. If the cluster isn't reachable, it logs an error and searches with PostgreSQL only until you restart it.

## Shards and replicas

Marmot uses the cluster's shard and replica counts unless you set them for its index:

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    shards: 3
    replicas: 1
```

They only apply when Marmot creates the index. To change them later, delete the index and let Marmot recreate it, or use the Elasticsearch split and shrink APIs.

## Index existing data

When you add Elasticsearch to an instance that already has data, set `reindex_on_start` to fill the index when Marmot starts:

```yaml
search:
  elasticsearch:
    enabled: true
    addresses:
      - "http://localhost:9200"
    reindex_on_start: true
```

Or open **Settings → System** and, under **Search index**, select **Reindex search**.

## Next steps

- [Search and the query language](../queries.md): what people can search for.
- [Configuration reference](index.md): every other setting.
