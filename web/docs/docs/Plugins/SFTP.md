---
title: SFTP
description: Discovers directories and files from an SFTP server.
status: experimental
---

# SFTP

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>
</div>
<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2"><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Assets</span><span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">Lineage</span></div>
</div>
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  docId="Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>


The SFTP plugin walks the directories under each configured root and creates a Folder per directory and a File per file, linked by CONTAINS lineage. It logs in with a password or a private key (RSA, Ed25519 or ECDSA).

## Names

An asset is named by its path below the root it was reached from, with no leading slash: `incoming/2026-09` for a folder, `incoming/2026-09/orders.csv` for a file. A root other than `/` contributes nothing to the name, so the same tree served from `/data` or from `/` produces the same assets. Files directly under a root are named by their bare file name.

The root itself is not an asset. The directories directly below each configured root are the top of the tree.

## Columns

`.csv` and `.tsv` files get columns typed INT, FLOAT, BOOLEAN, DATETIME or STRING, inferred from up to `sample_rows` rows. A column is nullable when a sampled row left it empty.

`.json` files holding an array of objects, and `.jsonl` or `.ndjson` files, get the union of their top-level keys plus one level of nesting written `parent.child`. Each column records the JSON types seen and how many sampled records carried the key.

`.parquet` and `.avro` files are catalogued and pass `structured_only`, but this plugin carries no reader for them, so they get no columns.

## Limits

No file is read beyond `max_read_bytes`, and a row count taken from a read that hit the cap is marked `row_count_exact: false`. `max_depth` and `max_files` bound how much of a large server one run covers.

Symlinks are skipped unless `follow_symlinks` is set. A directory that has already been walked is never walked again, so a link pointing back up the tree cannot loop and configured roots nested inside each other are walked once.

## Host key

Set `host_key` to the server's public key, as written in `known_hosts` or `authorized_keys`, to verify the server's identity. When it is empty the plugin accepts whatever key the server presents and logs a warning.

## Example Configuration

```yaml

host: "sftp.company.com"
port: 22
username: "marmot"
private_key: "${SFTP_PRIVATE_KEY}"
root_directories:
  - "/data/incoming"
  - "/data/archive"
max_depth: 5
structured_only: true
tags:
  - "sftp"

```

## Configuration
The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| external_links | []ExternalLink | false | External links to show on all assets |
| filter | Filter | false | Filter discovered assets by name (regex) |
| follow_symlinks | bool | false | Walk into symlinks instead of skipping them |
| host | string | true | SFTP server hostname or IP address |
| host_key | string | false | Server public key to trust, as written in known_hosts. Empty means any key is accepted |
| include_columns | bool | false | Infer columns for delimited and JSON files |
| include_statistics | bool | false | Emit size, row count and column count statistics |
| max_depth | int | false | Levels below each root to walk |
| max_files | int | false | Stop after this many files |
| max_read_bytes | int | false | Most bytes to read from a single file |
| password | string | false | Password for the user |
| port | int | false | SFTP server port |
| private_key | string | false | PEM private key for the user, RSA, Ed25519 or ECDSA |
| private_key_passphrase | string | false | Passphrase protecting the private key |
| root_directories | []string | false | Directories to walk |
| sample_rows | int | false | Rows read from a file to infer its columns |
| structured_only | bool | false | Only catalogue csv, tsv, json, jsonl, parquet and avro files |
| tags | TagsConfig | false | Tags to apply to discovered assets |
| username | string | true | User to log in as |

One of `password` or `private_key` is required.

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|
| column_name | string | Column name: the header cell of a delimited file, or the JSON key with one level of nesting written parent.child |
| data_type | string | Inferred type. Delimited files use INT, FLOAT, BOOLEAN, DATETIME or STRING; JSON files use the JSON types seen, joined by \| when mixed |
| directory | string | Name of the folder holding the file, absent at the top of a root |
| directory_count | int | Subdirectories directly in this directory |
| extension | string | Lowercase file extension without the dot |
| file_count | int | Files directly in this directory |
| file_type | string | File kind: csv, tsv, json, jsonl, parquet, avro or other |
| host | string | Server the file was read from |
| is_nullable | bool | Whether a sampled row left the column empty, null or missing |
| mime_type | string | MIME type guessed from the extension |
| mode | string | Permission bits, as ls prints them |
| modified | string | Last modification time, RFC3339 |
| occurrence | int | Sampled records that carried the JSON key |
| owner_gid | int | Numeric group id of the owner |
| owner_uid | int | Numeric user id of the owner |
| parent | string | Name of the folder above this one, absent at the top of a root |
| path | string | Absolute path on the server |
| port | int | Port the server was reached on |
| root | string | Configured root directory this folder was reached from |
| row_count_exact | bool | Whether the row count covers the whole file or stopped at max_read_bytes |
| size | int | File size in bytes |
| size_bytes | int | Total size of the files directly in this directory |
