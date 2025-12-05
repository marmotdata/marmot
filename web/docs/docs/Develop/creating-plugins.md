# Creating a Marmot Plugin

This guide walks you through creating a simple HelloWorld plugin for Marmot that demonstrates the core concepts of plugin development.

> **Documentation Comments**: The `+marmot:` comments throughout the code are used by Marmot's documentation generator. Always include these comments - they provide metadata for the plugin registry and generate user-facing documentation.

## 1. Create the Plugin Package

Create a new package in the `internal/plugin/providers` directory:

```bash
mkdir -p internal/plugin/providers/helloworld
```

## 2. Implement the Source Interface

Create `source.go` in your plugin directory:

```go
package helloworld

import (
    "context"
    "fmt"
    "time"

    "github.com/marmotdata/marmot/internal/core/asset"
    "github.com/marmotdata/marmot/internal/core/lineage"
    "github.com/marmotdata/marmot/internal/mrn"
    "github.com/marmotdata/marmot/internal/plugin"
    "github.com/rs/zerolog/log"
)

// +marmot:name=HelloWorld
// +marmot:description=A simple plugin that creates "hello" and "world" assets with lineage.
// +marmot:status=experimental
type Source struct {
    config *Config
}

// Config for HelloWorld plugin
// +marmot:config
type Config struct {
    plugin.BaseConfig `json:",inline"`

    // Add a simple config option
    Greeting string `json:"greeting" description:"Optional custom greeting message"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
greeting: "Hello, Marmot!"
tags:
  - "hello"
  - "example"
`

// Validate checks if the configuration is valid
func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
    config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
    if err != nil {
        return fmt.Errorf("unmarshaling config: %w", err)
    }

    s.config = config
    return nil
}

// Discover creates our hello and world assets
func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
    if err := s.Validate(pluginConfig); err != nil {
        return nil, fmt.Errorf("validating config: %w", err)
    }

    log.Info().Msg("HelloWorld plugin starting asset discovery")

    helloAsset := createHelloAsset(s.config)
    worldAsset := createWorldAsset(s.config)

    helloMRN := *helloAsset.MRN
    worldMRN := *worldAsset.MRN

    // Create lineage between assets
    lineageEdge := lineage.LineageEdge{
        Source:      helloMRN,
        Target:      worldMRN,
        Type:        "PRODUCES",
        Description: "Hello produces World",
        Metadata:    map[string]interface{}{"created_by": "helloworld_plugin"},
    }

    log.Info().
        Str("hello_mrn", helloMRN).
        Str("world_mrn", worldMRN).
        Msg("Created lineage relationship")

    return &plugin.DiscoveryResult{
        Assets:  []asset.Asset{helloAsset, worldAsset},
        Lineage: []lineage.LineageEdge{lineageEdge},
    }, nil
}

func createHelloAsset(config *Config) asset.Asset {
    name := "hello"
    mrnValue := mrn.New("Example", "HelloWorld", name)
    description := "Hello asset created by HelloWorld plugin"

    metadata := map[string]interface{}{
        "type": "foo",
    }

    if config.Greeting != "" {
        metadata["greeting"] = config.Greeting
    }

    return asset.Asset{
        Name:        &name,
        MRN:         &mrnValue,
        Type:        "Example",
        Providers:   []string{"HelloWorld"},
        Description: &description,
        Metadata:    metadata,
        Tags:        config.Tags,
        Sources: []asset.AssetSource{{
            Name:       "HelloWorld",
            LastSyncAt: time.Now(),
            Properties: metadata,
            Priority:   1,
        }},
    }
}

func createWorldAsset(config *Config) asset.Asset {
    name := "world"
    mrnValue := mrn.New("Example", "HelloWorld", name)
    description := "World asset created by HelloWorld plugin"

    metadata := map[string]interface{}{
        "type": "bar",
    }

    return asset.Asset{
        Name:        &name,
        MRN:         &mrnValue,
        Type:        "Example",
        Providers:   []string{"HelloWorld"},
        Description: &description,
        Metadata:    metadata,
        Tags:        config.Tags,
        Sources: []asset.AssetSource{{
            Name:       "HelloWorld",
            LastSyncAt: time.Now(),
            Properties: metadata,
            Priority:   1,
        }},
    }
}
```

## 3. Define Metadata Types

Create a simple `metadata.go` file. This defines what metadata is available and exported from your plugin.

```go
package helloworld

// HelloWorldFields represents example metadata fields
// +marmot:metadata
type HelloWorldFields struct {
    Type string `json:"type" metadata:"type" description:"The type of asset created"`
    Greeting  string `json:"greeting" metadata:"greeting" description:"Optional custom greeting message"`
}
```

## 4. Register the Plugin

Create an `init()` function at the end of `source.go` to auto-register your plugin:

```go
func init() {
	meta := plugin.PluginMeta{
		ID:          "helloworld",
		Name:        "HelloWorld",
		Description: "A simple plugin that creates hello and world assets with lineage",
		Icon:        "wave",
		Category:    "example",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register HelloWorld plugin")
	}
}
```

The init() function:

- Runs automatically when the package is imported
- Defines plugin metadata (ID, name, description, icon, category)
- Auto-generates the configuration spec from your Config struct
- Registers the plugin with the global registry

**Important**: Your plugin must be imported in `internal/api/v1/server.go` to trigger registration:

```go
import (
    _ "github.com/marmotdata/marmot/internal/plugin/providers/helloworld"
)
```

## 5. Test the Plugin

Create a test configuration file `hello.yaml`:

```yaml
runs:
  - helloworld:
      greeting: "Hello from my first plugin!"
      tags:
        - "example"
        - "hello"
```

Run the ingestion:

```bash
go run cmd/main.go ingest -c hello.yaml -H http://localhost:8080 -k your-api-key
```

After running, you should see two new assets in your catalog:

1. An asset named "hello"
2. An asset named "world"
3. A lineage relationship showing "hello" produces "world"

## Configuration Spec Generation

The `plugin.GenerateConfigSpec()` function automatically generates a UI-ready configuration schema from your Config struct using struct tags:

```go
type Config struct {
    plugin.BaseConfig `json:",inline"`

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
- `validate`: Validation rules (required, min, max, oneof, etc.)
- `sensitive`: Marks field as password/secret
- `default`: Default value

## Plugin Interface

All plugins must implement the `plugin.Source` interface:

```go
type Source interface {
    Validate(rawConfig RawPluginConfig) error
    Discover(ctx context.Context, config RawPluginConfig) (*DiscoveryResult, error)
}
```

**Validate**: Unmarshals and validates configuration before discovery runs
**Discover**: Performs the actual asset discovery and returns assets + lineage
