package airflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      pluginsdk.RawConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with basic auth",
			config: pluginsdk.RawConfig{
				"host":     "http://localhost:8080",
				"username": "admin",
				"password": "admin",
			},
			wantErr: false,
		},
		{
			name: "valid config with api token",
			config: pluginsdk.RawConfig{
				"host":      "http://localhost:8080",
				"api_token": "my-api-token",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: pluginsdk.RawConfig{
				"username": "admin",
				"password": "admin",
			},
			wantErr:     true,
			errContains: "host",
		},
		{
			name: "missing authentication",
			config: pluginsdk.RawConfig{
				"host": "http://localhost:8080",
			},
			wantErr:     true,
			errContains: "authentication required",
		},
		{
			name: "trailing slash removed from host",
			config: pluginsdk.RawConfig{
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

	config := pluginsdk.RawConfig{
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
	//
	// The dataset also names producer_dag.produce_task as a producer, but
	// this fixture's /dags response never returns producer_dag, so no asset
	// backs it. That edge is dropped rather than emitted: the server
	// silently discards edges whose endpoints do not exist, so emitting it
	// would only hide the gap.
	assert.Len(t, result.Lineage, 4)
	assertLineageOnlyReferencesDiscoveredAssets(t, result)
}

func TestSource_DiscoverWithFilter(t *testing.T) {
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

	config := pluginsdk.RawConfig{
		"host":              server.URL,
		"username":          "admin",
		"password":          "admin",
		"discover_dags":     true,
		"discover_tasks":    false,
		"discover_datasets": false,
		"filter": map[string]interface{}{
			"include": []interface{}{"^analytics_.*"},
			"exclude": []interface{}{},
		},
	}

	s := &Source{}
	result, err := s.Discover(context.Background(), config)

	require.NoError(t, err)
	require.NotNil(t, result)

	// The plugin returns all 4 DAGs; name filtering is applied by the
	// Marmot host after discovery.
	assert.Len(t, result.Assets, 4)
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
			BaseConfig: pluginsdk.BaseConfig{
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
			BaseConfig: pluginsdk.BaseConfig{
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
			BaseConfig: pluginsdk.BaseConfig{
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
		// The relational schemes address a table the way the Marmot plugin
		// that owns it does, so a dataset and the table itself are one asset.
		{"postgresql://host:5432/db/public/users", "PostgreSQL", "Table", "public.users"},
		{"postgresql://host/db/schema/table", "PostgreSQL", "Table", "schema.table"},
		{"mysql://host/db/table", "MySQL", "Table", "db.table"},
		{"bigquery://project/dataset/table", "BigQuery", "Table", "dataset.table"},
		// No Marmot plugin owns Snowflake, so all three levels stay.
		{"snowflake://account/db/schema/table", "Snowflake", "Table", "db.schema.table"},
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

// A URI whose scheme is empty used to index scheme[:1] and panic, taking
// the whole discovery run down.
func TestParseDatasetURI_EmptySchemeDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		provider, assetType, name := parseDatasetURI("://host/db/table")
		assert.Equal(t, "Airflow", provider)
		assert.Equal(t, "Dataset", assetType)
		assert.Equal(t, "://host/db/table", name)
	})
}

// assertLineageOnlyReferencesDiscoveredAssets is the guard for the bug
// class this plugin family kept reproducing: an edge naming an MRN the
// same run never emits is silently dropped by the server, so the lineage
// just disappears instead of failing loudly.
func assertLineageOnlyReferencesDiscoveredAssets(t *testing.T, result *pluginsdk.DiscoveryResult) {
	t.Helper()

	emitted := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		if a.MRN != nil {
			emitted[*a.MRN] = struct{}{}
		}
	}

	for _, edge := range result.Lineage {
		assert.Contains(t, emitted, edge.Source,
			"lineage edge source %q has no asset behind it", edge.Source)
		assert.Contains(t, emitted, edge.Target,
			"lineage edge target %q has no asset behind it", edge.Target)
	}
}

// A dataset can name a DAG that only_active filtered out, or that
// discover_dags never fetched at all. Neither may leave a dangling edge.
func TestDiscover_DatasetLineageSkipsUndiscoveredDAGs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/dags":
			// Only one DAG is active; paused_dag is filtered out upstream.
			_ = json.NewEncoder(w).Encode(DAGCollection{
				DAGs:       []DAG{{DagID: "active_dag", IsActive: true}},
				TotalCount: 1,
			})
		case "/api/v1/dags/active_dag/tasks":
			_ = json.NewEncoder(w).Encode(TaskCollection{
				Tasks:      []Task{{TaskID: "writer"}},
				TotalCount: 1,
			})
		case "/api/v1/datasets":
			_ = json.NewEncoder(w).Encode(DatasetCollection{
				Datasets: []Dataset{{
					URI: "s3://analytics/events",
					// active_dag exists, paused_dag does not.
					ConsumingDags: []DagRef{{DagID: "active_dag"}, {DagID: "paused_dag"}},
					ProducingTasks: []TaskRef{
						{DagID: "active_dag", TaskID: "writer"},
						{DagID: "paused_dag", TaskID: "writer"},
					},
				}},
				TotalCount: 1,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := &Source{}
	result, err := s.Discover(context.Background(), pluginsdk.RawConfig{
		"host":              server.URL,
		"username":          "u",
		"password":          "p",
		"discover_dags":     true,
		"discover_tasks":    true,
		"discover_datasets": true,
	})
	require.NoError(t, err)

	assertLineageOnlyReferencesDiscoveredAssets(t, result)

	// The edges for the DAG that does exist must still be there.
	var kinds []string
	for _, e := range result.Lineage {
		if e.Type == "FEEDS" || e.Type == "PRODUCES" {
			kinds = append(kinds, e.Type)
		}
	}
	assert.ElementsMatch(t, []string{"FEEDS", "PRODUCES"}, kinds,
		"the discovered DAG should keep exactly one FEEDS and one PRODUCES edge")
}

// With discover_dags off there are no Pipeline assets at all, so every
// dataset edge must be dropped rather than left dangling.
func TestDiscover_DatasetLineageDroppedWhenDAGsNotDiscovered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/datasets" {
			_ = json.NewEncoder(w).Encode(DatasetCollection{
				Datasets: []Dataset{{
					URI:            "s3://analytics/events",
					ConsumingDags:  []DagRef{{DagID: "some_dag"}},
					ProducingTasks: []TaskRef{{DagID: "some_dag", TaskID: "writer"}},
				}},
				TotalCount: 1,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := &Source{}
	result, err := s.Discover(context.Background(), pluginsdk.RawConfig{
		"host":              server.URL,
		"username":          "u",
		"password":          "p",
		"discover_dags":     false,
		"discover_datasets": true,
	})
	require.NoError(t, err)

	assertLineageOnlyReferencesDiscoveredAssets(t, result)
	assert.Empty(t, result.Lineage, "no Pipeline assets exist, so no dataset edge can be emitted")
}

// The producing task is named by the API and its asset already exists, so
// the edge should start at the Task rather than the whole DAG.
func TestDiscover_ProducesEdgeStartsAtTheProducingTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/dags":
			_ = json.NewEncoder(w).Encode(DAGCollection{
				DAGs:       []DAG{{DagID: "etl", IsActive: true}},
				TotalCount: 1,
			})
		case "/api/v1/dags/etl/tasks":
			_ = json.NewEncoder(w).Encode(TaskCollection{
				Tasks:      []Task{{TaskID: "load"}},
				TotalCount: 1,
			})
		case "/api/v1/datasets":
			_ = json.NewEncoder(w).Encode(DatasetCollection{
				Datasets: []Dataset{{
					URI:            "s3://warehouse/out",
					ProducingTasks: []TaskRef{{DagID: "etl", TaskID: "load"}},
				}},
				TotalCount: 1,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := &Source{}
	result, err := s.Discover(context.Background(), pluginsdk.RawConfig{
		"host":              server.URL,
		"username":          "u",
		"password":          "p",
		"discover_dags":     true,
		"discover_tasks":    true,
		"discover_datasets": true,
	})
	require.NoError(t, err)

	var produces []string
	for _, e := range result.Lineage {
		if e.Type == "PRODUCES" {
			produces = append(produces, e.Source)
		}
	}
	assert.Equal(t, []string{"mrn://task/airflow/etl.load"}, produces)
}
