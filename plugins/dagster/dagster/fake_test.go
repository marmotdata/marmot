package dagster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The fixtures below are trimmed copies of real responses from a Dagster
// 1.13.21 webserver, so the tests decode the shapes the plugin meets in
// production rather than shapes invented for the test.

const repositoriesFixture = `{
  "data": {
    "repositoriesOrError": {
      "__typename": "RepositoryConnection",
      "nodes": [
        {
          "id": "4d94642df0c8ec600d6a48d740e8587b4946eb0a",
          "name": "__repository__",
          "location": {"id": "defs.py", "name": "defs.py"},
          "jobs": [
            {
              "id": "8243f6b55ea18c4fa2513c883f8945d54952cc27",
              "name": "__ASSET_JOB",
              "description": null,
              "isJob": true,
              "isAssetJob": true,
              "tags": [],
              "schedules": [],
              "sensors": []
            },
            {
              "id": "3dba13a21c3c82f287949fee9e86abc6379709f0",
              "name": "etl_job",
              "description": "Op based ETL job.",
              "isJob": true,
              "isAssetJob": false,
              "tags": [{"key": "team", "value": "data-platform"}],
              "schedules": [
                {"name": "etl_schedule", "cronSchedule": "0 3 * * *", "scheduleState": {"status": "RUNNING"}}
              ],
              "sensors": [
                {"name": "etl_sensor", "sensorState": {"status": "STOPPED"}}
              ]
            },
            {
              "id": "d24fc9b3adae7881f1fa3580f90d87f422d618c1",
              "name": "orders_job",
              "description": null,
              "isJob": true,
              "isAssetJob": true,
              "tags": [],
              "schedules": [],
              "sensors": []
            }
          ]
        }
      ]
    }
  }
}`

const etlJobHandlesFixture = `{
  "data": {
    "pipelineOrError": {
      "__typename": "Pipeline",
      "solidHandles": [
        {
          "handleID": "extract",
          "solid": {
            "name": "extract",
            "definition": {"name": "extract", "description": "Extract records from the source."},
            "inputs": [],
            "outputs": [{"definition": {"name": "result"}}]
          }
        },
        {
          "handleID": "load",
          "solid": {
            "name": "load",
            "definition": {"name": "load", "description": "Load records into the warehouse."},
            "inputs": [
              {
                "definition": {"name": "records"},
                "dependsOn": [{"solid": {"name": "extract"}, "definition": {"name": "result"}}]
              }
            ],
            "outputs": [{"definition": {"name": "result"}}]
          }
        },
        {
          "handleID": "transform",
          "solid": {
            "name": "transform",
            "definition": {"name": "transform", "description": "Transform extracted records."},
            "inputs": [
              {
                "definition": {"name": "records"},
                "dependsOn": [{"solid": {"name": "extract"}, "definition": {"name": "result"}}]
              }
            ],
            "outputs": [{"definition": {"name": "result"}}]
          }
        }
      ]
    }
  }
}`

const ordersJobHandlesFixture = `{
  "data": {
    "pipelineOrError": {
      "__typename": "Pipeline",
      "solidHandles": [
        {
          "handleID": "clean_orders",
          "solid": {
            "name": "clean_orders",
            "definition": {"name": "clean_orders", "description": "Cleaned orders stored in DuckDB."},
            "inputs": [
              {
                "definition": {"name": "raw_orders"},
                "dependsOn": [{"solid": {"name": "raw_orders"}, "definition": {"name": "result"}}]
              }
            ],
            "outputs": [{"definition": {"name": "result"}}]
          }
        },
        {
          "handleID": "raw_orders",
          "solid": {
            "name": "raw_orders",
            "definition": {"name": "raw_orders", "description": "Raw orders landed from the source system."},
            "inputs": [],
            "outputs": [{"definition": {"name": "result"}}]
          }
        }
      ]
    }
  }
}`

const etlJobRunsFixture = `{
  "data": {
    "runsOrError": {
      "__typename": "Runs",
      "results": [
        {
          "runId": "0a85d76d-cf75-4fa6-af87-8815e349db45",
          "status": "SUCCESS",
          "startTime": 1788853304.552549,
          "endTime": 1788853306.937723,
          "updateTime": 1788853306.937723,
          "tags": [
            {"key": "dagster/code_location", "value": "defs.py"},
            {"key": "team", "value": "data-platform"}
          ],
          "stepStats": [
            {"stepKey": "extract", "status": "SUCCESS", "startTime": 1788853305.496759, "endTime": 1788853305.518494},
            {"stepKey": "transform", "status": "SUCCESS", "startTime": 1788853306.666722, "endTime": 1788853306.690628},
            {"stepKey": "load", "status": "SUCCESS", "startTime": 1788853306.66859, "endTime": 1788853306.691176}
          ]
        }
      ]
    }
  }
}`

const emptyRunsFixture = `{"data": {"runsOrError": {"__typename": "Runs", "results": []}}}`

const assetNodesFixture = `{
  "data": {
    "repositoryOrError": {
      "__typename": "Repository",
      "assetNodes": [
        {
          "id": "r.defs.py.__repository__.clean_orders",
          "assetKey": {"path": ["clean_orders"]},
          "description": "Cleaned orders stored in DuckDB.",
          "computeKind": "duckdb",
          "groupName": "staging",
          "opNames": ["clean_orders"],
          "jobNames": ["__ASSET_JOB", "orders_job"],
          "isPartitioned": false,
          "isObservable": false,
          "isMaterializable": true,
          "metadataEntries": [
            {"__typename": "TextMetadataEntry", "label": "dagster/table_name", "text": "analytics.staging.clean_orders"},
            {"__typename": "TextMetadataEntry", "label": "database", "text": "analytics"},
            {"__typename": "TextMetadataEntry", "label": "schema", "text": "staging"},
            {"__typename": "TextMetadataEntry", "label": "table", "text": "clean_orders"},
            {"__typename": "UrlMetadataEntry", "label": "docs_url", "url": "https://example.com/docs/clean_orders"},
            {"__typename": "PathMetadataEntry", "label": "source_path", "path": "/data/staging/clean_orders.parquet"},
            {"__typename": "JsonMetadataEntry", "label": "profile", "jsonString": "{\"rows\": 2}"},
            {
              "__typename": "TableSchemaMetadataEntry",
              "label": "dagster/column_schema",
              "schema": {
                "columns": [
                  {"name": "id", "type": "integer", "description": "Order identifier", "constraints": {"nullable": true, "unique": false}},
                  {"name": "amount", "type": "double", "description": "Order amount", "constraints": {"nullable": true, "unique": false}}
                ]
              }
            }
          ],
          "dependencies": [{"asset": {"assetKey": {"path": ["raw_orders"]}}}],
          "dependedBy": [{"asset": {"assetKey": {"path": ["order_metrics"]}}}],
          "assetMaterializations": [{"runId": "05c1267f-d0b5-496c-a5dc-17aa462775f1", "timestamp": "1788853307723"}]
        },
        {
          "id": "r.defs.py.__repository__.order_metrics",
          "assetKey": {"path": ["order_metrics"]},
          "description": "Aggregated order metrics.",
          "computeKind": null,
          "groupName": "marts",
          "opNames": ["order_metrics"],
          "jobNames": ["__ASSET_JOB", "orders_job"],
          "isPartitioned": false,
          "isObservable": false,
          "isMaterializable": true,
          "metadataEntries": [],
          "dependencies": [{"asset": {"assetKey": {"path": ["clean_orders"]}}}],
          "dependedBy": [],
          "assetMaterializations": [{"runId": "05c1267f-d0b5-496c-a5dc-17aa462775f1", "timestamp": "1788853308896"}]
        },
        {
          "id": "r.defs.py.__repository__.raw_orders",
          "assetKey": {"path": ["raw_orders"]},
          "description": "Raw orders landed from the source system.",
          "computeKind": null,
          "groupName": "raw",
          "opNames": ["raw_orders"],
          "jobNames": ["__ASSET_JOB", "orders_job"],
          "isPartitioned": false,
          "isObservable": false,
          "isMaterializable": true,
          "metadataEntries": [],
          "dependencies": [],
          "dependedBy": [{"asset": {"assetKey": {"path": ["clean_orders"]}}}],
          "assetMaterializations": [{"runId": "05c1267f-d0b5-496c-a5dc-17aa462775f1", "timestamp": "1788853306483"}]
        }
      ]
    }
  }
}`

// fakeDagster serves the captured fixtures from one GraphQL endpoint, picking
// the response by operation name and by the job or location the variables ask
// for, exactly as the real webserver would.
type fakeDagster struct {
	repositories string
	handles      map[string]string
	runs         map[string]string
	assetNodes   map[string]string

	// tokens records the Dagster-Cloud-Api-Token header of every request.
	tokens []string
}

func newFakeDagster() *fakeDagster {
	return &fakeDagster{
		repositories: repositoriesFixture,
		handles: map[string]string{
			"etl_job":    etlJobHandlesFixture,
			"orders_job": ordersJobHandlesFixture,
		},
		runs: map[string]string{
			"etl_job":    etlJobRunsFixture,
			"orders_job": emptyRunsFixture,
		},
		assetNodes: map[string]string{
			"defs.py": assetNodesFixture,
		},
	}
}

func (f *fakeDagster) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			http.NotFound(w, r)
			return
		}
		f.tokens = append(f.tokens, r.Header.Get("Dagster-Cloud-Api-Token"))

		var request struct {
			Query     string `json:"query"`
			Variables struct {
				Selector struct {
					PipelineName           string `json:"pipelineName"`
					RepositoryLocationName string `json:"repositoryLocationName"`
				} `json:"selector"`
				Filter struct {
					PipelineName string `json:"pipelineName"`
				} `json:"filter"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(request.Query, "MarmotRepositories"):
			writeFixture(w, f.repositories)
		case strings.Contains(request.Query, "MarmotOpHandles"):
			writeFixture(w, f.handles[request.Variables.Selector.PipelineName])
		case strings.Contains(request.Query, "MarmotRuns"):
			writeFixture(w, f.runs[request.Variables.Filter.PipelineName])
		case strings.Contains(request.Query, "MarmotAssetNodes"):
			writeFixture(w, f.assetNodes[request.Variables.Selector.RepositoryLocationName])
		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func writeFixture(w http.ResponseWriter, body string) {
	if body == "" {
		http.Error(w, "no fixture for this request", http.StatusNotFound)
		return
	}
	w.Write([]byte(body))
}
