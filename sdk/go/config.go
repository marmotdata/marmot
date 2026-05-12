package marmot

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/marmotdata/marmot/sdk/go/auth"
)

// configFile mirrors ~/.config/marmot/config.yaml written by the CLI.
type configFile struct {
	Host           string                  `yaml:"host"`
	APIKey         string                  `yaml:"api_key"`
	Output         string                  `yaml:"output"`
	CurrentContext string                  `yaml:"current_context"`
	Contexts       map[string]contextEntry `yaml:"contexts"`
}

type contextEntry struct {
	Host string `yaml:"host"`
}

// activeHost returns the host for the active context, or the legacy top-level
// host as a fallback.
func (c *configFile) activeHost() string {
	if c.CurrentContext != "" {
		if ctx, ok := c.Contexts[c.CurrentContext]; ok && ctx.Host != "" {
			return ctx.Host
		}
	}
	return c.Host
}

func loadConfigFile() (*configFile, error) {
	dir, err := auth.ConfigDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return &configFile{}, nil
		}
		return nil, fmt.Errorf("read config.yaml: %w", err)
	}
	var c configFile
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config.yaml: %w", err)
	}
	return &c, nil
}
