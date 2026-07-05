# Creating a Marmot Plugin

This guide walks you through creating a simple HelloWorld plugin for Marmot that demonstrates the core concepts of plugin development.

import { CalloutCard } from '@site/src/components/DocCard';

Marmot plugins are standalone binaries built on the [Marmot plugin SDK](https://github.com/marmotdata/plugin-sdk). Marmot launches them on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to them over gRPC: once at startup to read their metadata, then once per run to validate configuration and discover assets. Your plugin lives in its own repository, with its own dependencies and release cycle. [marmot-plugin-gcs](https://github.com/marmotdata/marmot-plugin-gcs) is a complete real-world example.

## 1. Create the Plugin Module

Create a new Go module and add the SDK:

```bash
mkdir marmot-plugin-helloworld && cd marmot-plugin-helloworld
go mod init github.com/you/marmot-plugin-helloworld
go get github.com/marmotdata/plugin-sdk
```

## 2. Implement the Source Interface

Create `source.go`:

```go
package main

import (
    "context"
    "fmt"
    "time"

    pluginsdk "github.com/marmotdata/plugin-sdk"
    "github.com/marmotdata/plugin-sdk/mrn"
)

// Config for the HelloWorld plugin
type Config struct {
    pluginsdk.BaseConfig `json:",inline"`

    // Add a simple config option
    Greeting string `json:"greeting" description:"Optional custom greeting message"`
}

type Source struct {
    config *Config
}

// Validate checks if the configuration is valid
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
    config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
    if err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    if err := pluginsdk.ValidateStruct(config); err != nil {
        return nil, err
    }

    s.config = config
    return rawConfig, nil
}

// Discover creates our hello and world assets
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
    if _, err := s.Validate(rawConfig); err != nil {
        return nil, fmt.Errorf("validating config: %w", err)
    }

    helloAsset := createHelloAsset(s.config)
    worldAsset := createWorldAsset(s.config)

    // Create lineage between assets
    lineageEdge := pluginsdk.LineageEdge{
        Source: *helloAsset.MRN,
        Target: *worldAsset.MRN,
        Type:   "PRODUCES",
    }

    return &pluginsdk.DiscoveryResult{
        Assets:  []pluginsdk.Asset{helloAsset, worldAsset},
        Lineage: []pluginsdk.LineageEdge{lineageEdge},
    }, nil
}

func createHelloAsset(config *Config) pluginsdk.Asset {
    name := "hello"
    mrnValue := mrn.New("Example", "HelloWorld", name)
    description := "Hello asset created by HelloWorld plugin"

    metadata := map[string]interface{}{
        "type": "foo",
    }

    if config.Greeting != "" {
        metadata["greeting"] = config.Greeting
    }

    return pluginsdk.Asset{
        Name:        &name,
        MRN:         &mrnValue,
        Type:        "Example",
        Providers:   []string{"HelloWorld"},
        Description: &description,
        Metadata:    metadata,
        Tags:        config.Tags,
        Sources: []pluginsdk.AssetSource{{
            Name:       "HelloWorld",
            LastSyncAt: time.Now(),
            Properties: metadata,
            Priority:   1,
        }},
    }
}

func createWorldAsset(config *Config) pluginsdk.Asset {
    name := "world"
    mrnValue := mrn.New("Example", "HelloWorld", name)
    description := "World asset created by HelloWorld plugin"

    metadata := map[string]interface{}{
        "type": "bar",
    }

    return pluginsdk.Asset{
        Name:        &name,
        MRN:         &mrnValue,
        Type:        "Example",
        Providers:   []string{"HelloWorld"},
        Description: &description,
        Metadata:    metadata,
        Tags:        config.Tags,
        Sources: []pluginsdk.AssetSource{{
            Name:       "HelloWorld",
            LastSyncAt: time.Now(),
            Properties: metadata,
            Priority:   1,
        }},
    }
}
```

## 3. Serve the Plugin

Create `main.go`. `Serve` hands your source to go-plugin and blocks until Marmot disconnects:

```go
package main

import (
    pluginsdk "github.com/marmotdata/plugin-sdk"
)

func main() {
    pluginsdk.Serve(&pluginsdk.ServeConfig{
        Meta: pluginsdk.Meta{
            ID:          "helloworld",
            Name:        "HelloWorld",
            Description: "A simple plugin that creates hello and world assets with lineage",
            Icon:        "wave",
            Category:    "example",
            ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
        },
        Source: &Source{},
    })
}
```

The metadata defines how your plugin shows up in Marmot: its ID (the source name used in ingest configs), display name, description, icon, category, and the configuration form rendered in the UI.

## 4. Install the Plugin

Build the binary and copy it into the directory Marmot scans for local plugins. The binary name must start with `marmot-plugin-`:

```bash
go build -o ~/.marmot/plugins/marmot-plugin-helloworld .
```

Marmot discovers it at startup, both the server and the CLI. Set `MARMOT_PLUGINS_DIR` if your Marmot uses a different plugins directory.

## 5. Test the Plugin

Create a test configuration file `hello.yaml`:

```yaml
name: "helloworld"
runs:
  - helloworld:
      greeting: "Hello from my first plugin!"
      tags:
        - "example"
        - "hello"
```

Run the ingestion:

```bash
marmot ingest -c hello.yaml --host http://localhost:8080 --api-key your-api-key
```

After running, you should see two new assets in your catalog:

1. An asset named "hello"
2. An asset named "world"
3. A lineage relationship showing "hello" produces "world"

## Configuration Spec Generation

The `pluginsdk.GenerateConfigSpec()` function automatically generates a UI-ready configuration schema from your Config struct using struct tags:

```go
type Config struct {
    pluginsdk.BaseConfig `json:",inline"`

    // Text input
    Greeting string `json:"greeting" description:"Custom greeting message"`

    // Dropdown/select (using oneof validation)
    Mode string `json:"mode" description:"Operation mode" validate:"oneof=simple advanced"`

    // Sensitive field (password input)
    APIKey string `json:"api_key" description:"API authentication key" sensitive:"true"`

    // Number input with validation
    Timeout int `json:"timeout" description:"Request timeout in seconds" validate:"min=1,max=300" default:"30"`

    // Required field
    Host string `json:"host" description:"Server hostname" validate:"required"`

    // Nested object
    TLS *TLSConfig `json:"tls,omitempty" description:"TLS configuration"`
}
```

Supported tags:

- `json`: Field name in JSON
- `description`: Help text shown in UI
- `label`: Display label (defaults to a title-cased field name)
- `validate`: Validation rules (required, min, max, oneof, etc.)
- `sensitive`: Marks field as password/secret
- `default`: Default value

`BaseConfig` adds the standard `tags`, `external_links`, and `filter` fields every plugin supports. Filtering is applied by Marmot after discovery; your plugin only needs to carry the config.

## Plugin Interface

All plugins implement the `pluginsdk.Source` interface:

```go
type Source interface {
    Validate(config RawConfig) (RawConfig, error)
    Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error)
}
```

**Validate**: Unmarshals and validates configuration before discovery runs
**Discover**: Performs the actual asset discovery and returns assets, lineage, and documentation

## How Plugins Are Loaded

Marmot looks for `marmot-plugin-*` binaries in two places at startup:

- `~/.marmot/plugins` (`MARMOT_PLUGINS_DIR`): plugins you installed by hand, like the one in this guide
- `~/.marmot/plugins/cache` (`MARMOT_PLUGIN_CACHE_DIR`): core plugins Marmot downloads from `ghcr.io/marmotdata/plugins`

Local plugins load first, so a local binary shadows a downloaded core plugin with the same ID. That makes iterating on a core plugin easy: build it into `~/.marmot/plugins` and Marmot runs your build instead of the released one.

<CalloutCard
  title="Need Help Building a Plugin?"
  description="Join our Discord community to get help, share your plugins, and connect with other contributors."
  href="https://discord.gg/TWCk7hVFN4"
  buttonText="Join Discord"
  variant="secondary"
  icon="mdi:account-group"
/>
