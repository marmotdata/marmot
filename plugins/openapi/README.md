The OpenAPI plugin discovers API specifications from OpenAPI v3 files. It creates assets for services and their endpoints.

The plugin scans for `.json` and `.yaml` files and parses them as OpenAPI v3 specifications.

## File Sources

The `spec_path` field accepts local paths, S3 URIs (`s3://bucket/prefix`) or Git URIs (`git::https://...`). For S3 and Git sources, files are downloaded to a temporary directory before discovery and cleaned up afterwards.

See [File Sources](./Shared%20Configuration/File%20Sources.md) for the full list of supported backends, authentication options and configuration examples.
