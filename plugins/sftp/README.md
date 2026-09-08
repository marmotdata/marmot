The SFTP plugin discovers directories and files from an SFTP server, and infers columns for delimited and JSON files.

It connects with `golang.org/x/crypto/ssh` and `github.com/pkg/sftp`, using a password or a private key (RSA, Ed25519 or ECDSA).

## Names

A Folder or File is named by its path below the configured root, with no leading slash: `incoming/2026-09` and `incoming/2026-09/orders.csv`. A root other than `/` contributes nothing to the name, so the same tree served from `/data` or from `/` produces the same assets.

The root itself is not an asset. The directories directly below each configured root are the top of the tree.

## Columns

`.csv` and `.tsv` files get columns typed INT, FLOAT, BOOLEAN, DATETIME or STRING from up to `sample_rows` rows. `.json` (an array of objects) and `.jsonl` files get the union of their top-level keys plus one level of nesting, written `parent.child`. `.parquet` and `.avro` files are catalogued but get no columns.

No file is read beyond `max_read_bytes`. A row count taken from a truncated read is marked `row_count_exact: false`.

## Limits

`max_depth` and `max_files` bound a run. Symlinks are skipped unless `follow_symlinks` is set, and a directory already walked is never walked again, so a link pointing back up the tree cannot loop.
