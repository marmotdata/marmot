#!/usr/bin/env bash
# Runs a DuckDB scenario with the duck_lineage community extension so its OpenLineage events go to $OPENLINEAGE_URL. It attaches the source Postgres database as pg, copies a table to parquet (issue 377), builds tables with CTAS, joins, INSERT ... SELECT, reads CSV and parquet, renames and drops a table, creates a view and runs a statement that fails.
#
# Env:
#   OPENLINEAGE_URL       required, base URL; /api/v1/lineage is appended
#   OPENLINEAGE_API_KEY   optional, sent as a bearer token
#   SOURCE_PG             required, libpq attach string for the source database
#   DUCKDB_VERSION        optional, CLI version to download (default 1.5.6)
set -euo pipefail

cd "$(dirname "$0")"

: "${OPENLINEAGE_URL:?OPENLINEAGE_URL must be set}"
: "${SOURCE_PG:?SOURCE_PG must be set}"
version="${DUCKDB_VERSION:-1.5.6}"

if [[ ! -x .cache/duckdb-$version ]]; then
  mkdir -p .cache
  curl -sSfL -o .cache/duckdb.zip "https://github.com/duckdb/duckdb/releases/download/v$version/duckdb_cli-linux-amd64.zip"
  unzip -o -q .cache/duckdb.zip -d .cache
  mv .cache/duckdb ".cache/duckdb-$version"
  rm .cache/duckdb.zip
fi
duckdb="$PWD/.cache/duckdb-$version"

rm -rf work
mkdir work
cp regions.csv work/
cd work

# Statements stay byte-identical across runs: duck_lineage names each job after a hash of its SQL.
"$duckdb" shopdb.duckdb <<SQL
INSTALL duck_lineage FROM community;
LOAD duck_lineage;
INSTALL postgres;
LOAD postgres;
SET duck_lineage_url='${OPENLINEAGE_URL%/}/api/v1/lineage';
SET duck_lineage_api_key='${OPENLINEAGE_API_KEY:-}';
ATTACH '$SOURCE_PG' AS pg (TYPE postgres, READ_ONLY);
COPY (SELECT * FROM pg.public.orders) TO 'out.parquet';
CREATE TABLE main.orders AS SELECT * FROM pg.public.orders;
CREATE TABLE main.customers AS SELECT * FROM pg.public.customers;
CREATE TABLE main.regions AS SELECT * FROM read_csv('regions.csv');
CREATE TABLE main.revenue_by_region AS SELECT c.region, r.label, sum(o.total) AS revenue, count(*) AS orders FROM main.orders o JOIN main.customers c ON o.customer_id = c.id JOIN main.regions r ON r.region = c.region GROUP BY ALL;
INSERT INTO main.orders SELECT * FROM read_parquet('out.parquet') WHERE id > 3;
CREATE TABLE main.top_regions AS SELECT region, revenue FROM main.revenue_by_region WHERE revenue > 10;
ALTER TABLE main.top_regions RENAME TO best_regions;
CREATE TABLE main.scratch AS SELECT 1 AS x;
DROP TABLE main.scratch;
CREATE VIEW main.eu_orders AS SELECT * FROM main.orders WHERE customer_id IN (SELECT id FROM main.customers WHERE region = 'eu');
CREATE TABLE main.uniq (id INTEGER PRIMARY KEY);
SQL

# A statement that fails, in a session of its own so the error does not stop the scenario above.
if "$duckdb" shopdb.duckdb <<SQL 2>err.txt
LOAD duck_lineage;
SET duck_lineage_url='${OPENLINEAGE_URL%/}/api/v1/lineage';
SET duck_lineage_api_key='${OPENLINEAGE_API_KEY:-}';
INSERT INTO main.uniq SELECT id FROM main.orders UNION ALL SELECT id FROM main.orders;
SQL
then
  echo "expected the duplicate insert to fail" >&2
  exit 1
fi
grep -q "constraint violation" err.txt

# duck_lineage sends from a background queue; give it a moment to drain before the next producer runs.
sleep 2
echo "database: $PWD/shopdb.duckdb"
