---
slug: postgres-one-database-to-rule-them-all
title: "Postgres: One Database to Rule Them All"
authors:
  - name: Charlie Haley
    url: https://github.com/charlie-haley
image: /img/postgres-banner.png
---

import { CalloutCard } from '@site/src/components/DocCard';

<div style={{textAlign: 'center', marginBottom: '2rem'}}>
  <img src="/img/postgres-banner.png" alt="Postgres: One Database to Rule Them All" style={{maxWidth: '100%', borderRadius: '8px'}} />
</div>

I'm a huge fan of simple software. There's something really satisfying about solving a problem and reducing the number of moving parts. Most of the time, you'll find the tool you already have can do more than you thought.

<!-- truncate -->

## The infrastructure tax

As a platform engineer, I was genuinely surprised by how much infrastructure existing data catalogs need. Multiple databases, search engines, message queues, workflow orchestration - that's a lot of moving parts before you've even cataloged anything.

"But surely big companies need X? They can justify the complexity, right?"

Maybe they can soak up the extra cost and complexity, or maybe they're already running this infrastructure for other services, but it doesn't necessarily mean it's required.

It's a common trap - reaching for "best practice" tools without questioning if they match your actual scale and requirements. Data catalogs aren't consumer products. They're internal tools used by engineering teams. Even at large organisations, you're looking at hundreds of concurrent users, maybe a couple thousand at most. When the scale ceiling is predictable and modest, why immediately reach for these tools?

Whilst building Marmot, I really wanted to see how far I could push Postgres before needing to reach for dedicated search indexers. Turns out, Postgres has a lot more features than most people realise.

---

## What Postgres can do

### Full-text search

Postgres has had full-text search since version 8.3 (2008). The core abstraction is `tsvector`, a sorted list of normalised words with positional information. You can weight fields into four priority levels (A highest, D lowest) so for example, matches in a name rank higher than matches in a description:

```sql
search_text tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(mrn, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(type, '')), 'B') ||
    setweight(to_tsvector('english', array_to_string(providers, ' ')), 'B') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'C')
) STORED

```

A GIN index on this column lets you query it efficiently. The `websearch_to_tsquery` function handles query parsing with phrases, boolean operators and prefix matching. Ranking uses `ts_rank_cd` which considers density and weight.

### Fuzzy matching

Users make typos, also, data assets traditionally don't have very friendly names and will likely include underscores and special characters.

The `pg_trgm` extension handles this with trigram similarity. A trigram is three consecutive characters - the PostgreSQL docs note that "a string is considered to have two spaces prefixed and one space suffixed when determining the set of trigrams." So "foobar" becomes `{"  f"," fo","foo","oo "," b"," ba","bar","ar "}`. Comparing trigram sets gives you fuzzy matching that handles misspellings and partial matches.

### GIN vs GiST

Postgres offers two index types for trigram operations: GiST and GIN. I chose GIN for Marmot after load testing revealed significant performance differences under concurrent load.

[The PostgreSQL documentation recommends GIN as the preferred text search index type.](https://www.postgresql.org/docs/current/textsearch-indexes.html)

I load tested both approaches under Marmot's read-heavy workload. GIN indexes, which store trigrams in an inverted index structure, showed roughly 3x faster lookups than GiST's balanced tree structure. While GiST indexes are faster for writes and smaller on disk, they were significantly less performant under heavy concurrent read load.

For Marmot at least, GIN was the clear winner. The trade-off is slightly slower index updates, which is acceptable since search index updates happen via triggers on entity changes instead of refreshing the index periodically.

### Graph traversal

Data lineage is a graph problem.You need to traverse these assets relationships in both directions.

Postgres handles this with recursive CTEs:

```sql
WITH RECURSIVE upstream AS (
    SELECT source_mrn as mrn, -1 as depth
    FROM lineage_edges
    WHERE target_mrn = $1

    UNION ALL

    SELECT e.source_mrn, u.depth - 1
    FROM lineage_edges e
    JOIN upstream u ON e.target_mrn = u.mrn
    WHERE u.depth > -$2
) CYCLE mrn SET is_cycle USING path
SELECT DISTINCT mrn, depth FROM upstream WHERE NOT is_cycle

```

The `RECURSIVE` keyword tells Postgres to iteratively expand the result set - starting with direct dependencies, then repeatedly joining to find dependencies of dependencies. With data pipelines, it can be common for lineage trees to eventually loop around so the `CYCLE` clause detects and marks them automatically.

For typical catalog queries (a few dozen assets, 5-10 levels deep), this works well.
When rendering massive lineage trees - 250+ assets with depth of 10+ - performance degrades noticeably. The solution for Marmot was to restrict depth in the UI for very large graphs.

---

## How Marmot uses these features

### Search strategy

Full-text search and trigram similarity solve different problems. Full-text understands language - stemming, phrases, boolean logic. Trigrams handle typos and partial matches without caring about word boundaries.

Marmot uses trigram similarity for its primary search. Why? Data asset names are messy. They're underscore_delimited, dot.separated, or CamelCased. Full-text search handles underscores fine, but struggles with dot.separated names, CamelCase, and abbreviations. Trigrams don't care about delimiters or word boundaries, they just compare character sequences.

For structured queries with filters (type, provider, tags), Marmot combines trigram matching with standard SQL predicates. The query planner handles it efficiently in a single round trip.

### Keeping search in sync

Marmot maintains a unified search index table that consolidates assets, glossary terms, teams and data products. Each entity type has its own trigger that keeps the index in sync:

```sql
CREATE TRIGGER search_index_asset_sync
    AFTER INSERT OR UPDATE OR DELETE ON assets
    FOR EACH ROW EXECUTE FUNCTION search_index_asset_trigger();
```

When an asset changes, the trigger upserts into the search index in the same transaction. The index is always exactly in sync because updates are atomic.

[Modern Postgres recommends `GENERATED ALWAYS AS` columns for maintaining tsvector indexes](https://www.postgresql.org/docs/current/textsearch-tables.html#TEXTSEARCH-TABLES-INDEX) - it's simpler and more efficient than triggers. But generated columns only work within a single table; they can't write to separate tables. Since Marmot consolidates multiple entity types into one unified search table, triggers worked well for this use-case. This gives us a denormalised search structure optimized for queries while keeping source tables normalized for writes.

For expensive aggregations like facet counts, Marmot maintains a counter cache table that tracks counts by dimension (entity type, asset type, provider, tag). The same triggers that update the search index also update these counters.

The downside is that writes become slightly slower since they wait for triggers to complete. For a data catalog with bursty workloads - bulk imports overnight, occasional metadata updates during the day - I found this trade-off is acceptable.

---

## Trade-offs

### What you give up

Postgres full-text search isn't Elasticsearch. There are things you give up:

- **Scale** - Elasticsearch is built for billions of documents; Postgres full-text search is a bit more modest, however in my testing, it seemed to handle a million assets surprisingly well.
- **Simpler field boosting** - four weight levels (A/B/C/D) vs Elasticsearch's numeric boost factors
- **No language detection** - you pick your dictionary upfront

These are real limitations. Whether they matter depends on your use-case. A data catalog indexes internal metadata - tens of thousands of assets, maybe up to a million for some use-cases, not billions. Users search for table names and column descriptions, not multilingual documents. For this workload, I've found Postgres full-text search to be more than enough.

### Operational simplicity

Running Elasticsearch, Neo4j and Kafka means monitoring, tuning, upgrades and capacity planning. That's infrastructure cost and engineering time. Most teams building a data catalog don't have a platform team to run all this.

Is Postgres the best tool for search? No, Elasticsearch is better. For graphs? Neo4j wins. But Postgres is good enough for both, and you're already running it. Sometimes the right architecture isn't the one with the best components - it's the one you can actually maintain.

---

## Does it actually work?

This all sounds great in theory, but does it actually work beyond 1 user in my local dev environment?

### Test environment

I used a dedicated Kubernetes cluster on Hetzner Cloud:

- **Cluster**: 7 ARM64 nodes (1 control plane + 6 workers), 4 vCPU and 8GB RAM each
- **PostgreSQL**: CloudNativePG with 3 instances (1 primary + 2 read replicas), PgBouncer connection pooling
- **Marmot**: 4 replicas spread across workers
- **Load generator**: k6 on a dedicated node

The database was seeded with 500,000 assets - tables, topics, dashboards, pipelines - with realistic metadata, tags and lineage relationships.

k6 simulated 100 concurrent users for 15 minutes, cycling through various patterns including search queries of varying scope and complexity, averaging around 85 requests/second across all endpoints.

### Results

```
─────────────────────────────────────────────────────────────────────
OVERALL HTTP PERFORMANCE
─────────────────────────────────────────────────────────────────────
All Requests              avg=43.19ms   p95=205.32ms  p99=481.21ms

─────────────────────────────────────────────────────────────────────
ENDPOINT PERFORMANCE
─────────────────────────────────────────────────────────────────────
Asset Get                 avg=7.90ms    p95=19.24ms   p99=33.66ms
Asset Summary             avg=5.96ms    p95=15.17ms   p99=27.36ms
Search (Plain)            avg=150.99ms  p95=452.54ms  p99=886.54ms
Search (Structured)       avg=16.90ms   p95=48.96ms   p99=101.69ms
Search (Empty/Browse)     avg=9.50ms    p95=20.59ms   p99=35.61ms
Lineage                   avg=20.17ms   p95=43.69ms   p99=61.72ms
Metrics Overview          avg=11.80ms   p95=29.26ms   p99=44.65ms
Tag Suggestions           avg=29.24ms   p95=151.49ms  p99=305.55ms
Field Suggestions         avg=14.04ms   p95=65.59ms   p99=156.24ms
```

Most operations respond in under 20ms. Plain text search is the slowest at ~150ms average - that's the cost of trigram fuzzy matching with 500k assets - but it still feels fast to users.

The point isn't that Postgres scales infinitely. I'm sure I could break it with enough load. But for actual data catalog workloads - internal tools used by engineering teams - it handles 100 concurrent users quite easily and I'm sure there's more tuning I could make and more read replicas I could add to keep scaling out!

I used CloudNativePG and it made the load test setup really straightforward. These tests ran on modest ARM64 nodes - I'd be interested to see how Marmot performs on managed Postgres services with more resources, if you want to sponsor load testing with some cloud credits, please reach out!

---

## Conclusion

Postgres is an awesome tool with a lot of very cool features, [OpenAI recently shared how they run ChatGPT's backend on a single Postgres primary with read replicas](https://openai.com/index/scaling-postgresql/) - serving 800 million users.

That said, this approach won't solve all your problems. If you need geo-spatial search, real-time analytics or advanced graph algorithms - specialised tools make sense. The operational complexity pays for itself at that scale.

The real bottleneck in data catalogs was never database throughput anyway. It's getting people to actually document their data and keep it up to date!
