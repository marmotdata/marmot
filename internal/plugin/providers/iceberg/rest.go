package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RESTConfig for REST catalog
type RESTConfig struct {
	URI  string          `json:"uri" yaml:"uri" description:"URI of the REST catalog"`
	Auth *RESTAuthConfig `json:"auth,omitempty" yaml:"auth,omitempty" description:"Authentication configuration"`
}

// RESTAuthConfig for REST catalog authentication
type RESTAuthConfig struct {
	Type         string `json:"type" yaml:"type" description:"Authentication type: none, basic, oauth2, bearer"`
	Username     string `json:"username,omitempty" yaml:"username,omitempty" description:"Username for basic authentication"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty" description:"Password for basic authentication"`
	Token        string `json:"token,omitempty" yaml:"token,omitempty" description:"Token for bearer authentication"`
	ClientID     string `json:"client_id,omitempty" yaml:"client_id,omitempty" description:"Client ID for OAuth2"`
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty" description:"Client secret for OAuth2"`
	TokenURL     string `json:"token_url,omitempty" yaml:"token_url,omitempty" description:"Token URL for OAuth2"`
	CertPath     string `json:"cert_path,omitempty" yaml:"cert_path,omitempty" description:"Path to certificate file"`
}

// RESTNamespaceListResponse represents the response structure for namespace list
type RESTNamespaceListResponse struct {
	Namespaces []RESTNamespaceItem `json:"namespaces"`
}

// RESTNamespaceItem represents a namespace in the REST catalog
type RESTNamespaceItem struct {
	Namespace string `json:"namespace"`
}

// RESTTableListResponse represents the response structure for table list
type RESTTableListResponse struct {
	Identifiers []RESTTableIdentifier `json:"identifiers"`
}

// RESTTableIdentifier represents a table identifier in the REST catalog
type RESTTableIdentifier struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (s *Source) initRESTClient(ctx context.Context) error {
	// Initialize HTTP client with appropriate auth
	httpClient := &http.Client{
		Timeout: time.Second * 30, // Set a reasonable timeout
	}

	// Store in the interface field
	s.client = httpClient

	return nil
}

func (s *Source) addRESTAuth(req *http.Request) error {
	if s.config.REST.Auth == nil {
		return nil
	}

	switch s.config.REST.Auth.Type {
	case "none", "":
		// No authentication
		return nil
	case "basic":
		req.SetBasicAuth(s.config.REST.Auth.Username, s.config.REST.Auth.Password)
		return nil
	case "bearer":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.REST.Auth.Token))
		return nil
	case "oauth2":
		// In a real implementation, this would handle OAuth2 token acquisition
		return fmt.Errorf("oauth2 authentication not implemented yet")
	default:
		return fmt.Errorf("unsupported authentication type: %s", s.config.REST.Auth.Type)
	}
}

func (s *Source) discoverRESTNamespaces(ctx context.Context) ([]string, error) {
	httpClient := s.client.(*http.Client)

	// Make a request to get namespaces - follow Iceberg REST API spec
	uri := fmt.Sprintf("%s/v1/namespaces", strings.TrimSuffix(s.config.REST.URI, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set common headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication if configured
	if err := s.addRESTAuth(req); err != nil {
		return nil, fmt.Errorf("adding authentication: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting namespaces: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Parse the response according to the REST API spec
	var response RESTNamespaceListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// Try alternative format (some implementations return just an array of strings)
		var namespaces []string
		if altErr := json.Unmarshal(body, &namespaces); altErr == nil {
			return namespaces, nil
		}

		// Try another alternative format (some implementations use "namespace" directly)
		var nsItems []RESTNamespaceItem
		if altErr := json.Unmarshal(body, &nsItems); altErr == nil {
			result := make([]string, len(nsItems))
			for i, item := range nsItems {
				result[i] = item.Namespace
			}
			return result, nil
		}

		return nil, fmt.Errorf("parsing namespaces: %w", err)
	}

	// Extract namespace strings
	result := make([]string, len(response.Namespaces))
	for i, item := range response.Namespaces {
		result[i] = item.Namespace
	}

	// If no namespaces were returned, check if there are tables in the default namespace
	if len(result) == 0 {
		// Try to list tables in the default namespace
		tablesURI := fmt.Sprintf("%s/v1/namespaces/default/tables", strings.TrimSuffix(s.config.REST.URI, "/"))
		tablesReq, err := http.NewRequestWithContext(ctx, "GET", tablesURI, nil)
		if err == nil {
			tablesReq.Header.Set("Content-Type", "application/json")
			tablesReq.Header.Set("Accept", "application/json")

			if err := s.addRESTAuth(tablesReq); err == nil {
				tablesResp, err := httpClient.Do(tablesReq)
				if err == nil && tablesResp.StatusCode == http.StatusOK {
					defer tablesResp.Body.Close()
					// If we got a successful response, it means the default namespace exists
					result = append(result, "default")
				}
			}
		}
	}

	return result, nil
}

func (s *Source) discoverRESTTables(ctx context.Context, namespace string) ([]string, error) {
	httpClient := s.client.(*http.Client)

	// Make a request to get tables in namespace - follow Iceberg REST API spec
	uri := fmt.Sprintf("%s/v1/namespaces/%s/tables", strings.TrimSuffix(s.config.REST.URI, "/"), namespace)
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set common headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication if configured
	if err := s.addRESTAuth(req); err != nil {
		return nil, fmt.Errorf("adding authentication: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting tables: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Parse the response according to the REST API spec
	var response RESTTableListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// Try alternative format (some implementations return just an array of strings)
		var tables []string
		if altErr := json.Unmarshal(body, &tables); altErr == nil {
			return tables, nil
		}

		// Try alternative format where namespace is an array
		type altTableIdentifier struct {
			Namespace []string `json:"namespace"`
			Name      string   `json:"name"`
		}
		var altResponse struct {
			Identifiers []altTableIdentifier `json:"identifiers"`
		}
		if altErr := json.Unmarshal(body, &altResponse); altErr == nil {
			result := make([]string, len(altResponse.Identifiers))
			for i, item := range altResponse.Identifiers {
				result[i] = item.Name
			}
			return result, nil
		}

		return nil, fmt.Errorf("parsing tables: %w", err)
	}

	// Extract table names
	result := make([]string, len(response.Identifiers))
	for i, item := range response.Identifiers {
		result[i] = item.Name
	}

	return result, nil
}

func (s *Source) getRESTTableMetadata(ctx context.Context, namespace, table string) (*IcebergMetadata, error) {
	httpClient := s.client.(*http.Client)

	// Make a request to get table metadata - follow Iceberg REST API spec
	uri := fmt.Sprintf("%s/v1/namespaces/%s/tables/%s", strings.TrimSuffix(s.config.REST.URI, "/"), namespace, table)
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set common headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication if configured
	if err := s.addRESTAuth(req); err != nil {
		return nil, fmt.Errorf("adding authentication: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting table metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Parse the response into a map first so we can extract what we need
	var rawMetadata map[string]interface{}
	if err := json.Unmarshal(body, &rawMetadata); err != nil {
		return nil, fmt.Errorf("parsing table metadata: %w", err)
	}

	// Extract relevant fields into our IcebergMetadata struct
	metadata := &IcebergMetadata{
		Identifier:  fmt.Sprintf("%s.%s", namespace, table),
		Namespace:   namespace,
		TableName:   table,
		CatalogType: "rest",
	}

	// Extract location
	if location, ok := rawMetadata["location"].(string); ok {
		metadata.Location = location
	} else if metadataLoc, ok := rawMetadata["metadata-location"].(string); ok {
		// Some implementations use metadata-location instead
		metadata.Location = metadataLoc
	}

	// Extract format version
	if formatVersion, ok := rawMetadata["format-version"].(float64); ok {
		metadata.FormatVersion = int(formatVersion)
	}

	// Extract UUID
	if uuid, ok := rawMetadata["uuid"].(string); ok {
		metadata.UUID = uuid
	}

	// Extract schema info if included
	if s.config.IncludeSchemaInfo {
		// Current schema ID
		if currentSchemaID, ok := rawMetadata["current-schema-id"].(float64); ok {
			metadata.CurrentSchemaID = int(currentSchemaID)
		}

		// Schema JSON
		if schema, ok := rawMetadata["schema"]; ok {
			schemaJSON, err := json.Marshal(schema)
			if err == nil {
				metadata.SchemaJSON = string(schemaJSON)
			}
		}

		// Partition spec
		if partitionSpec, ok := rawMetadata["partition-spec"]; ok {
			partSpecJSON, err := json.Marshal(partitionSpec)
			if err == nil {
				metadata.PartitionSpec = string(partSpecJSON)
			}
		}
	}

	// Extract snapshot info if included
	if s.config.IncludeSnapshotInfo {
		// Current snapshot ID
		if currentSnapshotID, ok := rawMetadata["current-snapshot-id"].(float64); ok {
			metadata.CurrentSnapshotID = int64(currentSnapshotID)
		}

		// Last updated timestamp
		if lastUpdatedMs, ok := rawMetadata["last-updated-ms"].(float64); ok {
			metadata.LastUpdatedMs = int64(lastUpdatedMs)
		} else if lastUpdatedMs, ok := rawMetadata["last-modified-ms"].(float64); ok {
			// Some implementations use last-modified-ms instead
			metadata.LastUpdatedMs = int64(lastUpdatedMs)
		}

		// Number of snapshots
		if snapshots, ok := rawMetadata["snapshots"].([]interface{}); ok {
			metadata.NumSnapshots = len(snapshots)
		}
	}

	// Extract properties if included
	if s.config.IncludeProperties {
		if properties, ok := rawMetadata["properties"].(map[string]interface{}); ok {
			metadata.Properties = make(map[string]string)
			for k, v := range properties {
				if strVal, ok := v.(string); ok {
					metadata.Properties[k] = strVal
				} else {
					// Convert non-string values to string
					jsonVal, err := json.Marshal(v)
					if err == nil {
						metadata.Properties[k] = string(jsonVal)
					}
				}
			}
		}
	}

	// Extract statistics if included
	if s.config.IncludeStatistics {
		if currentSnapshot, ok := findCurrentSnapshot(rawMetadata); ok {
			if summary, ok := currentSnapshot["summary"].(map[string]interface{}); ok {
				if totalRecords, ok := summary["total-records"].(float64); ok {
					metadata.NumRows = int64(totalRecords)
				}
				if totalFilesSizeInBytes, ok := summary["total-files-size"].(float64); ok {
					metadata.FileSizeBytes = int64(totalFilesSizeInBytes)
				}
				if totalDataFiles, ok := summary["total-data-files"].(float64); ok {
					metadata.NumDataFiles = int(totalDataFiles)
				}
				if totalDeleteFiles, ok := summary["total-delete-files"].(float64); ok {
					metadata.NumDeleteFiles = int(totalDeleteFiles)
				}
			}
		}
	}

	// Extract partition information if included
	if s.config.IncludePartitionInfo && metadata.PartitionSpec != "" {
		var partSpec []map[string]interface{}
		if err := json.Unmarshal([]byte(metadata.PartitionSpec), &partSpec); err == nil {
			metadata.NumPartitions = len(partSpec)

			// Extract transformers
			var transformers []string
			for _, p := range partSpec {
				if transform, ok := p["transform"].(string); ok {
					transformers = append(transformers, transform)
				}
			}
			metadata.PartitionTransformers = strings.Join(transformers, ", ")
		}
	}

	// Extract sort order if available
	if sortOrder, ok := rawMetadata["sort-order"]; ok {
		sortOrderJSON, err := json.Marshal(sortOrder)
		if err == nil {
			metadata.SortOrderJSON = string(sortOrderJSON)
		}
	}

	return metadata, nil
}
