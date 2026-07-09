# marmot-plugin-openapi

Marmot plugin for [OpenAPI](https://www.openapis.org/) v3 specifications. Walks a directory of spec files (JSON or YAML) and produces:

- **Services** as `Service` assets from each spec's `info` section (version, servers, contact, license, endpoint counts, external docs).
- **Endpoints** as `Endpoint` assets per operation, with HTTP method, path, operation ID, status codes, deprecation state, and response schemas converted to JSON Schema. Endpoints are parented to their service and tagged with the service name and operation tags.

Specs older than OpenAPI v3 are skipped.

## Example Configurations

### Local spec directory

```yaml
spec_path: "/app/openapi-specs"
tags:
  - "openapi"
  - "specifications"
```

### S3-hosted specs

```yaml
spec_path: "s3://my-specs-bucket/openapi/"
s3_source:
  credentials:
    region: "eu-west-2"
    use_default: true
```

### Git repository

```yaml
spec_path: "git::https://github.com/example/api-specs//openapi?ref=main"
git_source:
  token: "${GITHUB_TOKEN}"
```

## Development

Build and test:

```sh
make build
make test
```

To run a local build inside Marmot:

```sh
make install
```

This copies the binary to `~/.marmot/plugins/`, the directory Marmot scans for local plugins. A local plugin shadows the released core plugin with the same name: Marmot skips downloading it and loads your build instead. Delete the binary from `~/.marmot/plugins/` to fall back to the released version.

If your Marmot runs with a custom plugins directory (`MARMOT_PLUGINS_DIR`), set the same value for `make install` so both point at the same place.
