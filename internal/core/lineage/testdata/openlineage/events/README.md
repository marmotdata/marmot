# Captured OpenLineage events

Events real producers sent, kept exactly as they arrived so the tests run against what producers actually send. `TestReplay*` sends each folder in file order and compares the catalog with lineage written out by hand, and `TestEveryProducerFixtureIsValidOpenLineage` validates every file against the schemas in `../spec`.

| Folder | Producer |
| --- | --- |
| `duck_lineage` | duck_lineage 0.2.2 on DuckDB 1.5.6 |
| `dbt` | openlineage-dbt 1.55.0 with dbt-core 1.12.7 and dbt-duckdb 1.11.1 |
| `spark` | openlineage-spark 1.55.0 on Spark 4.0.5 |

To capture them again, run the producer's `run.sh` in `test/integration/openlineage` with `OPENLINEAGE_URL` pointing at a listener that writes each request body to a file. The duck_lineage events had the scratch directory path replaced with `/data`; nothing else was changed.

## Where producers break the spec

These look odd in the files on purpose. Marmot handles each one, and the tests prove it:

- **openlineage-spark**: `dataSource.uri` is the bare word `file`, which isn't an absolute URI, so the schema test repairs only that field before validating. A failed job sends FAIL and then COMPLETE for one run. Schema-inference jobs send START and nothing after. A `CREATE TABLE AS SELECT` arrives under two job names, and the `DROP TABLE` before it names a path with `shop.db` twice.
- **duck_lineage**: a failed statement is reported as COMPLETE, never FAIL. Tables in attached databases are named under its own `duckdb` namespace, with the real address only in the `dataSource` and `catalog` facets. Output columns it can't name arrive as `#[4.0]`. It sends `outputStatistics.sizeInBytes` instead of `size`, the deprecated `transformationType` in column lineage, and a view's inputs leave out tables read in a subquery.
- **dbt-ol**: seeds are missing from a model's inputs and appear only in its column lineage, a failed model carries no `errorMessage`, and the schema facet lists only documented columns while column lineage lists them all.
