# marmot-plugin-sns

Marmot plugin for [Amazon SNS](https://aws.amazon.com/sns/). Lists the topics in an account and produces a `Topic` asset per topic with its ARN, owner, policy, display name, and subscription counts. Topic tags can optionally be converted to asset metadata.

Authentication uses the standard AWS credential chain: static keys, a shared profile, an assumed role, or the environment defaults.

## Example Configuration

```yaml
credentials:
  region: "us-east-1"
  profile: "production"
  role: "arn:aws:iam::123456789012:role/MarmotDiscovery"
tags:
  - "aws"
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
