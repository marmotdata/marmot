// Package deltalake discovers tables from Delta Lake transaction logs.
package deltalake

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/filesource"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "deltalake",
		Name:        "Delta Lake",
		Description: "Discover tables from Delta Lake transaction logs on local filesystems",
		Icon:        "deltalake",
		Category:    "data-lake",
		Status:      "experimental",
		Features:    []string{"Assets"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

type Config struct {
	pluginsdk.BaseConfig         `json:",inline"`
	*filesource.FileSourceConfig `json:",inline"`
	TablePaths                   []string `json:"table_paths" description:"Paths to Delta Lake table directories (local paths, s3://bucket/prefix or git::url)" validate:"required,min=1"`
}

// Example configuration for the plugin
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

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	for _, p := range config.TablePaths {
		if filesource.DetectSourceType(p) != "local" || (config.FileSourceConfig != nil && config.FileSourceConfig.SourceType != "" && config.FileSourceConfig.SourceType != "local") {
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

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	s.config = config

	var allAssets []pluginsdk.Asset
	var cleanups []func()
	defer func() {
		for _, fn := range cleanups {
			fn()
		}
	}()

	for _, tablePath := range config.TablePaths {
		localPath, cleanup, err := filesource.ResolveFilePath(ctx, config.FileSourceConfig, tablePath)
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

	return &pluginsdk.DiscoveryResult{
		Assets: allAssets,
	}, nil
}
