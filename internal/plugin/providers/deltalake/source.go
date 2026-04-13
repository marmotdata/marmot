// +marmot:name=Delta Lake
// +marmot:description=This plugin discovers tables from Delta Lake transaction logs on local filesystems.
// +marmot:status=experimental
// +marmot:features=Assets
package deltalake

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig        `json:",inline"`
	*plugin.FileSourceConfig `json:",inline"`
	TablePaths               []string `json:"table_paths" description:"Paths to Delta Lake table directories (local paths, s3://bucket/prefix or git::url)" validate:"required,min=1"`
}

// +marmot:example-config
var _ = `
table_paths:
  - "/data/delta/events"
  - "/data/delta/users"
tags:
  - "delta-lake"
`

type Source struct {
	config *Config
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	for _, p := range config.TablePaths {
		if plugin.DetectSourceType(p) != "local" || (config.FileSourceConfig != nil && config.FileSourceConfig.SourceType != "" && config.FileSourceConfig.SourceType != "local") {
			continue
		}
		deltaLogDir := filepath.Join(p, "_delta_log")
		info, err := os.Stat(deltaLogDir)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("path %q does not contain a _delta_log directory", p)
		}
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	s.config = config

	var allAssets []asset.Asset
	var cleanups []func()
	defer func() {
		for _, fn := range cleanups {
			fn()
		}
	}()

	for _, tablePath := range config.TablePaths {
		localPath, cleanup, err := plugin.ResolveFilePath(ctx, config.FileSourceConfig, tablePath)
		if err != nil {
			log.Warn().Err(err).Str("path", tablePath).Msg("Failed to resolve Delta table path")
			continue
		}
		cleanups = append(cleanups, cleanup)

		snapshot, err := readDeltaLog(localPath)
		if err != nil {
			log.Warn().Err(err).Str("path", tablePath).Msg("Failed to read Delta table")
			continue
		}

		a := createTableAsset(snapshot, tablePath, config)
		allAssets = append(allAssets, a)
	}

	result := &plugin.DiscoveryResult{
		Assets: allAssets,
	}

	plugin.FilterDiscoveryResult(result, pluginConfig)

	return result, nil
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "deltalake",
		Name:        "Delta Lake",
		Description: "Discover tables from Delta Lake transaction logs on local filesystems",
		Icon:        "deltalake",
		Category:    "data-lake",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Delta Lake plugin")
	}
}
