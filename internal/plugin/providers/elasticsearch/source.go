// +marmot:name=Elasticsearch
// +marmot:description=This plugin discovers indices, data streams, and aliases from Elasticsearch clusters.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package elasticsearch

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Addresses []string `json:"addresses" description:"List of Elasticsearch node URLs"`
	CloudID   string   `json:"cloud_id" label:"Cloud ID" description:"Elastic Cloud ID for connecting to Elastic Cloud"`

	Username string `json:"username" description:"Username for basic authentication"`
	Password string `json:"password" description:"Password for basic authentication" sensitive:"true"`
	APIKey   string `json:"api_key" label:"API Key" description:"API key for authentication (mutually exclusive with username/password)" sensitive:"true"`

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
  - "https://elasticsearch.company.com:9200"
username: "elastic"
password: "changeme"
tags:
  - "elasticsearch"
  - "search"
`

type Source struct {
	config *Config
	client *elasticsearch.Client
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

	if len(config.Addresses) == 0 && config.CloudID == "" {
		return nil, fmt.Errorf("either addresses or cloud_id is required")
	}

	if config.APIKey != "" && (config.Username != "" || config.Password != "") {
		return nil, fmt.Errorf("api_key and username/password are mutually exclusive")
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) initClient() error {
	cfg := elasticsearch.Config{
		Addresses: s.config.Addresses,
		CloudID:   s.config.CloudID,
		Username:  s.config.Username,
		Password:  s.config.Password,
		APIKey:    s.config.APIKey,
	}

	if s.config.TLSSkipVerify || s.config.CACertPath != "" {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
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

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("creating elasticsearch client: %w", err)
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
		Msg("Elasticsearch discovery completed")

	result := &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}

	plugin.FilterDiscoveryResult(result, pluginConfig)

	return result, nil
}

func (s *Source) getClusterName(ctx context.Context) (string, error) {
	res, err := s.client.Info(s.client.Info.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("getting cluster info: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("cluster info returned status %s", res.Status())
	}

	var info struct {
		ClusterName string `json:"cluster_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decoding cluster info: %w", err)
	}

	return info.ClusterName, nil
}

func (s *Source) discoverIndices(ctx context.Context, clusterName string) ([]asset.Asset, []plugin.Statistic, error) {
	res, err := s.client.Cat.Indices(
		s.client.Cat.Indices.WithContext(ctx),
		s.client.Cat.Indices.WithFormat("json"),
		s.client.Cat.Indices.WithH("index", "health", "status", "uuid", "pri", "rep", "docs.count", "store.size", "creation.date.string"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("listing indices: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, nil, fmt.Errorf("cat indices returned status %s", res.Status())
	}

	var indices []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, nil, fmt.Errorf("decoding indices: %w", err)
	}

	mappings, err := s.getMappings(ctx, indices)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get index mappings")
		mappings = make(map[string]map[string]interface{})
	}

	var assets []asset.Asset
	var statistics []plugin.Statistic

	for _, idx := range indices {
		indexName, _ := idx["index"].(string)
		if indexName == "" {
			continue
		}

		if !s.config.IncludeSystemIndices && strings.HasPrefix(indexName, ".") {
			continue
		}

		metadata := map[string]interface{}{
			"cluster":       clusterName,
			"index_name":    indexName,
			"health":        getString(idx, "health"),
			"status":        getString(idx, "status"),
			"uuid":          getString(idx, "uuid"),
			"shards":        getInt(idx, "pri"),
			"replicas":      getInt(idx, "rep"),
			"docs_count":    getInt64(idx, "docs.count"),
			"store_size":    getString(idx, "store.size"),
			"creation_date": getString(idx, "creation.date.string"),
		}

		mrnValue := mrn.New("table", "elasticsearch", indexName)
		name := indexName
		description := fmt.Sprintf("Elasticsearch index %s in cluster %s", indexName, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		a := asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Table",
			Providers:   []string{"Elasticsearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "Elasticsearch",
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

		if mapping, ok := mappings[indexName]; ok {
			columns := flattenMappingProperties("", mapping)
			if len(columns) > 0 {
				jsonBytes, err := json.Marshal(columns)
				if err != nil {
					log.Warn().Err(err).Str("index", indexName).Msg("Failed to marshal schema columns")
				} else {
					a.Schema = map[string]string{
						"columns": string(jsonBytes),
					}
				}
			}
		}

		assets = append(assets, a)

		if s.config.IncludeIndexStats {
			docsCount := getInt64(idx, "docs.count")
			statistics = append(statistics, plugin.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "docs_count",
				Value:      float64(docsCount),
			})
		}
	}

	return assets, statistics, nil
}

func (s *Source) getMappings(ctx context.Context, indices []map[string]interface{}) (map[string]map[string]interface{}, error) {
	var indexNames []string
	for _, idx := range indices {
		name, _ := idx["index"].(string)
		if name == "" {
			continue
		}
		if !s.config.IncludeSystemIndices && strings.HasPrefix(name, ".") {
			continue
		}
		indexNames = append(indexNames, name)
	}

	if len(indexNames) == 0 {
		return nil, nil
	}

	res, err := s.client.Indices.GetMapping(
		s.client.Indices.GetMapping.WithContext(ctx),
		s.client.Indices.GetMapping.WithIndex(indexNames...),
	)
	if err != nil {
		return nil, fmt.Errorf("getting mappings: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("get mappings returned status %s", res.Status())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding mappings: %w", err)
	}

	result := make(map[string]map[string]interface{})
	for indexName, mappingJSON := range raw {
		var indexMapping struct {
			Mappings struct {
				Properties map[string]interface{} `json:"properties"`
			} `json:"mappings"`
		}
		if err := json.Unmarshal(mappingJSON, &indexMapping); err != nil {
			log.Warn().Err(err).Str("index", indexName).Msg("Failed to unmarshal mapping")
			continue
		}
		if indexMapping.Mappings.Properties != nil {
			result[indexName] = indexMapping.Mappings.Properties
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
	res, err := s.client.Indices.GetDataStream(
		s.client.Indices.GetDataStream.WithContext(ctx),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("listing data streams: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, nil, fmt.Errorf("get data streams returned status %s", res.Status())
	}

	var dsResponse struct {
		DataStreams []struct {
			Name           string `json:"name"`
			TimestampField struct {
				Name string `json:"name"`
			} `json:"timestamp_field"`
			Indices []struct {
				IndexName string `json:"index_name"`
			} `json:"indices"`
			Generation int    `json:"generation"`
			Status     string `json:"status"`
			ILMPolicy  string `json:"ilm_policy"`
			Template   string `json:"template"`
		} `json:"data_streams"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dsResponse); err != nil {
		return nil, nil, fmt.Errorf("decoding data streams: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for _, ds := range dsResponse.DataStreams {
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
			"ilm_policy":       ds.ILMPolicy,
			"template":         ds.Template,
		}

		mrnValue := mrn.New("data-stream", "elasticsearch", ds.Name)
		name := ds.Name
		description := fmt.Sprintf("Elasticsearch data stream %s in cluster %s", ds.Name, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Data Stream",
			Providers:   []string{"Elasticsearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "Elasticsearch",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		for _, backingIndex := range ds.Indices {
			if !s.config.IncludeSystemIndices && strings.HasPrefix(backingIndex.IndexName, ".") {
				continue
			}
			indexMRN := mrn.New("table", "elasticsearch", backingIndex.IndexName)
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
	res, err := s.client.Cat.Aliases(
		s.client.Cat.Aliases.WithContext(ctx),
		s.client.Cat.Aliases.WithFormat("json"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("listing aliases: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, nil, fmt.Errorf("cat aliases returned status %s", res.Status())
	}

	var aliasEntries []struct {
		Alias        string `json:"alias"`
		Index        string `json:"index"`
		Filter       string `json:"filter"`
		IsWriteIndex string `json:"is_write_index"`
	}
	if err := json.NewDecoder(res.Body).Decode(&aliasEntries); err != nil {
		return nil, nil, fmt.Errorf("decoding aliases: %w", err)
	}

	// Group alias entries by alias name (one alias can point to multiple indices)
	type aliasInfo struct {
		indices      []string
		isWriteIndex string
		hasFilter    bool
	}
	aliasMap := make(map[string]*aliasInfo)

	for _, entry := range aliasEntries {
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

		mrnValue := mrn.New("alias", "elasticsearch", aliasName)
		name := aliasName
		description := fmt.Sprintf("Elasticsearch alias %s in cluster %s", aliasName, clusterName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &name,
			Description: &description,
			Type:        "Alias",
			Providers:   []string{"Elasticsearch"},
			MRN:         &mrnValue,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "Elasticsearch",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		// Create REFERENCES lineage from alias to target indices
		for _, indexName := range info.indices {
			indexMRN := mrn.New("table", "elasticsearch", indexName)
			lineages = append(lineages, lineage.LineageEdge{
				Source: mrnValue,
				Target: indexMRN,
				Type:   "REFERENCES",
			})
		}
	}

	return assets, lineages, nil
}

// Helper functions for safe type extraction from map[string]interface{}

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func getInt64(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "elasticsearch",
		Name:        "Elasticsearch",
		Description: "Discover indices, data streams, and aliases from Elasticsearch clusters",
		Icon:        "elasticsearch",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Elasticsearch plugin")
	}
}
