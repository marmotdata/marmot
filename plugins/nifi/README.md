The NiFi plugin discovers process groups and processors from Apache NiFi through its REST API. Each process group becomes a Pipeline, each processor a Task, and the connections between them become lineage. Well-known processors are also linked to the S3 and GCS buckets, Azure Blob containers, Kafka topics, database tables and Elasticsearch indexes they read or write.

## Authentication

Single-user and LDAP installs take `username` and `password`; the plugin exchanges them for a bearer token. An existing bearer token goes in `token`. Installs secured with client certificates take `client_cert` and `client_key`. NiFi ships with a self-signed certificate, so `verify_ssl: false` is needed unless the certificate is trusted or `ca_cert` points at its CA.

The user needs `view the user interface` and read access to the process groups to discover, plus read access to controller services for table lineage through connection pools.

## Run history

NiFi records provenance events, not runs, so the plugin emits no run history.
