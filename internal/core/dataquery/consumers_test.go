package dataquery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type agentAssets struct {
	asset.Service
	mu       sync.Mutex
	existing *asset.Asset
	created  []asset.CreateInput
}

func (a *agentAssets) GetByTypeAndName(_ context.Context, assetType, name string) (*asset.Asset, error) {
	if a.existing != nil && assetType == AgentAssetType && *a.existing.Name == name {
		return a.existing, nil
	}
	return nil, asset.ErrAssetNotFound
}

func (a *agentAssets) Create(_ context.Context, in asset.CreateInput) (*asset.Asset, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.created = append(a.created, in)
	return &asset.Asset{Name: in.Name, MRN: in.MRN}, nil
}

type edgeLog struct {
	mu    sync.Mutex
	edges []lineage.ObservedEdge
}

func (l *edgeLog) BatchObservedLineage(_ context.Context, edges []lineage.ObservedEdge) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.edges = append(l.edges, edges...)
	return nil
}

func (l *edgeLog) snapshot() []lineage.ObservedEdge {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]lineage.ObservedEdge(nil), l.edges...)
}

type personPrincipal struct{ fakePrincipal }

func (personPrincipal) Type() auth.PrincipalType { return auth.PrincipalTypeUser }

func TestConsumerRecorderCatalogsAgentAndThrottlesEdges(t *testing.T) {
	assets := &agentAssets{}
	edges := &edgeLog{}
	rec := newConsumerRecorder(assets, edges, time.Hour)

	rec.record(fakePrincipal{}, []string{orders, SourceMRN("shop-pg")})
	require.Eventually(t, func() bool { return len(edges.snapshot()) == 1 }, time.Second, 5*time.Millisecond)

	got := edges.snapshot()[0]
	assert.Equal(t, orders, got.Source)
	assert.Equal(t, "mrn://agent/marmot/test-agent", got.Target)
	assert.Equal(t, EdgeTypeDataQuery, got.Type)
	require.Len(t, assets.created, 1)
	assert.Equal(t, AgentAssetType, assets.created[0].Type)

	// A repeat read inside the interval writes nothing; a new table does, without creating the agent again.
	rec.record(fakePrincipal{}, []string{orders})
	rec.record(fakePrincipal{}, []string{salaries})
	require.Eventually(t, func() bool { return len(edges.snapshot()) == 2 }, time.Second, 5*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	assert.Len(t, edges.snapshot(), 2)
	assert.Equal(t, salaries, edges.snapshot()[1].Source)
	assert.Len(t, assets.created, 1)
}

func TestConsumerRecorderReusesSDKAgent(t *testing.T) {
	name, existing := "test-agent", "mrn://agent/claudeagent/test-agent"
	assets := &agentAssets{existing: &asset.Asset{Name: &name, MRN: &existing}}
	edges := &edgeLog{}
	rec := newConsumerRecorder(assets, edges, time.Hour)

	rec.record(fakePrincipal{}, []string{orders})
	require.Eventually(t, func() bool { return len(edges.snapshot()) == 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, existing, edges.snapshot()[0].Target)
	assert.Empty(t, assets.created)
}

func TestConsumerRecorderSkipsPeople(t *testing.T) {
	edges := &edgeLog{}
	rec := newConsumerRecorder(&agentAssets{}, edges, time.Hour)

	rec.record(personPrincipal{}, []string{orders})
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, edges.snapshot())
}
