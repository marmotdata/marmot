# marmot-plugin-azureblob

Marmot plugin for [Azure Blob Storage](https://azure.microsoft.com/en-us/products/storage/blobs). Lists the containers in a storage account and produces a `Container` asset per container with its properties (lease status, public access level, immutability policy, legal hold), custom container metadata, and optionally a blob count.

Authentication is either a connection string or an account name plus account key; a custom endpoint supports Azurite and other emulators.

## Example Configurations

### Connection string

```yaml
connection_string: "${AZURE_STORAGE_CONNECTION_STRING}"
include_metadata: true
include_blob_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "azure"
  - "storage"
```

### Account name and key

```yaml
account_name: "mystorageaccount"
account_key: "${AZURE_STORAGE_ACCOUNT_KEY}"
include_metadata: true
```

### Azurite emulator

```yaml
account_name: "devstoreaccount1"
account_key: "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
endpoint: "http://localhost:10000/devstoreaccount1"
```

Counting blobs (`include_blob_count: true`) walks every blob in each container and can be slow for large containers; it is off by default.

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
