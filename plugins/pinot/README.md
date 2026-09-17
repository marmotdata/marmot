The Pinot plugin discovers tables from Apache Pinot clusters through the controller REST API. It captures the schema (dimensions, metrics and date-time fields), table settings, segment counts, row counts and sizes, and links realtime tables to the Kafka topic or Kinesis stream they consume.

## Connection

`controller_url` is required. Row counts and sample data run as SQL; they go through the controller's `/sql` proxy unless `broker_url` points at a broker directly. Basic auth (`username`, `password`) and a bearer `token` are both supported when the cluster has authentication enabled. Set `database` to discover one logical database on Pinot 1.1 or later.

## Required Permissions

A read-only user with access to the table, schema, segment and query endpoints of the controller is enough:

```
GET  /health
GET  /version
GET  /tables
GET  /tables/{name}
GET  /tables/{name}/schema
GET  /tables/{name}/size
GET  /segments/{name}
GET  /schemas/{name}
POST /sql
```
