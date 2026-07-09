package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// docID returns the document ID for a given type and entity ID.
func docID(entityType, entityID string) string {
	return entityType + ":" + entityID
}

// Index indexes a single document.
func (c *Client) Index(ctx context.Context, doc search.SearchDocument) error {
	body, err := json.Marshal(documentToMap(doc))
	if err != nil {
		return fmt.Errorf("marshaling document %s:%s: %w", doc.Type, doc.EntityID, err)
	}

	_, err = c.es.Index(ctx, opensearchapi.IndexReq{
		Index:      c.index,
		DocumentID: docID(doc.Type, doc.EntityID),
		Body:       bytes.NewReader(body),
	})
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

	var buf bytes.Buffer
	for _, doc := range docs {
		id := docID(doc.Type, doc.EntityID)
		meta, err := json.Marshal(map[string]any{
			"index": map[string]any{"_id": id},
		})
		if err != nil {
			return fmt.Errorf("marshaling bulk meta for %s: %w", id, err)
		}
		body, err := json.Marshal(documentToMap(doc))
		if err != nil {
			return fmt.Errorf("marshaling document %s: %w", id, err)
		}
		buf.Write(meta)
		buf.WriteByte('\n')
		buf.Write(body)
		buf.WriteByte('\n')
	}

	res, err := c.es.Bulk(ctx, opensearchapi.BulkReq{
		Index: c.index,
		Body:  &buf,
	})
	if err != nil {
		return fmt.Errorf("bulk indexing: %w", err)
	}

	if res.Errors {
		var failed []string
		for _, item := range res.Items {
			for _, op := range item {
				if op.Error != nil {
					failed = append(failed, fmt.Sprintf("%s: %s", op.ID, op.Error.Reason))
				}
			}
		}
		return fmt.Errorf("bulk indexing completed with errors: %s", strings.Join(failed, "; "))
	}

	return nil
}

// Delete removes a document from the index. A 404 is treated as success.
func (c *Client) Delete(ctx context.Context, entityType, entityID string) error {
	resp, err := c.es.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      c.index,
		DocumentID: docID(entityType, entityID),
	})
	if err != nil {
		if resp != nil && resp.Inspect().Response != nil && resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("deleting document %s:%s: %w", entityType, entityID, err)
	}

	return nil
}

// Search executes a search query.
func (c *Client) Search(ctx context.Context, filter search.Filter) ([]*search.Result, int, *search.Facets, error) {
	body, err := json.Marshal(buildSearchQuery(filter))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("marshaling search query: %w", err)
	}

	res, err := c.es.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{c.index},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return nil, 0, nil, fmt.Errorf("executing search: %w", err)
	}

	results := make([]*search.Result, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		results = append(results, hitToResult(hit))
	}

	facets := extractFacets(res.Aggregations)
	total := res.Hits.Total.Value

	return results, total, facets, nil
}

// CreateIndex creates the search index with mappings and settings if it does
// not already exist.
func (c *Client) CreateIndex(ctx context.Context) error {
	existsResp, err := c.es.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{c.index},
	})
	if existsResp != nil && existsResp.StatusCode == http.StatusOK {
		return nil
	}
	if err != nil && (existsResp == nil || existsResp.StatusCode != http.StatusNotFound) {
		return fmt.Errorf("checking index existence: %w", err)
	}

	createBody, err := json.Marshal(map[string]any{
		"settings": c.buildIndexSettings(),
		"mappings": buildTypeMappings(),
	})
	if err != nil {
		return fmt.Errorf("marshaling index body: %w", err)
	}

	_, err = c.es.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: c.index,
		Body:  bytes.NewReader(createBody),
	})
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
			// Replace dots in keys — ES/OS interpret them as nested objects,
			// which conflicts with the text dynamic template.
			safeKey := strings.ReplaceAll(k, ".", "_")
			flat[safeKey] = fmt.Sprintf("%v", v)
		}
		m["metadata"] = flat
	}

	return m
}

func hitToResult(hit opensearchapi.SearchHit) *search.Result {
	source := map[string]interface{}{}
	if len(hit.Source) > 0 {
		_ = json.Unmarshal(hit.Source, &source)
	}

	result := &search.Result{
		Type: search.ResultType(getString(source, "type")),
		ID:   getString(source, "entity_id"),
		Name: getString(source, "name"),
		URL:  getString(source, "url_path"),
		Rank: hit.Score,
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

// termsAggregation matches the response shape of a `terms` aggregation.
type termsAggregation struct {
	Buckets []struct {
		Key      string `json:"key"`
		DocCount int    `json:"doc_count"`
	} `json:"buckets"`
}

func extractFacets(raw json.RawMessage) *search.Facets {
	facets := &search.Facets{
		Types:      make(map[search.ResultType]int),
		AssetTypes: []search.FacetValue{},
		Providers:  []search.FacetValue{},
		Tags:       []search.FacetValue{},
	}

	if len(raw) == 0 {
		return facets
	}

	var aggs struct {
		Types      termsAggregation `json:"types"`
		AssetTypes termsAggregation `json:"asset_types"`
		Providers  termsAggregation `json:"providers"`
		Tags       termsAggregation `json:"tags"`
	}
	if err := json.Unmarshal(raw, &aggs); err != nil {
		return facets
	}

	for _, b := range aggs.Types.Buckets {
		facets.Types[search.ResultType(b.Key)] = b.DocCount
	}
	for _, b := range aggs.AssetTypes.Buckets {
		facets.AssetTypes = append(facets.AssetTypes, search.FacetValue{Value: b.Key, Count: b.DocCount})
	}
	for _, b := range aggs.Providers.Buckets {
		facets.Providers = append(facets.Providers, search.FacetValue{Value: b.Key, Count: b.DocCount})
	}
	for _, b := range aggs.Tags.Buckets {
		facets.Tags = append(facets.Tags, search.FacetValue{Value: b.Key, Count: b.DocCount})
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
