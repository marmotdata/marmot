The MySQL plugin discovers databases and tables from MySQL instances. It captures column information, row counts, and foreign key relationships for lineage.

## Required Permissions

The user needs read access to the information schema:

```sql
CREATE USER 'marmot_reader'@'%' IDENTIFIED BY 'your-password';
GRANT SELECT ON your_database.* TO 'marmot_reader'@'%';
GRANT SELECT ON information_schema.* TO 'marmot_reader'@'%';
```

## Querying

This plugin lets Marmot run read-only SQL against a MySQL source for data queries. Every statement runs inside a `START TRANSACTION READ ONLY` transaction that is always rolled back, over a connection whose DSN leaves `multiStatements` unset, so the server accepts at most one statement per call and that statement can neither write nor smuggle a second statement past the transaction. MySQL refuses most writes itself once inside that transaction, and also refuses `FOR UPDATE`, but it does not refuse `SELECT ... INTO OUTFILE` or `INTO DUMPFILE`, which write a file rather than a table, nor `FOR SHARE` or `LOCK IN SHARE MODE`, which still take a lock. The plugin refuses all of these itself before anything reaches the engine, as well as anything that is not a `SELECT`, `WITH`, `TABLE`, `VALUES`, `SHOW` or `DESCRIBE` style read, and any statement that reads or assigns a user variable (`@name`). A user variable lives on the connection's session for as long as it stays open, not just for the transaction that set it, so one caller's `SELECT @leak := secret_col FROM secret` could otherwise still answer a later, unrelated caller's `SELECT @leak` if the two happen to share a pooled connection; reading a system variable (`@@name`) is unaffected, since it carries no caller data. As a second, independent safeguard, the connection pool for data queries never keeps an idle connection open: MySQL has no statement that clears a session's user variables, so each call gets a connection that has either just been opened or was never used by anyone else.

The statement is planned with `EXPLAIN FORMAT=JSON`, read alongside the optimizer trace that planning it leaves behind. The trace, not `EXPLAIN`'s own output, is what the plan is built from: once a statement or a view it reads through gives a table an alias, `EXPLAIN FORMAT=JSON`'s `table_name` field reports that alias instead of the table's name, and an alias chosen to match another real table's name would otherwise make the plan name the wrong table. Each resolved table is mapped to the MRN discovery would give it, so a grant written against a catalogued asset binds on the query path too; a statement that calls a stored function, which can run a query of its own that reads tables the plan never sees, is marked incomplete and falls back to requiring a grant on the whole source, as does a `SHOW` or `DESCRIBE` statement, which `EXPLAIN` cannot plan at all.

None of this is a substitute for database-level permissions: the read-only transaction and the checks above stop a statement from writing or reaching outside the database, but the connecting user's own privileges are the real boundary on what it can read. Use a dedicated account with only `SELECT` for `marmot_reader` rather than relying on Marmot alone.
