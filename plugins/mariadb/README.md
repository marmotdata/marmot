The MariaDB plugin discovers a database and its tables, views and sequences from a MariaDB server. It captures column information (including invisible and generated columns), row and size statistics, foreign key relationships and view lineage (a view is fed by the tables it reads: base table -> view).

## Required Permissions

The user needs read access to the database and the information schema. `SHOW VIEW` is needed to read view definitions:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT, SHOW VIEW ON your_database.* TO 'marmot_reader'@'%';
```

## Querying

This plugin lets Marmot run read-only data queries against the configured server. A statement is checked before it ever reaches MariaDB: anything other than a plain `SELECT`, `WITH` or `VALUES` is refused, and so is `FOR UPDATE`, `LOCK IN SHARE MODE`, `SELECT ... INTO OUTFILE`/`DUMPFILE` and any reference to a user variable (`@name`), none of which a read-only transaction actually stops on MariaDB (DDL commits on its own, writing a file is not a change to a table, and a user variable survives a rolled-back transaction on the connection that set it). Marmot then runs the statement inside a `START TRANSACTION READ ONLY` transaction that is always rolled back, as a second line of defence, and the connection never enables multiple statements, so a second statement cannot ride in on the back of the first.

Every query also opens its own fresh connection rather than reusing one left over from an earlier query: MariaDB has no lightweight way to reset a connection's session state, so reusing one would risk a later, unrelated caller reading back something an earlier caller's statement left on it, such as a user variable. The cost is a fresh TCP handshake and authentication per query rather than a warm connection, which is worth paying since data queries run far less often than a discovery crawl.

The database user's own privileges are the real boundary: queries run as the same account discovery uses, so a user who should not be able to query everything the account can see should get a separate, more restricted account for Marmot to query with. The account only needs `SELECT` on the tables it should be able to query; add `SHOW VIEW` too if queries may read through a view.

By default, data queries run against the `database` field. Set `query_database` to point data queries at a different database on the same server; this does not change what Discover catalogues.

A statement is planned by reading back MariaDB's own rewritten form of it (`EXPLAIN EXTENDED` plus `SHOW WARNINGS`), which fully qualifies every table it reads and already accounts for aliases, index hints, `PARTITION` clauses and old-style comma joins the statement's own text used, so the plan can never attribute what it reads to the wrong table. A view the optimiser can merge into the query is planned against its base tables; one that forces a materialisation step (typically because it aggregates) is planned against the view itself, since that is what MariaDB's own rewrite names. A statement that calls a stored function is always planned as needing access to the whole source, since MariaDB's query plan never shows what a function itself reads. Whenever a table a statement reads cannot be resolved against what Discover catalogued with certainty, the plan is marked incomplete and access to the whole source is required, rather than guessing.
