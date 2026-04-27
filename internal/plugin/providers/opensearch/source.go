// +marmot:name=OpenSearch
// +marmot:description=This plugin discovers indices, data streams, and aliases from OpenSearch clusters.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package opensearch

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	opensearchgo "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/rs/zerolog/log"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Addresses []string `json:"addresses" description:"List of OpenSearch node URLs" validate:"required"`
	Username  string   `json:"username" description:"Username for basic authentication"`
	Password  string   `json:"password" description:"Password for basic authentication" sensitive:"true"`

	TLSSkipVerify bool   `json:"tls_skip_verify" label:"TLS Skip Verify" description:"Skip TLS certificate verification" default:"false"`
	CACertPath    string `json:"ca_cert_path" label:"CA Certificate Path" description:"Path to a custom CA certificate file"`

	IncludeDataStreams   bool `json:"include_data_streams" description:"Discover data streams" default:"true"`
	IncludeAliases       bool `json:"include_aliases" description:"Discover aliases" default:"true"`
	IncludeIndexStats    bool `json:"include_index_stats" description:"Collect document count and store size metrics" default:"true"`
	IncludeSystemIndices bool `json:"include_system_indices" description:"Include system indices (prefixed with .)" default:"false"`
}

// +marmot:example-config
var _ = `
addresses:
  - "https://opensearch.company.com:9200"
username: "admin"
password: "admin"
tags:
  - "opensearch"
  - "search"
`

type Source struct {
	config *Config
	client *opensearchapi.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if _, ok := rawConfig["include_data_streams"]; !ok {
		config.IncludeDataStreams = true
	}
	if _, ok := rawConfig["include_aliases"]; !ok {
		config.IncludeAliases = true
	}
	if _, ok := rawConfig["include_index_stats"]; !ok {
		config.IncludeIndexStats = true
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) initClient() error {
	cfg := opensearchgo.Config{
		Addresses: s.config.Addresses,
		Username:  s.config.Username,
		Password:  s.config.Password,
	}

	if s.config.TLSSkipVerify || s.config.CACertPath != "" {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = s.config.TLSSkipVerify //nolint:gosec // User-configured option

		if s.config.CACertPath != "" {
			cert, err := os.ReadFile(s.config.CACertPath)
			if err != nil {
				return fmt.Errorf("reading CA certificate: %w", err)
			}
			cfg.CACert = cert
		}

		cfg.Transport = transport
	}

	client, err := opensearchapi.NewClient(opensearchapi.Config{Client: cfg})
	if err != nil {
		return fmt.Errorf("creating opensearch client: %w", err)
	}

	s.client = client
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if err := s.initClient(); err != nil {
		return nil, fmt.Errorf("initializing client: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var statistics []plugin.Statistic

	clusterName, err := s.getClusterName(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get cluster info")
		clusterName = "unknown"
	}

	indexAssets, indexStats, err := s.discoverIndices(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("discovering indices: %w", err)
	}
	assets = append(assets, indexAssets...)
	statistics = append(statistics, indexStats...)
	log.Debug().Int("count", len(indexAssets)).Msg("Discovered indices")

	if s.config.IncludeDataStreams {
		dsAssets, dsLineages, err := s.discoverDataStreams(ctx, clusterName)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover data streams")
		} else {
			assets = append(assets, dsAssets...)
			lineages = append(lineages, dsLineages...)
			log.Debug().Int("count", len(dsAssets)).Msg("Discovered data streams")
		}
	}

	if s.config.IncludeAliases {
		aliasAssets, aliasLineages, err := s.discoverAliases(ctx, clusterName)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover aliases")
		} else {
			assets = append(assets, aliasAssets...)
			lineages = append(lineages, aliasLineages...)
			log.Debug().Int("count", len(aliasAssets)).Msg("Discovered aliases")
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("OpenSearch discovery completed")

	result := &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}

	plugin.FilterDiscoveryResult(result, pluginConfig)

	return result, nil
}

func (s *Source) getClusterName(ctx context.Context) (string, error) {
	resp, err := s.client.Info(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("getting cluster info: %w", err)
	}
	return resp.ClusterName, nil
}

func (s *Source) discoverIndices(ctx context.Context, clusterName string) ([]asset.Asset, []plugin.Statistic, error) {
	resp, err := s.client.Cat.Indices(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing indices: %w", err)
	}

	var indexNames []string
	for _, idx := range resp.Indices {
		if idx.Index == "" {
			continue
		}
		if !s.config.IncludeSystemIndices && strings.HasPrefix(idx.Index, ".") {
			continue
		}
		indexNames = append(indexNames, idx.Index)
	}

	mappings, err := s.getMappings(ctx, indexNames)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get index mappings")
		mappings = make(map[string]map[string]interface{})
	}

	var assets []asset.Asset
	var statistics []plugin.Statistic

	for _, idx := range resp.Indices {
		if idx.Index == "" {
			continue
		}
		if !s.config.IncludeSystemIndices && strings.HasPrefix(idx.Index, ".") {
			continue
		}

		var shards, replicas int
		if idx.Primary != nil {
			shards = *idx.Primary
		}
		if idx.Replica != nil {
			replicas = *idx.Replica
		}
		var docsCount int64
		if idx.DocsCount != nil {
			docsCount = int64(*idx.DocsCount)
		}
		var storeSize string
		if idx.StoreSize != nil {
			storeSize = *idx.StoreSize
		}

		metadata := map[string]interface{}{
			"cluster":       clusterName,
			"index_name":    idx.Index,
			"health":        idx.Health,
			"status":        idx.Status,
			"uuid":          idx.UUID,
			"shards":        shards,
			"replicas":      replicas,
			"docs_count":    docsCount,
			"store_size":    storeSize,
			"creation_date": idx.CreationDateString,
		}

		mrnValue := mrn.New("table", "opensearch", idx.Index)
		name := idx.Index
		description := fmt.Sprintf("OpenSearch index %s in cluster %s", idx.Index, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		a := asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Table",
			Providers:   []string{"OpenSearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "OpenSearch",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}

		if len(s.config.ExternalLinks) > 0 {
			for _, link := range s.config.ExternalLinks {
				a.ExternalLinks = append(a.ExternalLinks, asset.ExternalLink{
					Name: link.Name,
					Icon: link.Icon,
					URL:  link.URL,
				})
			}
		}

		if mapping, ok := mappings[idx.Index]; ok {
			columns := flattenMappingProperties("", mapping)
			if len(columns) > 0 {
				jsonBytes, err := json.Marshal(columns)
				if err != nil {
					log.Warn().Err(err).Str("index", idx.Index).Msg("Failed to marshal schema columns")
				} else {
					a.Schema = map[string]string{
						"columns": string(jsonBytes),
					}
				}
			}
		}

		assets = append(assets, a)

		if s.config.IncludeIndexStats {
			statistics = append(statistics, plugin.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "docs_count",
				Value:      float64(docsCount),
			})
		}
	}

	return assets, statistics, nil
}

func (s *Source) getMappings(ctx context.Context, indexNames []string) (map[string]map[string]interface{}, error) {
	if len(indexNames) == 0 {
		return nil, nil
	}

	resp, err := s.client.Indices.Mapping.Get(ctx, &opensearchapi.MappingGetReq{
		Indices: indexNames,
	})
	if err != nil {
		return nil, fmt.Errorf("getting mappings: %w", err)
	}

	result := make(map[string]map[string]interface{})
	for indexName, indexMapping := range resp.Indices {
		var parsed struct {
			Properties map[string]interface{} `json:"properties"`
		}
		if err := json.Unmarshal(indexMapping.Mappings, &parsed); err != nil {
			log.Warn().Err(err).Str("index", indexName).Msg("Failed to unmarshal mapping")
			continue
		}
		if parsed.Properties != nil {
			result[indexName] = parsed.Properties
		}
	}

	return result, nil
}

func flattenMappingProperties(prefix string, properties map[string]interface{}) []map[string]interface{} {
	var columns []map[string]interface{}

	for fieldName, fieldDef := range properties {
		fullName := fieldName
		if prefix != "" {
			fullName = prefix + "." + fieldName
		}

		fieldMap, ok := fieldDef.(map[string]interface{})
		if !ok {
			continue
		}

		if nestedProps, ok := fieldMap["properties"].(map[string]interface{}); ok {
			columns = append(columns, flattenMappingProperties(fullName, nestedProps)...)
			continue
		}

		fieldType, _ := fieldMap["type"].(string)
		if fieldType == "" {
			continue
		}

		col := map[string]interface{}{
			"name": fullName,
			"type": fieldType,
		}

		if analyzer, ok := fieldMap["analyzer"].(string); ok {
			col["analyzer"] = analyzer
		}

		if index, ok := fieldMap["index"]; ok {
			col["index"] = fmt.Sprintf("%v", index)
		}

		columns = append(columns, col)
	}

	return columns
}

func (s *Source) discoverDataStreams(ctx context.Context, clusterName string) ([]asset.Asset, []lineage.LineageEdge, error) {
	resp, err := s.client.DataStream.Get(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing data streams: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for _, ds := range resp.DataStreams {
		if !s.config.IncludeSystemIndices && strings.HasPrefix(ds.Name, ".") {
			continue
		}

		metadata := map[string]interface{}{
			"cluster":          clusterName,
			"data_stream_name": ds.Name,
			"timestamp_field":  ds.TimestampField.Name,
			"backing_indices":  len(ds.Indices),
			"generation":       ds.Generation,
			"status":           ds.Status,
			"template":         ds.Template,
		}

		mrnValue := mrn.New("data-stream", "opensearch", ds.Name)
		name := ds.Name
		description := fmt.Sprintf("OpenSearch data stream %s in cluster %s", ds.Name, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Data Stream",
			Providers:   []string{"OpenSearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "OpenSearch",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		for _, backingIndex := range ds.Indices {
			if !s.config.IncludeSystemIndices && strings.HasPrefix(backingIndex.Name, ".") {
				continue
			}
			indexMRN := mrn.New("table", "opensearch", backingIndex.Name)
			lineages = append(lineages, lineage.LineageEdge{
				Source: mrnValue,
				Target: indexMRN,
				Type:   "CONTAINS",
			})
		}
	}

	return assets, lineages, nil
}

func (s *Source) discoverAliases(ctx context.Context, clusterName string) ([]asset.Asset, []lineage.LineageEdge, error) {
	resp, err := s.client.Cat.Aliases(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing aliases: %w", err)
	}

	type aliasInfo struct {
		indices      []string
		isWriteIndex string
		hasFilter    bool
	}
	aliasMap := make(map[string]*aliasInfo)

	for _, entry := range resp.Aliases {
		if !s.config.IncludeSystemIndices && strings.HasPrefix(entry.Alias, ".") {
			continue
		}
		info, ok := aliasMap[entry.Alias]
		if !ok {
			info = &aliasInfo{}
			aliasMap[entry.Alias] = info
		}
		info.indices = append(info.indices, entry.Index)
		if entry.IsWriteIndex == "true" {
			info.isWriteIndex = "true"
		}
		if entry.Filter != "-" && entry.Filter != "" {
			info.hasFilter = true
		}
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for aliasName, info := range aliasMap {
		filterDefined := "false"
		if info.hasFilter {
			filterDefined = "true"
		}
		isWriteIndex := info.isWriteIndex
		if isWriteIndex == "" {
			isWriteIndex = "false"
		}

		metadata := map[string]interface{}{
			"cluster":        clusterName,
			"alias_name":     aliasName,
			"indices":        strings.Join(info.indices, ","),
			"is_write_index": isWriteIndex,
			"filter_defined": filterDefined,
		}

		mrnValue := mrn.New("alias", "opensearch", aliasName)
		name := aliasName
		description := fmt.Sprintf("OpenSearch alias %s in cluster %s", aliasName, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Alias",
			Providers:   []string{"OpenSearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "OpenSearch",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		for _, indexName := range info.indices {
			indexMRN := mrn.New("table", "opensearch", indexName)
			lineages = append(lineages, lineage.LineageEdge{
				Source: mrnValue,
				Target: indexMRN,
				Type:   "REFERENCES",
			})
		}
	}

	return assets, lineages, nil
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "opensearch",
		Name:        "OpenSearch",
		Description: "Discover indices, data streams, and aliases from OpenSearch clusters",
		Icon:        "opensearch",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register OpenSearch plugin")
	}
}
