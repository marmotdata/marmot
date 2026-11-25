package runs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/plugin"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// DefaultConnectionTestTimeout bounds a connection test. Plugins have no way to only connect, so a test is a discovery that is thrown away, and a large source would otherwise hold a request open for as long as a full run takes.
const DefaultConnectionTestTimeout = 30 * time.Second

// MaxConnectionTestTimeout caps the time a plugin may ask for, so a test still never holds a request open for as long as a full run of a large source.
const MaxConnectionTestTimeout = 5 * time.Minute

// ConnectionTestTimeout is how long a test of a plugin may run: the time the plugin asks for, up to MaxConnectionTestTimeout, or DefaultConnectionTestTimeout when it asks for none.
func ConnectionTestTimeout(meta pluginsdk.Meta) time.Duration {
	if meta.ConnectionTestTimeoutSeconds <= 0 {
		return DefaultConnectionTestTimeout
	}
	return min(time.Duration(meta.ConnectionTestTimeoutSeconds)*time.Second, MaxConnectionTestTimeout)
}

// ConnectionTestResult is what a connection test learned. Succeeded means the plugin connected and finished discovering within the time allowed; TimedOut means it was still working when the time ran out, which a large source can do with correct settings.
type ConnectionTestResult struct {
	Succeeded  bool           `json:"succeeded"`
	TimedOut   bool           `json:"timed_out,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	Assets     int            `json:"assets"`
	Tables     int            `json:"tables"`
	Lineage    int            `json:"lineage"`
	AssetTypes map[string]int `json:"asset_types,omitempty"`
} // @name ConnectionTestResult

// TestConnection runs a plugin's discovery against a validated config and reports what it could see, writing nothing to the catalog. The plugin process is stopped when the timeout passes, so a test never outlives its request.
func TestConnection(ctx context.Context, source plugin.Source, config plugin.RawPluginConfig, timeout time.Duration) *ConnectionTestResult {
	if timeout <= 0 {
		timeout = DefaultConnectionTestTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	result, err := source.Discover(ctx, config)
	out := &ConnectionTestResult{DurationMs: time.Since(started).Milliseconds()}
	if err != nil {
		out.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		if !out.TimedOut {
			out.Error = err.Error()
		}
		return out
	}
	if result == nil {
		out.Succeeded = true
		return out
	}

	// The same filters a run applies, so the counts match what the pipeline would catalog.
	plugin.FilterDiscoveryResult(result, config)

	out.Succeeded = true
	out.Assets = len(result.Assets)
	out.Lineage = len(result.Lineage)
	out.AssetTypes = map[string]int{}
	for _, a := range result.Assets {
		out.AssetTypes[a.Type]++
		if isTableType(a.Type) {
			out.Tables++
		}
	}
	return out
}

// isTableType matches the asset types the data query pages count as tables.
func isTableType(assetType string) bool {
	return strings.EqualFold(assetType, "table") || strings.EqualFold(assetType, "view")
}
