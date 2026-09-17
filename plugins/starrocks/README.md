The StarRocks plugin discovers databases, tables, views and asynchronous materialized views from StarRocks clusters.

StarRocks speaks the MySQL wire protocol, so the plugin connects to a frontend (FE) node on the query port with the `go-sql-driver/mysql` driver. It records the details a StarRocks table has and a plain SQL table does not: the key model, the aggregate function of each value column, partitioning, bucketing and the sort key.

## Required Permissions

The user needs read access to the databases you want in the catalog:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT ON shop.* TO 'marmot_reader'@'%';
```

`SHOW CATALOGS`, `SHOW DATABASES` and `SHOW CREATE TABLE` only return objects the user can read, so a reader with no grants discovers nothing rather than failing.

## Catalogs

Discovery runs against one catalog per configuration block. `catalog` defaults to `default_catalog`, the cluster's own storage. Naming an external catalog runs `SET CATALOG` first and discovers the tables it exposes; add a second run to cover a second catalog.

## Row Counts

Row counts and sizes come from `information_schema.tables`, which StarRocks fills in from backend tablet reports. A cluster that has not reported yet, and the `starrocks/allin1-ubuntu` image in particular, returns 0 for both. Set `include_statistics: false` to leave them out.

## Testing

Unit tests need nothing installed:

```sh
go test ./...
```

The end to end tests run against a real cluster and are skipped unless `MARMOT_TEST_STARROCKS_HOST` is set. They read a `shop` database holding the four key models, a view and a materialized view, which `starrocks/testdata/seed.sql` creates:

```sh
docker run -d --name starrocks --memory 4g -p 9030:9030 -p 8030:8030 starrocks/allin1-ubuntu:latest
# wait until SHOW BACKENDS reports Alive: true, about one to three minutes
mysql -h127.0.0.1 -P9030 -uroot < starrocks/testdata/seed.sql
MARMOT_TEST_STARROCKS_HOST=localhost MARMOT_TEST_STARROCKS_USER=root go test ./starrocks -run TestE2E
```
