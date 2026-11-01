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

// ReplaceBinary registers the plugin binary at path under id, replacing whatever is registered there, so a plugin can move to another version without a restart. The binary must report id as its own: a cached binary never gets to stand in for a different plugin.
func ReplaceBinary(id, path string) error {
	meta, source, err := openExternalPlugin(path)
	if err != nil {
		return err
	}
	if meta.ID != id {
		return fmt.Errorf("plugin binary %s reports id %q, want %q", path, meta.ID, id)
	}
	GetRegistry().Replace(*meta, source)
	log.Info().Str("plugin", path).Str("id", id).Msg("Replaced external plugin")
	return nil
}

// registerExternalPlugin launches the plugin binary just long enough
// to read its metadata, then registers it as an ExternalSource. It
// reports whether the plugin was registered: a plugin whose ID is
// already taken is skipped, because the first registration wins and a
// plugin cannot shadow a built-in or another plugin that loaded
// earlier.
func registerExternalPlugin(path string) (bool, error) {
	meta, source, err := openExternalPlugin(path)
	if err != nil {
		return false, err
	}

	if _, err := GetRegistry().Get(meta.ID); err == nil {
		log.Debug().Str("plugin", path).Str("id", meta.ID).Msg("Plugin already registered, skipping")
		return false, nil
	}

	if err := GetRegistry().Register(*meta, source); err != nil {
		// Lost a race with a concurrent loader; the first registration
		// wins, same as the check above.
		log.Debug().Str("plugin", path).Str("id", meta.ID).Msg("Plugin already registered, skipping")
		return false, nil
	}
	return true, nil
}

// openExternalPlugin checks a plugin binary can be trusted, launches it just long enough to read its metadata and refuses it if it says it does not work with this Marmot.
func openExternalPlugin(path string) (*pluginsdk.Meta, Source, error) {
	if err := checkBinaryTrust(path); err != nil {
		return nil, nil, err
	}
	sum, err := pluginsdk.HashBinary(path)
	if err != nil {
		return nil, nil, err
	}

	binary := Binary{Path: path, SHA256: sum}
	process, err := binary.open()
	if err != nil {
		return nil, nil, err
	}
	defer process.Kill()

	sdkMeta, err := process.Source.GetMeta(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("fetching plugin metadata: %w", err)
	}
	if sdkMeta.ID == "" {
		return nil, nil, fmt.Errorf("plugin metadata has no id")
	}
	if err := checkCompatibility(sdkMeta); err != nil {
		return nil, nil, err
	}

	// Only plugins that advertise data-preview support get a source that
	// implements DataFetcher, so the preview handler's type assertion
	// keeps distinguishing plugins with and without preview support.
	var source Source = &ExternalSource{binary: binary}
	if sdkMeta.SupportsDataPreview {
		source = &ExternalDataFetcherSource{ExternalSource{binary: binary}}
	}
	return sdkMeta, source, nil
}

// Binary is a plugin binary Marmot has decided to trust, pinned to the sha256 it had at that moment so every later launch runs exactly those bytes.
type Binary struct {
	Path   string
	SHA256 []byte
}

func (b Binary) open() (*pluginsdk.PluginProcess, error) {
	return pluginsdk.Open(b.Path, b.SHA256, pluginLogger())
}

// ExternalSource adapts an external plugin binary to the internal
// Source interface. Each call spawns the plugin process, performs the
// call over gRPC, and kills the process again.
type ExternalSource struct {
	binary Binary
}

// Binary returns the pinned plugin binary so the instance manager can
// keep a long-running process of the same plugin for data queries.
func (s *ExternalSource) Binary() Binary {
	return s.binary
}

func (s *ExternalSource) Validate(config RawPluginConfig) (RawPluginConfig, error) {
	process, err := s.binary.open()
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
	process, err := s.binary.open()
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
	process, err := s.binary.open()
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
