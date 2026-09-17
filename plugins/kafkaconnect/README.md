The Kafka Connect plugin discovers connectors, their tasks and the topics they move data through from a Kafka Connect cluster, and links each connector to the tables, collections and buckets it reads or writes.

It talks to the Connect REST API (self-hosted workers and Confluent Cloud) with optional basic auth.

## Assets

| Type | Name | Provider |
|------|------|----------|
| Pipeline | connector name | Kafka Connect |
| Task | `<connector>.task-<id>` | Kafka Connect |
| Topic | topic name | Kafka |

Topics use the identity the Kafka plugin gives them, so a topic both plugins see is one asset with two sources.

## Lineage

- Pipeline CONTAINS Task
- Source connectors: Table FEEDS Pipeline, Pipeline PRODUCES Topic
- Sink connectors: Topic FEEDS Pipeline, Pipeline PRODUCES Table, Collection, Bucket or Container

Table, collection and bucket edges are built from the connector config and point at the asset the owning Marmot plugin creates; this plugin never creates those assets itself. Resolvers exist for Debezium (PostgreSQL, MySQL, SQL Server, MongoDB, Oracle) and the matching Confluent Cloud CDC connectors, the Confluent JDBC source and sink, the S3, GCS and Azure Blob sinks, the Snowflake, BigQuery and Elasticsearch sinks, and the MongoDB source and sink. Other connector classes get topic edges only.

Topics come from the worker's active topics endpoint (KIP-558, Kafka 2.5+) when it has records for the connector, otherwise from the config: `topics`, `topic`, `kafka.topic` and each family's naming rule, with RegexRouter transforms applied.
