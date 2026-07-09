# marmot-plugin-gcs

Marmot plugin for Google Cloud Storage. Discovers buckets and registers them as assets in your Marmot catalog.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. It is built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk).

## Install

Nothing to install: gcs is a core Marmot plugin. Marmot pulls the release pinned in its core plugin manifest from `ghcr.io/marmotdata/plugins/gcs` at startup and caches it under `~/.marmot/plugins/cache`.

## Configuration

```yaml
project_id: "my-gcp-project"
credentials_file: "/path/to/service-account.json"
include_metadata: true
include_object_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "gcs"
  - "storage"
```

| Field | Description |
| --- | --- |
| `project_id` | Google Cloud project ID (required) |
| `credentials_file` | Path to a service account JSON file |
| `credentials_json` | Service account JSON content (sensitive) |
| `endpoint` | Custom endpoint URL, for fake-gcs-server or other emulators |
| `disable_auth` | Disable authentication, for local emulators |
| `include_metadata` | Include bucket metadata like labels (default `true`) |
| `include_object_count` | Count objects in each bucket; can be slow for large buckets (default `false`) |

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
