package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// External plugins are standalone marmot-plugin-* binaries built on
// github.com/marmotdata/plugin-sdk. Marmot launches them on demand via
// go-plugin and talks to them over gRPC: once at load time to read
// their metadata, then once per Validate or Discover call.

// PluginsDirEnvVar overrides the directory scanned for locally
// installed plugins.
const PluginsDirEnvVar = "MARMOT_PLUGINS_DIR"

// CacheDirEnvVar overrides the directory downloaded plugins are cached in.
const CacheDirEnvVar = "MARMOT_PLUGIN_CACHE_DIR"

// DefaultPluginsDir returns ~/.marmot/plugins, the default directory
// for locally installed plugins.
func DefaultPluginsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".marmot", "plugins")
}

// PluginsDir resolves the local plugins directory from the environment,
// falling back to the default.
func PluginsDir() string {
	if dir := os.Getenv(PluginsDirEnvVar); dir != "" {
		return dir
	}
	return DefaultPluginsDir()
}

// CacheDir resolves the plugin cache directory. Downloaded plugins are
// stored under <registry>/<name>/<version>/<os>_<arch>/ inside it.
func CacheDir() string {
	if dir := os.Getenv(CacheDirEnvVar); dir != "" {
		return dir
	}
	return filepath.Join(PluginsDir(), "cache")
}

// LoadBinary registers the external plugin binary at path in the
// global registry. The first registration of an ID wins; a binary
// whose ID is already registered is skipped. Load failures are logged,
// not returned: one broken plugin must not take down the rest.
func LoadBinary(path string) {
	registered, err := registerExternalPlugin(path)
	if err != nil {
		log.Error().Err(err).Str("plugin", path).Msg("Failed to load external plugin")
		return
	}
	if registered {
		log.Info().Str("plugin", path).Msg("Loaded external plugin")
	}
}

// registerExternalPlugin launches the plugin binary just long enough
// to read its metadata, then registers it as an ExternalSource. It
// reports whether the plugin was registered: a plugin whose ID is
// already taken is skipped, because the first registration wins and a
// plugin cannot shadow a built-in or another plugin that loaded
// earlier.
func registerExternalPlugin(path string) (bool, error) {
	process, err := pluginsdk.Open(path, pluginLogger())
	if err != nil {
		return false, err
	}
	defer process.Kill()

	sdkMeta, err := process.Source.GetMeta(context.Background())
	if err != nil {
		return false, fmt.Errorf("fetching plugin metadata: %w", err)
	}
	if sdkMeta.ID == "" {
		return false, fmt.Errorf("plugin metadata has no id")
	}

	if _, err := GetRegistry().Get(sdkMeta.ID); err == nil {
		log.Debug().Str("plugin", path).Str("id", sdkMeta.ID).Msg("Plugin already registered, skipping")
		return false, nil
	}

	// Only plugins that advertise data-preview support get a source that
	// implements DataFetcher, so the preview handler's type assertion
	// keeps distinguishing plugins with and without preview support.
	var source Source = &ExternalSource{path: path}
	if sdkMeta.SupportsDataPreview {
		source = &ExternalDataFetcherSource{ExternalSource{path: path}}
	}

	if err := GetRegistry().Register(*sdkMeta, source); err != nil {
		// Lost a race with a concurrent loader; the first registration
		// wins, same as the check above.
		log.Debug().Str("plugin", path).Str("id", sdkMeta.ID).Msg("Plugin already registered, skipping")
		return false, nil
	}
	return true, nil
}

// ExternalSource adapts an external plugin binary to the internal
// Source interface. Each call spawns the plugin process, performs the
// call over gRPC, and kills the process again.
type ExternalSource struct {
	path string
}

func (s *ExternalSource) Validate(config RawPluginConfig) (RawPluginConfig, error) {
	process, err := pluginsdk.Open(s.path, pluginLogger())
	if err != nil {
		return nil, err
	}
	defer process.Kill()

	validated, err := process.Source.Validate(context.Background(), pluginsdk.RawConfig(config))
	if err != nil {
		return nil, err
	}
	return RawPluginConfig(validated), nil
}

func (s *ExternalSource) Discover(ctx context.Context, config RawPluginConfig) (*DiscoveryResult, error) {
	process, err := pluginsdk.Open(s.path, pluginLogger())
	if err != nil {
		return nil, err
	}
	defer process.Kill()

	sdkResult, err := process.Source.Discover(ctx, pluginsdk.RawConfig(config))
	if err != nil {
		return nil, err
	}

	return convertDiscoveryResult(sdkResult)
}

// ExternalDataFetcherSource is an ExternalSource whose plugin advertises
// data-preview support; it additionally implements DataFetcher.
type ExternalDataFetcherSource struct {
	ExternalSource
}

func (s *ExternalDataFetcherSource) FetchSampleData(ctx context.Context, config RawPluginConfig, a *asset.Asset) ([]string, [][]interface{}, error) {
	process, err := pluginsdk.Open(s.path, pluginLogger())
	if err != nil {
		return nil, nil, err
	}
	defer process.Kill()

	sdkAsset, err := convertAsset(a)
	if err != nil {
		return nil, nil, err
	}

	return process.Source.FetchSampleData(ctx, pluginsdk.RawConfig(config), sdkAsset)
}

// convertAsset converts an internal asset into the SDK asset shape via
// a JSON round trip; fields the SDK does not mirror are dropped.
func convertAsset(a *asset.Asset) (*pluginsdk.Asset, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshaling asset: %w", err)
	}

	var sdkAsset pluginsdk.Asset
	if err := json.Unmarshal(data, &sdkAsset); err != nil {
		return nil, fmt.Errorf("unmarshaling asset: %w", err)
	}
	return &sdkAsset, nil
}

// convertDiscoveryResult converts the SDK result into the internal
// DiscoveryResult via a JSON round trip.
func convertDiscoveryResult(sdkResult *pluginsdk.DiscoveryResult) (*DiscoveryResult, error) {
	data, err := json.Marshal(sdkResult)
	if err != nil {
		return nil, fmt.Errorf("marshaling discovery result: %w", err)
	}

	var result DiscoveryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling discovery result: %w", err)
	}
	return &result, nil
}

func pluginLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Level:  hclog.Warn,
		Output: os.Stderr,
	})
}
