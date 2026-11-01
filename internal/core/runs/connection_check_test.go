package runs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
)

type fakeSource struct {
	discover func(ctx context.Context) (*plugin.DiscoveryResult, error)
}

func (f fakeSource) Validate(c plugin.RawPluginConfig) (plugin.RawPluginConfig, error) { return c, nil }

func (f fakeSource) Discover(ctx context.Context, _ plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	return f.discover(ctx)
}

func named(name, assetType string) asset.Asset {
	return asset.Asset{Name: &name, Type: assetType}
}

func TestConnectionTestCountsWhatTheSourceCanSee(t *testing.T) {
	source := fakeSource{discover: func(context.Context) (*plugin.DiscoveryResult, error) {
		return &plugin.DiscoveryResult{
			Assets:  []asset.Asset{named("orders", "Table"), named("order_totals", "View"), named("shop", "Database")},
			Lineage: []lineage.LineageEdge{{Source: "a", Target: "b"}},
		}, nil
	}}

	got := TestConnection(context.Background(), source, plugin.RawPluginConfig{}, time.Second)

	assert.True(t, got.Succeeded)
	assert.False(t, got.TimedOut)
	assert.Empty(t, got.Error)
	assert.Equal(t, 3, got.Assets)
	assert.Equal(t, 2, got.Tables)
	assert.Equal(t, 1, got.Lineage)
	assert.Equal(t, map[string]int{"Table": 1, "View": 1, "Database": 1}, got.AssetTypes)
}

func TestConnectionTestReportsThePluginError(t *testing.T) {
	source := fakeSource{discover: func(context.Context) (*plugin.DiscoveryResult, error) {
		return nil, errors.New("password authentication failed for user marmot")
	}}

	got := TestConnection(context.Background(), source, plugin.RawPluginConfig{}, time.Second)

	assert.False(t, got.Succeeded)
	assert.False(t, got.TimedOut)
	assert.Equal(t, "password authentication failed for user marmot", got.Error)
}

func TestConnectionTestStopsAtTheTimeout(t *testing.T) {
	source := fakeSource{discover: func(ctx context.Context) (*plugin.DiscoveryResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}

	started := time.Now()
	got := TestConnection(context.Background(), source, plugin.RawPluginConfig{}, 50*time.Millisecond)

	assert.Less(t, time.Since(started), time.Second)
	assert.False(t, got.Succeeded)
	assert.True(t, got.TimedOut)
	assert.Empty(t, got.Error)
}
