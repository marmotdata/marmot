The Kafka Connect plugin discovers connectors, their tasks and the topics they move data through from a Kafka Connect cluster, and links each connector to the tables, collections and buckets it reads or writes.

It talks to the Connect REST API (self-hosted workers and Confluent Cloud) with optional basic auth.

## Assets

| Type | Name | Provider |
|------|------|----------|
| Pipeline | connector name | Kafka Connect |
| Task | `task-<id>`, MRN `<connector>.task-<id>` | Kafka Connect |
| Topic | topic name | Kafka |

Topics use the identity the Kafka plugin gives them, so a topic both plugins see is one asset with two sources.

## Lineage

- Pipeline CONTAINS Task
- Source connectors: Table FEEDS Pipeline, Pipeline PRODUCES Topic
- Sink connectors: Topic FEEDS Pipeline, Pipeline PRODUCES Table, Collection, Bucket or Container

Table, collection and bucket edges are built from the connector config and point at the asset the owning Marmot plugin creates; this plugin never creates those assets itself. Resolvers exist for Debezium (PostgreSQL, MySQL, SQL Server, MongoDB, Oracle) and the matching Confluent Cloud CDC connectors, the Confluent JDBC source and sink, the S3, GCS and Azure Blob sinks, the Snowflake, BigQuery and Elasticsearch sinks, and the MongoDB source and sink. Other connector classes get topic edges only.

Topics come from the worker's active topics endpoint (KIP-558, Kafka 2.5+) when it has records for the connector, otherwise from the config: `topics`, `topic`, `kafka.topic` and each family's naming rule, with RegexRouter transforms applied.

## Zones

Set `zone` when you catalogue more than one Connect cluster, which puts it in the MRN of every connector and task.

The tables, collections, buckets and topics a connector references belong to other plugins, so they are named the way those plugins name them (for example `<database>.<schema>.<table>` for PostgreSQL). Each reference carries the host and port from the connector config (`connection.url`, `database.hostname` and `database.port`, `connection.uri`) as a hint, and Marmot picks the zone that owns that address. Topics use `bootstrap_servers`, or a connector's own `bootstrap.servers` override. When the address is not enough, `connector_map` names the zone outright, as connector name, then provider, then zone:

```yaml
connector_map:
  orders-sink:
    PostgreSQL: warehouse-eu
    Kafka: prod
```
