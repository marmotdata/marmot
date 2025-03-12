package sync_test

import (
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/sync"
	"github.com/stretchr/testify/assert"
)

// TestAssetSyncer_MetadataMerging tests different metadata merging scenarios
func TestAssetSyncer_MetadataMerging(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]interface{}
		new      map[string]interface{}
		config   plugin.RawPluginConfig
		want     map[string]interface{}
	}{
		{
			name: "only allowed fields are merged",
			existing: map[string]interface{}{
				"service_name": "service1",
				"owner":        "team1",
				"ignored":      "value",
			},
			new: map[string]interface{}{
				"service_name": "service2",
				"owner":        "team2",
				"other":        "value",
			},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					Metadata: plugin.MetadataConfig{
						Allow: []string{"service_name", "owner"},
					},
				},
			},
			want: map[string]interface{}{
				"service_name": "service2",
				"owner":        "team2",
			},
		},
	}

	syncer := sync.NewAssetSyncer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &asset.Asset{Metadata: tt.existing}
			new := &asset.Asset{Metadata: tt.new}

			result := syncer.ApplyConfig(existing, new, tt.config)

			assert.Equal(t, tt.want, result.Metadata)
		})
	}
}

// TestAssetSyncer_TagMerging tests different tag merging strategies
func TestAssetSyncer_TagMerging(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		new      []string
		config   plugin.RawPluginConfig
		want     []string
	}{
		{
			name:     "append strategy combines all tags",
			existing: []string{"tag1", "tag2"},
			new:      []string{"tag2", "tag3"},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					Merge: plugin.MergeConfig{
						Tags: "append",
					},
				},
			},
			want: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "append_only strategy only adds new tags",
			existing: []string{"tag1", "tag2"},
			new:      []string{"tag2", "tag3"},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					Merge: plugin.MergeConfig{
						Tags: "append_only",
					},
				},
			},
			want: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "overwrite strategy replaces all tags",
			existing: []string{"tag1", "tag2"},
			new:      []string{"tag3", "tag4"},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					Merge: plugin.MergeConfig{
						Tags: "overwrite",
					},
				},
			},
			want: []string{"tag3", "tag4"},
		},
		{
			name:     "keep_first strategy keeps existing tags",
			existing: []string{"tag1", "tag2"},
			new:      []string{"tag3", "tag4"},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					Merge: plugin.MergeConfig{
						Tags: "keep_first",
					},
				},
			},
			want: []string{"tag1", "tag2"},
		},
	}

	syncer := sync.NewAssetSyncer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &asset.Asset{Tags: tt.existing}
			new := &asset.Asset{Tags: tt.new}

			result := syncer.ApplyConfig(existing, new, tt.config)

			assert.ElementsMatch(t, tt.want, result.Tags)
		})
	}
}

// TestAssetSyncer_ExternalLinks tests external link processing
func TestAssetSyncer_ExternalLinks(t *testing.T) {
	tests := []struct {
		name   string
		asset  *asset.Asset
		config plugin.RawPluginConfig
		want   []asset.ExternalLink
	}{
		{
			name: "processes template URLs",
			asset: &asset.Asset{
				Metadata: map[string]interface{}{
					"team": "team1",
					"app":  "app1",
				},
			},
			config: plugin.RawPluginConfig{
				"aws": plugin.BaseConfig{
					ExternalLinks: []plugin.ExternalLink{
						{
							Name: "Slack",
							Icon: "slack",
							URL:  "https://slack.com/team-{team}",
						},
						{
							Name: "Repo",
							Icon: "git",
							URL:  "https://github.com/{team}/{app}",
						},
					},
				},
			},
			want: []asset.ExternalLink{
				{
					Name: "Slack",
					Icon: "slack",
					URL:  "https://slack.com/team-team1",
				},
				{
					Name: "Repo",
					Icon: "git",
					URL:  "https://github.com/team1/app1",
				},
			},
		},
	}

	syncer := sync.NewAssetSyncer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.ApplyConfig(tt.asset, tt.asset, tt.config)
			assert.Equal(t, tt.want, result.ExternalLinks)
		})
	}
}

// TestSummary tests the summary output functionality
func TestSummary(t *testing.T) {
	summary := sync.Summary{
		Assets: map[string]sync.ChangeType{
			"asset1": sync.Created,
			"asset2": sync.Updated,
			"asset3": sync.NoChange,
		},
		Lineage: map[string]sync.ChangeType{
			"edge1": sync.Created,
			"edge2": sync.NoChange,
		},
		Documentation: map[string]sync.ChangeType{
			"doc1": sync.Updated,
		},
	}

	// Test counting
	assert.Equal(t, 1, summary.CountByType(summary.Assets, sync.Created))
	assert.Equal(t, 1, summary.CountByType(summary.Assets, sync.Updated))
	assert.Equal(t, 1, summary.CountByType(summary.Assets, sync.NoChange))
	assert.Equal(t, 2, summary.CountByType(summary.Lineage, sync.Created)+
		summary.CountByType(summary.Lineage, sync.NoChange))
	assert.Equal(t, 1, summary.CountByType(summary.Documentation, sync.Updated))
}

// Helper function to create a test asset
func createTestAsset(name string, metadata map[string]interface{}, tags []string) *asset.Asset {
	return &asset.Asset{
		Name:     &name,
		Metadata: metadata,
		Tags:     tags,
	}
}
