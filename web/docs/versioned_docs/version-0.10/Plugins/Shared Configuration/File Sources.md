---
title: File Sources
description: How file-based plugins load files from local paths, S3 or Git.
sidebar_position: 1
---

# File Sources

Some plugins such as [DBT](../DBT.md), [DuckDB](../DuckDB.md) and [OpenAPI](../OpenAPI.md) read their input from files. Wherever a plugin exposes a path field (`path`, `target_path`, `spec_path`) it can point at one of three backends:

| Backend | Description |
|---------|-------------|
| `local` | A path on the machine running Marmot. |
| `s3`    | An object or prefix in S3. Downloaded to a temp directory before discovery. |
| `git`   | A path inside a Git repo. Shallow-cloned to a temp directory before discovery. |

The backend is auto-detected from the path prefix, or you can set it explicitly with `source_type`. Temporary files are cleaned up after the plugin runs.

## Local

The default. Any path that is not an `s3://` or `git::` URI is treated as a local filesystem path.

```yaml
path: "/data/analytics.duckdb"
```

## S3

Use `s3://bucket/key` for a single file or `s3://bucket/prefix/` for a directory.

```yaml
path: "s3://my-bucket/databases/analytics.duckdb"
s3_source:
  credentials:
    region: "us-east-1"
    use_default: true
```

Set `source_type: "s3"` explicitly if your path does not start with `s3://`:

```yaml
source_type: "s3"
spec_path: "openapi/"
s3_source:
  bucket: "api-specs"
  prefix: "openapi/"
  credentials:
    region: "us-east-1"
    use_default: true
```

`s3_source.credentials` accepts the same fields as other AWS-backed plugins. See [AWS Configuration](<./AWS Configuration.md>) for the full list.

## Git

Use `git::<repo-url>`, optionally with a subpath (`//subdir`) and a `?ref=<branch-or-tag>` query parameter.

```yaml
path: "git::https://github.com/org/repo//data/analytics.duckdb?ref=main"
```

Or configure it explicitly:

```yaml
source_type: "git"
target_path: "target"
git_source:
  url: "https://github.com/org/dbt-project"
  ref: "main"
  path: "target"
  token: "ghp_xxxx"
```

Authentication options:

| Field          | Description |
|----------------|-------------|
| `token`        | Personal access token for HTTPS auth. |
| `ssh_key_path` | Path to an SSH private key for SSH auth. |

Public repos can be cloned anonymously. `ref` defaults to `main` and may be a branch or a tag.
