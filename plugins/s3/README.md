# marmot-plugin-s3

Marmot plugin for [Amazon S3](https://aws.amazon.com/s3/). Lists the buckets in an account and produces a `Bucket` asset per bucket with region, creation date, and the state of its configuration: versioning, encryption, public access block, notifications, lifecycle, replication, website hosting, logging, transfer acceleration, and request payment. Bucket tags can optionally be converted to asset metadata.

Authentication uses the standard AWS credential chain: static keys, a shared profile, an assumed role, or the environment defaults. A custom `endpoint` (with path-style addressing) supports MinIO and other S3-compatible stores.

## Example Configuration

```yaml
credentials:
  region: "us-east-1"
  profile: "production"
  role: "arn:aws:iam::123456789012:role/MarmotDiscovery"
tags:
  - "s3"
tags_to_metadata: true
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
