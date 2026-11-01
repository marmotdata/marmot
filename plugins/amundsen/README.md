The Amundsen plugin imports the contents of an Amundsen metadata graph over Bolt, reading Neo4j directly.

Amundsen is a catalog, so every entry in it describes something that lives somewhere else. Each table is projected onto the Marmot provider and MRN its own technology's plugin uses, so an Amundsen import and a later native run land on one asset instead of two. `amundsen/projection.go` holds that mapping; it agrees with the OpenMetadata plugin's projection for the technologies they share.

## What it reads

Tables and views, their columns, owners, tags, badges and programmatic descriptions; dashboards and their charts; read counts as statistics; table to table lineage from `HAS_UPSTREAM`, and dashboard to table lineage from `DASHBOARD_WITH_TABLE`.

No database, cluster or schema assets are created. Those belong to the technology's own plugin.

## Naming

The Amundsen hierarchy is `Database -> Cluster -> Schema -> Table`, where `Database` holds the technology name. The cluster enters an asset's name only where the technology's Marmot name has room for it, such as the leading part of a Snowflake `database.schema.table`. Everywhere else it is an environment label and is recorded in metadata.

## Tests

Unit tests run without Neo4j. The end to end tests need a graph:

    docker run -d --name marmot-test-amundsen -p 17687:7687 \
      -e NEO4J_AUTH=neo4j/marmotpass neo4j:5-community
    docker cp testdata/seed.cypher marmot-test-amundsen:/tmp/seed.cypher
    docker exec marmot-test-amundsen cypher-shell -u neo4j -p marmotpass -f /tmp/seed.cypher

    MARMOT_TEST_AMUNDSEN_URI=bolt://localhost:17687 \
    MARMOT_TEST_AMUNDSEN_USER=neo4j \
    MARMOT_TEST_AMUNDSEN_PASSWORD=marmotpass \
    go test ./...
