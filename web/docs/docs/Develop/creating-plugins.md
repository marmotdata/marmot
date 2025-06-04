# Creating a Marmot Plugin

This guide demonstrates how to create a simple "Hello World" plugin for Marmot.

## Overview

We'll create a basic plugin that:

- Creates two assets: "hello" and "world"
- Establishes lineage between them
- Requires no external connections

> **Note**: The `+marmot` comments throughout the code are used by Marmot's documentation generator to build plugin documentation. Always include these comments in your plugins.

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

Add your plugin to the source registry in `ingest.go`:

```go
var sourceRegistry = map[string]func() plugin.Source{
    ...
    "helloworld": func() plugin.Source { return &helloworld.Source{} },
}
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
3. A lineage relationship between "hello" and "world"
