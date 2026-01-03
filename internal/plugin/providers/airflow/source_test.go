package airflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      plugin.RawPluginConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with basic auth",
			config: plugin.RawPluginConfig{
				"host":     "http://localhost:8080",
				"username": "admin",
				"password": "admin",
			},
			wantErr: false,
		},
		{
			name: "valid config with api token",
			config: plugin.RawPluginConfig{
				"host":      "http://localhost:8080",
				"api_token": "my-api-token",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: plugin.RawPluginConfig{
				"username": "admin",
				"password": "admin",
			},
			wantErr:     true,
			errContains: "host",
		},
		{
			name: "missing authentication",
			config: plugin.RawPluginConfig{
				"host": "http://localhost:8080",
			},
			wantErr:     true,
			errContains: "authentication required",
		},
		{
			name: "trailing slash removed from host",
			config: plugin.RawPluginConfig{
				"host":     "http://localhost:8080/",
				"username": "admin",
				"password": "admin",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_Discover(t *testing.T) {
	// Create a mock Airflow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/dags":
			response := DAGCollection{
				DAGs: []DAG{
					{
						DagID:    "example_dag",
						Fileloc:  "/opt/airflow/dags/example.py",
						IsPaused: false,
						IsActive: true,
						Owners:   []string{"airflow"},
						Tags:     []Tag{{Name: "example"}},
						ScheduleInterval: &ScheduleInterval{
							Type:  "cron",
							Value: "0 0 * * *",
						},
					},
				},
				TotalCount: 1,
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v1/dags/example_dag/tasks":
			response := TaskCollection{
				Tasks: []Task{
					{
						TaskID:            "task_1",
						OperatorName:      "BashOperator",
						TriggerRule:       "all_success",
						DownstreamTaskIDs: []string{"task_2"},
					},
					{
						TaskID:            "task_2",
						OperatorName:      "PythonOperator",
						TriggerRule:       "all_success",
						DownstreamTaskIDs: []string{},
					},
				},
				TotalCount: 2,
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v1/dags/example_dag/dagRuns":
			response := DAGRunCollection{
				DagRuns: []DAGRun{
					{
						DagRunID:      "run_1",
						DagID:         "example_dag",
						State:         "success",
						ExecutionDate: "2024-01-15T00:00:00+00:00",
					},
				},
				TotalCount: 1,
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v1/datasets":
			response := DatasetCollection{
				Datasets: []Dataset{
					{
						ID:        1,
						URI:       "s3://bucket/data.parquet",
						CreatedAt: "2024-01-01T00:00:00+00:00",
						UpdatedAt: "2024-01-15T00:00:00+00:00",
						ConsumingDags: []DagRef{
							{DagID: "example_dag"},
						},
						ProducingTasks: []TaskRef{
							{DagID: "producer_dag", TaskID: "produce_task"},
						},
					},
				},
				TotalCount: 1,
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := plugin.RawPluginConfig{
		"host":                server.URL,
		"username":            "admin",
		"password":            "admin",
		"discover_dags":       true,
		"discover_tasks":      true,
		"discover_datasets":   true,
		"include_run_history": true,
		"run_history_days":    7,
	}

	s := &Source{}
	result, err := s.Discover(context.Background(), config)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have: 1 DAG (Pipeline) + 2 Tasks + 1 Dataset = 4 assets
	assert.Len(t, result.Assets, 4)

	// Verify Pipeline asset
	var pipelineAsset *struct {
		Name string
		Type string
	}
	for _, a := range result.Assets {
		if a.Type == "Pipeline" {
			pipelineAsset = &struct {
				Name string
				Type string
			}{Name: *a.Name, Type: a.Type}
			break
		}
	}
	require.NotNil(t, pipelineAsset)
	assert.Equal(t, "example_dag", pipelineAsset.Name)

	// Verify Task assets
	taskCount := 0
	for _, a := range result.Assets {
		if a.Type == "Task" {
			taskCount++
		}
	}
	assert.Equal(t, 2, taskCount)

	// Verify Dataset asset (S3 bucket)
	var datasetAsset *struct {
		Name string
		Type string
	}
	for _, a := range result.Assets {
		if a.Type == "Bucket" { // S3 URI creates Bucket type
			datasetAsset = &struct {
				Name string
				Type string
			}{Name: *a.Name, Type: a.Type}
			break
		}
	}
	require.NotNil(t, datasetAsset)
	assert.Equal(t, "bucket", datasetAsset.Name) // Name is just the bucket name

	// Verify lineage edges
	// Should have:
	// - 2 DAG contains task (example_dag -> task_1, example_dag -> task_2)
	// - 1 task dependency (task_1 -> task_2)
	// - 1 dataset triggers DAG (dataset -> example_dag)
	// - 1 task produces dataset (producer_dag.produce_task -> dataset)
	assert.Len(t, result.Lineage, 5)
}

func TestSource_DiscoverWithDAGFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/dags":
			response := DAGCollection{
				DAGs: []DAG{
					{DagID: "analytics_daily", IsActive: true},
					{DagID: "analytics_hourly", IsActive: true},
					{DagID: "test_dag", IsActive: true},
					{DagID: "other_dag", IsActive: true},
				},
				TotalCount: 4,
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v1/datasets":
			response := DatasetCollection{
				Datasets:   []Dataset{},
				TotalCount: 0,
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			if r.URL.Path != "" {
				response := TaskCollection{Tasks: []Task{}, TotalCount: 0}
				_ = json.NewEncoder(w).Encode(response)
			}
		}
	}))
	defer server.Close()

	config := plugin.RawPluginConfig{
		"host":            server.URL,
		"username":        "admin",
		"password":        "admin",
		"discover_dags":   true,
		"discover_tasks":  false,
		"discover_datasets": false,
		"dag_filter": map[string]interface{}{
			"include": []interface{}{"^analytics_.*"},
			"exclude": []interface{}{},
		},
	}

	s := &Source{}
	result, err := s.Discover(context.Background(), config)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should only have 2 DAGs matching the analytics_ pattern
	assert.Len(t, result.Assets, 2)

	for _, a := range result.Assets {
		assert.Contains(t, *a.Name, "analytics_")
	}
}

func TestClient_ListDAGs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := DAGCollection{
			DAGs: []DAG{
				{DagID: "dag_1", IsActive: true},
				{DagID: "dag_2", IsActive: true},
			},
			TotalCount: 2,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "password",
	})

	dags, err := client.ListDAGs(context.Background(), true)
	require.NoError(t, err)
	assert.Len(t, dags, 2)
}

func TestClient_ListDatasets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := DatasetCollection{
			Datasets: []Dataset{
				{
					ID:        1,
					URI:       "s3://bucket/dataset1",
					CreatedAt: "2024-01-01T00:00:00+00:00",
					ConsumingDags: []DagRef{
						{DagID: "consumer_dag"},
					},
					ProducingTasks: []TaskRef{
						{DagID: "producer_dag", TaskID: "task_1"},
					},
				},
			},
			TotalCount: 1,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:  server.URL,
		APIToken: "my-token",
	})

	datasets, err := client.ListDatasets(context.Background())
	require.NoError(t, err)
	assert.Len(t, datasets, 1)
	assert.Equal(t, "s3://bucket/dataset1", datasets[0].URI)
	assert.Len(t, datasets[0].ConsumingDags, 1)
	assert.Len(t, datasets[0].ProducingTasks, 1)
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		response := APIError{
			Detail: "DAG not found",
			Status: 404,
			Title:  "Not Found",
			Type:   "about:blank",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	})

	_, err := client.GetDAG(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DAG not found")
}

func TestCreateDAGAsset(t *testing.T) {
	s := &Source{
		config: &Config{
			BaseConfig: plugin.BaseConfig{
				Tags: []string{"test-tag"},
			},
		},
	}

	description := "Test DAG description"
	dag := DAG{
		DagID:       "test_dag",
		Description: &description,
		Fileloc:     "/opt/airflow/dags/test.py",
		IsPaused:    false,
		IsActive:    true,
		Owners:      []string{"admin", "analyst"},
		Tags:        []Tag{{Name: "etl"}, {Name: "daily"}},
		ScheduleInterval: &ScheduleInterval{
			Type:  "cron",
			Value: "0 0 * * *",
		},
	}

	asset := s.createDAGAsset(dag)

	assert.Equal(t, "test_dag", *asset.Name)
	assert.Equal(t, "Pipeline", asset.Type)
	assert.Contains(t, asset.Providers, "Airflow")
	assert.Equal(t, "Test DAG description", *asset.Description)
	assert.Contains(t, *asset.MRN, "mrn://pipeline/airflow/test_dag")

	// Check metadata
	assert.Equal(t, "test_dag", asset.Metadata["dag_id"])
	assert.Equal(t, "/opt/airflow/dags/test.py", asset.Metadata["file_path"])
	assert.Equal(t, "0 0 * * *", asset.Metadata["schedule_interval"])
	assert.Equal(t, "admin, analyst", asset.Metadata["owners"])

	// Check tags (only config tags, not DAG tags from Airflow)
	assert.Equal(t, []string{"test-tag"}, asset.Tags)
}

func TestCreateTaskAsset(t *testing.T) {
	s := &Source{
		config: &Config{
			BaseConfig: plugin.BaseConfig{
				Tags: []string{"airflow"},
			},
		},
	}

	task := Task{
		TaskID:            "extract_data",
		OperatorName:      "PythonOperator",
		TriggerRule:       "all_success",
		Retries:           3,
		Pool:              "default_pool",
		DownstreamTaskIDs: []string{"transform_data"},
	}

	asset := s.createTaskAsset("my_dag", task)

	assert.Equal(t, "my_dag.extract_data", *asset.Name)
	assert.Equal(t, "Task", asset.Type)
	assert.Contains(t, asset.Providers, "Airflow")
	assert.Contains(t, *asset.MRN, "mrn://task/airflow/my_dag.extract_data")

	// Check metadata
	assert.Equal(t, "extract_data", asset.Metadata["task_id"])
	assert.Equal(t, "my_dag", asset.Metadata["dag_id"])
	assert.Equal(t, "PythonOperator", asset.Metadata["operator_name"])
	assert.Equal(t, 3, asset.Metadata["retries"])
	assert.Equal(t, "default_pool", asset.Metadata["pool"])
}

func TestCreateDatasetAsset(t *testing.T) {
	s := &Source{
		config: &Config{
			BaseConfig: plugin.BaseConfig{
				Tags: []string{"data-catalog"},
			},
		},
	}

	dataset := Dataset{
		ID:        1,
		URI:       "s3://my-bucket/path/to/data.parquet",
		CreatedAt: "2024-01-01T00:00:00+00:00",
		UpdatedAt: "2024-01-15T12:00:00+00:00",
		Extra: map[string]interface{}{
			"format": "parquet",
		},
		ConsumingDags: []DagRef{
			{DagID: "consumer_1"},
			{DagID: "consumer_2"},
		},
		ProducingTasks: []TaskRef{
			{DagID: "producer", TaskID: "write_task"},
		},
	}

	asset := s.createDatasetAsset(dataset)

	// S3 URI should create an S3 Bucket asset with just the bucket name
	assert.Equal(t, "my-bucket", *asset.Name)
	assert.Equal(t, "Bucket", asset.Type)
	assert.Contains(t, asset.Providers, "S3")
	assert.Contains(t, *asset.MRN, "mrn://bucket/s3/my-bucket")

	// Check metadata
	assert.Equal(t, "s3://my-bucket/path/to/data.parquet", asset.Metadata["uri"])
	assert.Equal(t, 2, asset.Metadata["consumer_count"])
	assert.Equal(t, 1, asset.Metadata["producer_count"])
	assert.Equal(t, "parquet", asset.Metadata["extra_format"])

	// Check tags - only config tags, no hardcoded tags
	assert.Contains(t, asset.Tags, "data-catalog")
	assert.NotContains(t, asset.Tags, "airflow-dataset")
}

func TestCreateDatasetAsset_Kafka(t *testing.T) {
	s := &Source{
		config: &Config{},
	}

	dataset := Dataset{
		ID:        2,
		URI:       "kafka://redpanda/user-events",
		CreatedAt: "2024-01-01T00:00:00+00:00",
		UpdatedAt: "2024-01-15T12:00:00+00:00",
	}

	asset := s.createDatasetAsset(dataset)

	// Kafka URI should create a Kafka Topic asset
	assert.Equal(t, "user-events", *asset.Name)
	assert.Equal(t, "Topic", asset.Type)
	assert.Contains(t, asset.Providers, "Kafka")
	assert.Contains(t, *asset.MRN, "mrn://topic/kafka/user-events")
}

func TestParseDatasetURI(t *testing.T) {
	tests := []struct {
		uri          string
		wantProvider string
		wantType     string
		wantName     string
	}{
		{"s3://bucket/path/file.parquet", "S3", "Bucket", "bucket"},
		{"s3a://bucket/data", "S3", "Bucket", "bucket"},
		{"s3://raw-data/events/", "S3", "Bucket", "raw-data"},
		{"gs://gcs-bucket/data", "GCS", "Bucket", "gcs-bucket"},
		{"kafka://broker/topic-name", "Kafka", "Topic", "topic-name"},
		{"kafka://localhost:9092/events", "Kafka", "Topic", "events"},
		{"postgresql://host/db/schema/table", "PostgreSQL", "Table", "host/db/schema/table"},
		{"mysql://host/db/table", "MySQL", "Table", "host/db/table"},
		{"bigquery://project/dataset/table", "BigQuery", "Table", "project/dataset/table"},
		{"snowflake://account/db/schema/table", "Snowflake", "Table", "account/db/schema/table"},
		{"http://api.example.com/data", "HTTP", "Endpoint", "http://api.example.com/data"},
		{"file:///path/to/file.csv", "File", "File", "/path/to/file.csv"},
		{"custom://some/path", "Custom", "Dataset", "some/path"},
		{"no-scheme-uri", "Airflow", "Dataset", "no-scheme-uri"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			provider, assetType, name := parseDatasetURI(tt.uri)
			assert.Equal(t, tt.wantProvider, provider)
			assert.Equal(t, tt.wantType, assetType)
			assert.Equal(t, tt.wantName, name)
		})
	}
}
