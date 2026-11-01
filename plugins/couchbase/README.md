The Couchbase plugin discovers buckets, scopes and collections from Couchbase Server and Capella clusters. It infers a document schema by sampling each collection, records query indexes and collects document counts and bucket sizes.

## Required Permissions

Discovery needs a user that can list buckets, scopes and collections, run read-only N1QL queries (`SELECT`, `INFER`, `system:indexes`) and read bucket statistics from the management API on port 8091 (18091 for `couchbases://`).

On Community Edition, `bucket_full_access` on the buckets to catalog is enough:

```
couchbase-cli user-manage -c localhost -u Administrator -p password --set \
  --rbac-username marmot_reader --rbac-password your-password \
  --roles 'bucket_full_access[*]' --auth-domain local
```

On Enterprise Edition and Capella, grant `query_select` on those buckets, `query_system_catalog` for the index listing, and a role that can read bucket settings and statistics, such as `ro_admin`.

## Indexes and Sampling

Sampling reads documents through the query service, which needs a primary index or, on Couchbase Server 7.6 and later, a sequential scan. Collections without one fall back to `INFER`, which samples through the data service and needs no index. Document counts (`SELECT COUNT(*)`) are skipped when no index can answer them; on a bucket with a single collection the bucket's item count stands in. Data previews always need a primary index on the collection.
