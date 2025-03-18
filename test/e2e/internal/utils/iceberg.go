package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/google/uuid"
)

// TestIcebergTable represents an Iceberg test table
type TestIcebergTable struct {
	Name        string
	Namespace   string
	Partitioned bool
	Fields      []TestIcebergField
	Tags        map[string]string
}

// TestIcebergField represents a field in an Iceberg table schema
type TestIcebergField struct {
	ID       int
	Name     string
	Type     string
	Required bool
}

// CreateTestIcebergTablesInREST creates Iceberg tables in a REST catalog
func CreateTestIcebergTablesInREST(ctx context.Context, restEndpoint string, tables []TestIcebergTable) error {
	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// First ensure the namespace exists
	if len(tables) > 0 {
		namespaces := make(map[string]bool)
		for _, table := range tables {
			namespaces[table.Namespace] = true
		}

		for namespace := range namespaces {
			if err := createNamespaceInREST(ctx, client, restEndpoint, namespace); err != nil {
				return fmt.Errorf("creating namespace %s: %w", namespace, err)
			}
		}
	}

	// Create each table
	for _, table := range tables {
		if err := createTableInREST(ctx, client, restEndpoint, table); err != nil {
			return fmt.Errorf("creating table %s.%s: %w", table.Namespace, table.Name, err)
		}
	}

	return nil
}

// createNamespaceInREST creates a namespace in the REST catalog
func createNamespaceInREST(ctx context.Context, client *http.Client, endpoint, namespace string) error {
	url := fmt.Sprintf("http://%s/v1/namespaces", endpoint)

	// Create request payload
	payload := map[string]interface{}{
		"namespace": []string{namespace},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling namespace payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("creating namespace request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending namespace request: %w", err)
	}
	defer resp.Body.Close()

	// If namespace already exists, that's fine (409 Conflict)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// createTableInREST creates a table in the REST catalog
func createTableInREST(ctx context.Context, client *http.Client, endpoint string, table TestIcebergTable) error {
	url := fmt.Sprintf("http://%s/v1/namespaces/%s/tables", endpoint, table.Namespace)

	// Create schema fields with proper type handling
	fields := make([]map[string]interface{}, len(table.Fields))
	for i, field := range table.Fields {
		// Process complex types
		fieldType := processComplexType(field.Type, field.ID)

		fields[i] = map[string]interface{}{
			"id":       field.ID,
			"name":     field.Name,
			"type":     fieldType,
			"required": field.Required,
		}
	}

	// Create request payload
	payload := map[string]interface{}{
		"name": table.Name,
		"schema": map[string]interface{}{
			"type":                 "struct",
			"schema-id":            0,
			"identifier-field-ids": []int{},
			"fields":               fields,
		},
		"properties": map[string]string{
			"format-version":                   "2",
			"write.format.default":             "parquet",
			"write.parquet.compression-codec":  "snappy",
			"write.object-storage.enabled":     "true",
			"write.metadata.compression-codec": "gzip",
			"write.summary.partition-limit":    "100",
			"commit.retry.num-retries":         "3",
		},
	}

	// Add partition spec if the table is partitioned
	if table.Partitioned {
		// Create partition spec in the format expected by REST API
		partitionSpec := map[string]interface{}{
			"spec-id": 0,
			"fields":  []map[string]interface{}{},
		}

		// Find suitable fields for partitioning based on their type
		for _, field := range table.Fields {
			// Skip complex types for partitioning
			if strings.Contains(field.Type, "<") {
				continue
			}

			if strings.ToLower(field.Type) == "string" {
				partitionSpec["fields"] = append(partitionSpec["fields"].([]map[string]interface{}), map[string]interface{}{
					"name":      field.Name,
					"transform": "identity",
					"source-id": field.ID,
					"field-id":  1000 + field.ID,
				})
			} else if strings.ToLower(field.Type) == "date" || strings.Contains(strings.ToLower(field.Type), "timestamp") {
				partitionSpec["fields"] = append(partitionSpec["fields"].([]map[string]interface{}), map[string]interface{}{
					"name":      fmt.Sprintf("%s_month", field.Name),
					"transform": "month",
					"source-id": field.ID,
					"field-id":  1000 + field.ID,
				})
			} else if strings.ToLower(field.Type) == "int" || strings.ToLower(field.Type) == "long" {
				partitionSpec["fields"] = append(partitionSpec["fields"].([]map[string]interface{}), map[string]interface{}{
					"name":      fmt.Sprintf("%s_bucket", field.Name),
					"transform": "bucket[4]",
					"source-id": field.ID,
					"field-id":  1000 + field.ID,
				})
			}
		}

		// Only add if we have partition fields
		if len(partitionSpec["fields"].([]map[string]interface{})) > 0 {
			payload["partition-spec"] = partitionSpec
		}
	}

	// Add sort order for better performance
	sortFields := make([]map[string]interface{}, 0)
	for _, field := range table.Fields {
		if field.Required && !strings.Contains(field.Type, "<") {
			// Add required fields to sort order (skip complex types)
			sortFields = append(sortFields, map[string]interface{}{
				"source-id":  field.ID,
				"direction":  "asc",
				"null-order": "nulls-first",
				"transform":  "identity",
			})
		}
	}

	if len(sortFields) > 0 {
		payload["sort-order"] = map[string]interface{}{
			"order-id": 1,
			"fields":   sortFields,
		}
	}

	// Add tags as properties
	if len(table.Tags) > 0 {
		properties := payload["properties"].(map[string]string)
		for k, v := range table.Tags {
			properties[fmt.Sprintf("tag.%s", k)] = v
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling table payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("creating table request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending table request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// processComplexType processes complex types into the format expected by Iceberg REST API
func processComplexType(fieldType string, baseId int) interface{} {
	// If it's a map type like "map<string, string>"
	if strings.HasPrefix(fieldType, "map<") {
		// Extract key and value types
		mapContent := strings.TrimPrefix(fieldType, "map<")
		mapContent = strings.TrimSuffix(mapContent, ">")
		parts := strings.Split(mapContent, ",")

		if len(parts) != 2 {
			// Invalid map type, return as is
			return fieldType
		}

		keyType := strings.TrimSpace(parts[0])
		valueType := strings.TrimSpace(parts[1])

		// Use unique IDs based on the baseId
		keyId := baseId*10 + 1
		valueId := baseId*10 + 2

		return map[string]interface{}{
			"type":           "map",
			"key-id":         keyId,
			"key":            processComplexType(keyType, keyId),
			"value-id":       valueId,
			"value-required": false,
			"value":          processComplexType(valueType, valueId),
		}
	}

	// If it's a struct type
	if strings.HasPrefix(fieldType, "struct<") {
		structContent := strings.TrimPrefix(fieldType, "struct<")
		structContent = strings.TrimSuffix(structContent, ">")

		fieldParts := strings.Split(structContent, ",")
		structFields := make([]map[string]interface{}, 0, len(fieldParts))

		for i, part := range fieldParts {
			fieldPart := strings.Split(part, ":")
			if len(fieldPart) != 2 {
				// Invalid struct field, skip
				continue
			}

			fieldName := strings.TrimSpace(fieldPart[0])
			fieldType := strings.TrimSpace(fieldPart[1])

			// Use unique ID for each nested field
			fieldId := baseId*10 + i + 1

			structFields = append(structFields, map[string]interface{}{
				"id":       fieldId,
				"name":     fieldName,
				"type":     processComplexType(fieldType, fieldId),
				"required": false,
			})
		}

		return map[string]interface{}{
			"type":   "struct",
			"fields": structFields,
		}
	}

	// If it's an array type
	if strings.HasPrefix(fieldType, "array<") {
		arrayContent := strings.TrimPrefix(fieldType, "array<")
		arrayContent = strings.TrimSuffix(arrayContent, ">")

		// Use unique ID for element
		elemId := baseId*10 + 1

		return map[string]interface{}{
			"type":             "list",
			"element-id":       elemId,
			"element-required": true,
			"element":          processComplexType(arrayContent, elemId),
		}
	}

	// For primitive types, return as string
	return fieldType
}

// CreateTestIcebergTablesInGlue creates test Iceberg tables in AWS Glue catalog using Localstack
func CreateTestIcebergTablesInGlue(ctx context.Context, endpoint string, region string, tables []TestIcebergTable) error {
	// Set up AWS config for Localstack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           endpoint,
					SigningRegion: region,
				}, nil
			})),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create Glue client
	glueClient := glue.NewFromConfig(cfg)

	// Process each table
	for _, table := range tables {
		// Create database if it doesn't exist
		_, err = glueClient.CreateDatabase(ctx, &glue.CreateDatabaseInput{
			DatabaseInput: &types.DatabaseInput{
				Name: aws.String(table.Namespace),
			},
		})
		if err != nil && !strings.Contains(err.Error(), "AlreadyExistsException") {
			return fmt.Errorf("failed to create database %s: %w", table.Namespace, err)
		}

		// Create columns for table
		columns := make([]types.Column, len(table.Fields))
		for i, field := range table.Fields {
			columns[i] = types.Column{
				Name:    aws.String(field.Name),
				Type:    aws.String(convertToGlueType(field.Type)),
				Comment: aws.String(fmt.Sprintf("Field %d", field.ID)),
			}
		}

		// Create partition keys if table is partitioned
		var partitionKeys []types.Column
		if table.Partitioned {
			// For testing, we'll partition by the first string field
			for _, field := range table.Fields {
				if strings.Contains(strings.ToLower(field.Type), "string") ||
					strings.Contains(strings.ToLower(field.Type), "date") {
					partitionKeys = append(partitionKeys, types.Column{
						Name: aws.String(field.Name),
						Type: aws.String(convertToGlueType(field.Type)),
					})
					break
				}
			}

			// If no suitable field was found, default to the first field
			if len(partitionKeys) == 0 && len(columns) > 0 {
				partitionKeys = append(partitionKeys, columns[0])
			}
		}

		// Table location in S3 - using a placeholder since Localstack supports this
		tableLocation := fmt.Sprintf("s3://iceberg-warehouse/%s/%s", table.Namespace, table.Name)

		// Iceberg specific parameters
		tableParams := map[string]string{
			"table_type":          "ICEBERG",
			"format-version":      "2",
			"uuid":                uuid.New().String(),
			"last-updated-ms":     fmt.Sprintf("%d", time.Now().Unix()*1000),
			"current-schema-id":   "0",
			"current-snapshot-id": "1",
		}

		// Add tags to parameters
		for k, v := range table.Tags {
			tableParams[fmt.Sprintf("tag.%s", k)] = v
		}

		// Create table input
		tableInput := &types.TableInput{
			Name:       aws.String(table.Name),
			TableType:  aws.String("EXTERNAL_TABLE"),
			Parameters: tableParams,
			StorageDescriptor: &types.StorageDescriptor{
				Columns:      columns,
				Location:     aws.String(tableLocation),
				InputFormat:  aws.String("org.apache.hadoop.mapred.FileInputFormat"),
				OutputFormat: aws.String("org.apache.hadoop.mapred.FileOutputFormat"),
				SerdeInfo: &types.SerDeInfo{
					SerializationLibrary: aws.String("org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"),
					Parameters:           map[string]string{},
				},
			},
			PartitionKeys: partitionKeys,
		}

		// Create the table
		_, err = glueClient.CreateTable(ctx, &glue.CreateTableInput{
			DatabaseName: aws.String(table.Namespace),
			TableInput:   tableInput,
		})
		if err != nil && !strings.Contains(err.Error(), "AlreadyExistsException") {
			return fmt.Errorf("failed to create table %s.%s: %w", table.Namespace, table.Name, err)
		}

		// Add tags to the table
		if len(table.Tags) > 0 {
			tagMap := make(map[string]string, len(table.Tags))
			for k, v := range table.Tags {
				tagMap[k] = v
			}

			_, err = glueClient.TagResource(ctx, &glue.TagResourceInput{
				ResourceArn: aws.String(fmt.Sprintf("arn:aws:glue:%s:000000000000:table/%s/%s",
					region, table.Namespace, table.Name)),
				TagsToAdd: tagMap,
			})
			if err != nil {
				fmt.Printf("Warning: failed to tag table %s.%s: %v\n", table.Namespace, table.Name, err)
				// Continue despite tagging errors
			}
		}
	}

	return nil
}

// convertToGlueType converts Iceberg data types to Glue data types
func convertToGlueType(icebergType string) string {
	icebergType = strings.ToLower(icebergType)

	switch {
	case icebergType == "int":
		return "int"
	case icebergType == "long":
		return "bigint"
	case icebergType == "float":
		return "float"
	case icebergType == "double":
		return "double"
	case icebergType == "boolean":
		return "boolean"
	case icebergType == "string":
		return "string"
	case icebergType == "date":
		return "date"
	case icebergType == "timestamp":
		return "timestamp"
	case icebergType == "binary":
		return "binary"
	case strings.HasPrefix(icebergType, "decimal"):
		return icebergType // Glue supports the same decimal(p,s) format
	case strings.HasPrefix(icebergType, "struct<"):
		return "struct<" + strings.TrimPrefix(icebergType, "struct<")
	case strings.HasPrefix(icebergType, "map<"):
		return "map<" + strings.TrimPrefix(icebergType, "map<")
	case strings.HasPrefix(icebergType, "array<"):
		return "array<" + strings.TrimPrefix(icebergType, "array<")
	default:
		return "string" // Default to string for unsupported types
	}
}
