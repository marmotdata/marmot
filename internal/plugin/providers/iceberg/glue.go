package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// GlueConfig for AWS Glue catalog
type GlueConfig struct {
	Region             string `json:"region" yaml:"region" description:"AWS region"`
	Database           string `json:"database,omitempty" yaml:"database,omitempty" description:"Default Glue database for narrowing search scope"`
	AccessKey          string `json:"access_key,omitempty" yaml:"access_key,omitempty" description:"AWS access key"`
	SecretKey          string `json:"secret_key,omitempty" yaml:"secret_key,omitempty" description:"AWS secret key"`
	CredentialsProfile string `json:"credentials_profile,omitempty" yaml:"credentials_profile,omitempty" description:"AWS credentials profile name"`
	AssumeRoleARN      string `json:"assume_role_arn,omitempty" yaml:"assume_role_arn,omitempty" description:"AWS role ARN to assume"`
	Endpoint           string `json:"endpoint,omitempty" yaml:"endpoint,omitempty" description:"Optional custom endpoint for Glue service"`
}

func (s *Source) initGlueClient(ctx context.Context) error {
	// Get AWS config (region, credentials, etc.)
	awsConfig, err := s.getAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("getting AWS config: %w", err)
	}

	// Create Glue client
	glueClient := glue.NewFromConfig(awsConfig)
	s.client = glueClient

	return nil
}

func (s *Source) getAWSConfig(ctx context.Context) (aws.Config, error) {
	// Start with config options for region and credentials
	var opts []func(*config.LoadOptions) error

	// Configure region
	if s.config.Glue.Region != "" {
		opts = append(opts, config.WithRegion(s.config.Glue.Region))
	}

	// Configure credentials if access key and secret key are provided
	if s.config.Glue.AccessKey != "" && s.config.Glue.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				s.config.Glue.AccessKey,
				s.config.Glue.SecretKey,
				"", // Optional session token
			),
		))
	}

	// Load specific profile if specified
	if s.config.Glue.CredentialsProfile != "" {
		opts = append(opts, config.WithSharedConfigProfile(s.config.Glue.CredentialsProfile))
	}

	// Load the AWS config
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	// Configure custom endpoint if provided
	// TODO: remove the deprecated here, use generic AWS config for all AWS plugins
	if s.config.Glue.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == glue.ServiceID {
				return aws.Endpoint{
					URL:           s.config.Glue.Endpoint,
					SigningRegion: region,
				}, nil
			}
			// Fallback to default resolver for other services
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})
		cfg.EndpointResolverWithOptions = customResolver
	}

	// Assume role if an ARN is provided
	if s.config.Glue.AssumeRoleARN != "" {
		// Create STS client using the base config
		stsClient := sts.NewFromConfig(cfg)

		// Assume role
		resp, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(s.config.Glue.AssumeRoleARN),
			RoleSessionName: aws.String("IcebergDiscoverySession"),
		})
		if err != nil {
			return aws.Config{}, fmt.Errorf("assuming role: %w", err)
		}

		// Create new credentials provider with the temporary credentials
		cfg.Credentials = credentials.NewStaticCredentialsProvider(
			*resp.Credentials.AccessKeyId,
			*resp.Credentials.SecretAccessKey,
			*resp.Credentials.SessionToken,
		)
	}

	return cfg, nil
}

func (s *Source) discoverGlueDatabases(ctx context.Context) ([]string, error) {
	glueClient := s.client.(*glue.Client)

	// If a specific database is configured, return just that
	if s.config.Glue.Database != "" {
		// Verify the database exists
		_, err := glueClient.GetDatabase(ctx, &glue.GetDatabaseInput{
			Name: aws.String(s.config.Glue.Database),
		})
		if err != nil {
			return nil, fmt.Errorf("database %s not found: %w", s.config.Glue.Database, err)
		}
		return []string{s.config.Glue.Database}, nil
	}

	// List all databases in Glue catalog
	var databases []string
	var nextToken *string

	for {
		resp, err := glueClient.GetDatabases(ctx, &glue.GetDatabasesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing Glue databases: %w", err)
		}

		for _, db := range resp.DatabaseList {
			databases = append(databases, *db.Name)
		}

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}

	return databases, nil
}

func (s *Source) discoverGlueTables(ctx context.Context, database string) ([]string, error) {
	glueClient := s.client.(*glue.Client)

	// List all tables in the specified database
	var tables []string
	var nextToken *string

	for {
		resp, err := glueClient.GetTables(ctx, &glue.GetTablesInput{
			DatabaseName: aws.String(database),
			NextToken:    nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing tables in database %s: %w", database, err)
		}

		for _, table := range resp.TableList {
			// Check if this is an Iceberg table by looking at parameters
			if table.Parameters != nil {
				if format, ok := table.Parameters["table_type"]; ok && strings.EqualFold(format, "iceberg") {
					tables = append(tables, *table.Name)
				}
			}
		}

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}

	return tables, nil
}

func (s *Source) getGlueTableMetadata(ctx context.Context, database, table string) (*IcebergMetadata, error) {
	glueClient := s.client.(*glue.Client)

	// Get table metadata
	tableResp, err := glueClient.GetTable(ctx, &glue.GetTableInput{
		DatabaseName: aws.String(database),
		Name:         aws.String(table),
	})
	if err != nil {
		return nil, fmt.Errorf("getting table metadata: %w", err)
	}

	if tableResp.Table == nil {
		return nil, fmt.Errorf("table not found: %s.%s", database, table)
	}

	glueTable := tableResp.Table

	// Verify this is an Iceberg table
	isIceberg := false
	if glueTable.Parameters != nil {
		if format, ok := glueTable.Parameters["table_type"]; ok && strings.EqualFold(format, "iceberg") {
			isIceberg = true
		}
	}

	if !isIceberg {
		return nil, fmt.Errorf("table %s.%s is not an Iceberg table", database, table)
	}

	// Create metadata object
	metadata := &IcebergMetadata{
		Identifier:  fmt.Sprintf("%s.%s", database, table),
		Namespace:   database,
		TableName:   *glueTable.Name,
		CatalogType: "glue",
	}

	// Extract location
	if glueTable.StorageDescriptor != nil && glueTable.StorageDescriptor.Location != nil {
		metadata.Location = *glueTable.StorageDescriptor.Location
	}

	// Extract properties from Glue table parameters
	if glueTable.Parameters != nil {
		metadata.Properties = make(map[string]string)

		// Copy all parameters to properties
		for k, v := range glueTable.Parameters {
			metadata.Properties[k] = v
		}

		// Extract format version
		if formatVersion, ok := glueTable.Parameters["format-version"]; ok {
			if version, err := parseInt(formatVersion); err == nil {
				metadata.FormatVersion = version
			}
		}

		// Extract UUID
		if uuid, ok := glueTable.Parameters["uuid"]; ok {
			metadata.UUID = uuid
		}

		// Extract snapshot info if included
		if s.config.IncludeSnapshotInfo {
			// Current snapshot ID
			if snapshotID, ok := glueTable.Parameters["current-snapshot-id"]; ok {
				if id, err := parseInt64(snapshotID); err == nil {
					metadata.CurrentSnapshotID = id
				}
			}

			// Last updated timestamp
			if lastUpdated, ok := glueTable.Parameters["last-updated-ms"]; ok {
				if ms, err := parseInt64(lastUpdated); err == nil {
					metadata.LastUpdatedMs = ms
				}
			}

			// Number of snapshots - harder to get from just the parameters
			// Would need to read the actual metadata file
		}

		// Extract statistics if included and available
		if s.config.IncludeStatistics {
			if rowCount, ok := glueTable.Parameters["total-records"]; ok {
				if count, err := parseInt64(rowCount); err == nil {
					metadata.NumRows = count
				}
			}

			if fileSize, ok := glueTable.Parameters["total-files-size"]; ok {
				if size, err := parseInt64(fileSize); err == nil {
					metadata.FileSizeBytes = size
				}
			}

			if dataFiles, ok := glueTable.Parameters["total-data-files"]; ok {
				if count, err := parseInt(dataFiles); err == nil {
					metadata.NumDataFiles = count
				}
			}
		}
	}

	// Extract schema info if included
	if s.config.IncludeSchemaInfo && glueTable.StorageDescriptor != nil {
		// Convert Glue columns to Iceberg schema
		if len(glueTable.StorageDescriptor.Columns) > 0 {
			schema := extractSchemaFromGlueColumns(glueTable.StorageDescriptor.Columns)
			if schema != nil {
				schemaJSON, err := json.Marshal(schema)
				if err == nil {
					metadata.SchemaJSON = string(schemaJSON)
				}
			}
		}

		// Extract partition info if included
		if s.config.IncludePartitionInfo && len(glueTable.PartitionKeys) > 0 {
			partSpec := extractPartitionSpecFromGluePartitionKeys(glueTable.PartitionKeys)
			if partSpec != nil {
				partSpecJSON, err := json.Marshal(partSpec)
				if err == nil {
					metadata.PartitionSpec = string(partSpecJSON)
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
		}
	}

	return metadata, nil
}

// Helper function to convert Glue columns to Iceberg schema
func extractSchemaFromGlueColumns(columns []types.Column) map[string]interface{} {
	if len(columns) == 0 {
		return nil
	}

	fields := make([]map[string]interface{}, 0, len(columns))

	for i, col := range columns {
		field := map[string]interface{}{
			"id":   i + 1, // Iceberg uses 1-based IDs
			"name": *col.Name,
			"type": convertGlueTypeToIcebergType(*col.Type),
		}

		// In AWS SDK v2, IsNullable is not a method but may be in the Parameters map
		// Default to nullable (required: false) unless explicitly marked as non-nullable
		isRequired := false
		if val, exists := col.Parameters["NULLABLE"]; exists && strings.ToLower(val) == "false" {
			isRequired = true
		}
		field["required"] = isRequired

		// Add comment if available
		if col.Comment != nil {
			field["doc"] = *col.Comment
		}

		fields = append(fields, field)
	}

	return map[string]interface{}{
		"type":   "struct",
		"fields": fields,
	}
}

// Helper function to convert Glue partition keys to Iceberg partition spec
func extractPartitionSpecFromGluePartitionKeys(partitionKeys []types.Column) []map[string]interface{} {
	if len(partitionKeys) == 0 {
		return nil
	}

	partSpec := make([]map[string]interface{}, 0, len(partitionKeys))

	for i, key := range partitionKeys {
		spec := map[string]interface{}{
			"source-id": i + 1,    // Match with field ID from schema
			"field-id":  1000 + i, // Convention for partition field IDs
			"name":      *key.Name,
			"transform": "identity", // Default transform is identity
		}

		partSpec = append(partSpec, spec)
	}

	return partSpec
}

// Helper function to convert Glue data types to Iceberg types
func convertGlueTypeToIcebergType(glueType string) string {
	// This is a simplified mapping, would need to be expanded for complex types
	switch strings.ToLower(glueType) {
	case "string":
		return "string"
	case "int", "integer", "smallint", "tinyint":
		return "int"
	case "bigint":
		return "long"
	case "double", "float", "decimal":
		return "double"
	case "boolean", "bool":
		return "boolean"
	case "timestamp":
		return "timestamp"
	case "date":
		return "date"
	case "binary":
		return "binary"
	default:
		// For complex types like maps, arrays, structs
		// would need more sophisticated parsing
		return glueType
	}
}

// Helper function to parse string to int
func parseInt(s string) (int, error) {
	var v int
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

// Helper function to parse string to int64
func parseInt64(s string) (int64, error) {
	var v int64
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

// Helper function to find the current snapshot in metadata
func findCurrentSnapshot(metadata map[string]interface{}) (map[string]interface{}, bool) {
	currentSnapshotID, ok := metadata["current-snapshot-id"].(float64)
	if !ok {
		return nil, false
	}

	snapshots, ok := metadata["snapshots"].([]interface{})
	if !ok {
		return nil, false
	}

	for _, s := range snapshots {
		snapshot, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		snapshotID, ok := snapshot["snapshot-id"].(float64)
		if ok && snapshotID == currentSnapshotID {
			return snapshot, true
		}
	}

	return nil, false
}
