package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/marmotdata/marmot/internal/core/search"
)

// docID returns the ES document ID for a given type and entity ID.
func docID(entityType, entityID string) string {
	return entityType + ":" + entityID
}

// Index indexes a single document into Elasticsearch.
func (c *Client) Index(ctx context.Context, doc search.SearchDocument) error {
	body := documentToMap(doc)
	id := docID(doc.Type, doc.EntityID)

	_, err := c.es.Index(c.index).Id(id).Document(body).Do(ctx)
	if err != nil {
		return fmt.Errorf("indexing document %s:%s: %w", doc.Type, doc.EntityID, err)
	}

	return nil
}

// BulkIndex indexes multiple documents using the bulk API.
func (c *Client) BulkIndex(ctx context.Context, docs []search.SearchDocument) error {
	if len(docs) == 0 {
		return nil
	}

	req := c.es.Bulk().Index(c.index)
	for _, doc := range docs {
		id := docID(doc.Type, doc.EntityID)
		body := documentToMap(doc)
		if err := req.IndexOp(types.IndexOperation{Id_: &id}, body); err != nil {
			return fmt.Errorf("adding bulk index op: %w", err)
		}
	}

	res, err := req.Do(ctx)
	if err != nil {
		return fmt.Errorf("bulk indexing: %w", err)
	}

	if res.Errors {
		return fmt.Errorf("bulk indexing completed with errors")
	}

	return nil
}

// Delete removes a document from the index.
// The typed Delete API treats 404 as success, so no special handling is needed.
func (c *Client) Delete(ctx context.Context, entityType, entityID string) error {
	_, err := c.es.Delete(c.index, docID(entityType, entityID)).Do(ctx)
	if err != nil {
		return fmt.Errorf("deleting document %s:%s: %w", entityType, entityID, err)
	}

	return nil
}

// Search executes a search query against Elasticsearch.
func (c *Client) Search(ctx context.Context, filter search.Filter) ([]*search.Result, int, *search.Facets, error) {
	body := buildSearchQuery(filter)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("marshaling search query: %w", err)
	}

	res, err := c.es.Search().Index(c.index).Raw(bytes.NewReader(data)).Do(ctx)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("executing search: %w", err)
	}

	results := make([]*search.Result, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		result := hitToResult(hit)
		results = append(results, result)
	}

	facets := extractFacets(res.Aggregations)

	var total int
	if res.Hits.Total != nil {
		total = int(res.Hits.Total.Value)
	}

	return results, total, facets, nil
}

// CreateIndex creates the Elasticsearch index with mappings.
func (c *Client) CreateIndex(ctx context.Context) error {
	exists, err := c.es.Indices.Exists(c.index).Do(ctx)
	if err != nil {
		return fmt.Errorf("checking index existence: %w", err)
	}

	if exists {
		return nil
	}

	_, err = c.es.Indices.Create(c.index).
		Settings(c.buildIndexSettings()).
		Mappings(buildTypeMappings()).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("creating index: %w", err)
	}

	return nil
}

// documentToMap converts a SearchDocument to a map for JSON serialization.
func documentToMap(doc search.SearchDocument) map[string]interface{} {
	m := map[string]interface{}{
		"type":       doc.Type,
		"entity_id":  doc.EntityID,
		"name":       doc.Name,
		"url_path":   doc.URLPath,
		"updated_at": doc.UpdatedAt.Format(time.RFC3339),
	}

	if doc.Description != nil {
		m["description"] = *doc.Description
	}
	if doc.AssetType != nil {
		m["asset_type"] = *doc.AssetType
	}
	if doc.PrimaryProvider != nil {
		m["primary_provider"] = *doc.PrimaryProvider
	}
	if len(doc.Providers) > 0 {
		m["providers"] = doc.Providers
	}
	if len(doc.Tags) > 0 {
		m["tags"] = doc.Tags
	}
	if doc.MRN != nil {
		m["mrn"] = *doc.MRN
	}
	if doc.CreatedBy != nil {
		m["created_by"] = *doc.CreatedBy
	}
	if doc.CreatedAt != nil {
		m["created_at"] = doc.CreatedAt.Format(time.RFC3339)
	}
	if doc.Documentation != nil {
		m["documentation"] = *doc.Documentation
	}
	if len(doc.Metadata) > 0 {
		flat := make(map[string]interface{}, len(doc.Metadata))
		for k, v := range doc.Metadata {
			// Replace dots in keys — ES interprets them as nested objects,
			// which conflicts with the text dynamic template.
			safeKey := strings.ReplaceAll(k, ".", "_")
			flat[safeKey] = fmt.Sprintf("%v", v)
		}
		m["metadata"] = flat
	}

	return m
}

func hitToResult(hit types.Hit) *search.Result {
	var source map[string]interface{}
	if hit.Source_ != nil {
		_ = json.Unmarshal(hit.Source_, &source)
	}
	if source == nil {
		source = map[string]interface{}{}
	}

	var rank float32
	if hit.Score_ != nil {
		rank = float32(*hit.Score_)
	}

	result := &search.Result{
		Type: search.ResultType(getString(source, "type")),
		ID:   getString(source, "entity_id"),
		Name: getString(source, "name"),
		URL:  getString(source, "url_path"),
		Rank: rank,
	}

	if desc, ok := source["description"].(string); ok {
		result.Description = &desc
	}

	if updatedStr, ok := source["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			result.UpdatedAt = &t
		}
	}

	// Build metadata to match PG result format
	metadata := make(map[string]interface{})
	metadata["id"] = result.ID
	metadata["name"] = result.Name
	if result.Description != nil {
		metadata["description"] = *result.Description
	}
	if result.UpdatedAt != nil {
		metadata["updated_at"] = *result.UpdatedAt
	}
	if v, ok := source["asset_type"].(string); ok {
		metadata["type"] = v
	}
	if v, ok := source["primary_provider"].(string); ok {
		metadata["primary_provider"] = v
	}
	if v := getStringSlice(source, "providers"); len(v) > 0 {
		metadata["providers"] = v
	}
	if v := getStringSlice(source, "tags"); len(v) > 0 {
		metadata["tags"] = v
	}
	if v, ok := source["mrn"].(string); ok {
		metadata["mrn"] = v
	}
	if v, ok := source["created_by"].(string); ok {
		metadata["created_by"] = v
	}
	result.Metadata = metadata

	return result
}

func extractFacets(aggs map[string]types.Aggregate) *search.Facets {
	facets := &search.Facets{
		Types:      make(map[search.ResultType]int),
		AssetTypes: []search.FacetValue{},
		Providers:  []search.FacetValue{},
		Tags:       []search.FacetValue{},
	}

	extractBuckets := func(agg types.Aggregate) (keys []string, counts []int) {
		m, ok := agg.(map[string]any)
		if !ok {
			return nil, nil
		}
		buckets, ok := m["buckets"].([]any)
		if !ok {
			return nil, nil
		}
		for _, b := range buckets {
			bucket, ok := b.(map[string]any)
			if !ok {
				continue
			}
			key, _ := bucket["key"].(string)
			docCount, _ := bucket["doc_count"].(float64)
			keys = append(keys, key)
			counts = append(counts, int(docCount))
		}
		return keys, counts
	}

	if agg, ok := aggs["types"]; ok {
		keys, counts := extractBuckets(agg)
		for i, key := range keys {
			facets.Types[search.ResultType(key)] = counts[i]
		}
	}
	if agg, ok := aggs["asset_types"]; ok {
		keys, counts := extractBuckets(agg)
		for i, key := range keys {
			facets.AssetTypes = append(facets.AssetTypes, search.FacetValue{Value: key, Count: counts[i]})
		}
	}
	if agg, ok := aggs["providers"]; ok {
		keys, counts := extractBuckets(agg)
		for i, key := range keys {
			facets.Providers = append(facets.Providers, search.FacetValue{Value: key, Count: counts[i]})
		}
	}
	if agg, ok := aggs["tags"]; ok {
		keys, counts := extractBuckets(agg)
		for i, key := range keys {
			facets.Tags = append(facets.Tags, search.FacetValue{Value: key, Count: counts[i]})
		}
	}

	return facets
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
