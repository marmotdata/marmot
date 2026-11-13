package openmetadata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenMetadata has no endpoint that dumps every edge, so lineage is read
// one entity at a time and each edge is seen twice, once from each end.

func lineageFixture() *fakeOM {
	raw := tableEntity("pg", "Postgres", "pg.shop.public.raw_orders", "Regular")
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")

	edge := lineageEdge{FromEntity: "id-pg.shop.public.raw_orders", ToEntity: "id-pg.shop.public.orders"}

	return newFakeOM().
		with("tables", raw, orders).
		withLineage("table", "id-pg.shop.public.raw_orders", lineageResponse{DownstreamEdges: []lineageEdge{edge}}).
		withLineage("table", "id-pg.shop.public.orders", lineageResponse{UpstreamEdges: []lineageEdge{edge}})
}

func TestLineage_TurnsOpenMetadataEdgesIntoMarmotLineage(t *testing.T) {
	result := discover(t, lineageFixture(), nil)

	assert.True(t, hasEdge(result,
		"mrn://asset/table/postgresql/shop.public.raw_orders", "mrn://asset/table/postgresql/shop.public.orders", "DEPENDS_ON"))
}

func TestLineage_DeduplicatesEdgesSeenFromBothEnds(t *testing.T) {
	result := discover(t, lineageFixture(), nil)

	count := 0
	for _, edge := range result.Lineage {
		if edge.Type == "DEPENDS_ON" {
			count++
		}
	}
	assert.Equal(t, 1, count, "one relationship must produce one edge")
}

func TestLineage_CarriesThePipelineThatMovedTheData(t *testing.T) {
	pipeline := entity("airflow", "Airflow", "airflow.orders_etl")

	edge := lineageEdge{
		FromEntity: "id-pg.shop.public.raw_orders",
		ToEntity:   "id-pg.shop.public.orders",
		Details:    &lineageDetails{Pipeline: &entityRef{ID: "id-airflow.orders_etl"}},
	}

	f := newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.raw_orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		with("pipelines", pipeline).
		withLineage("table", "id-pg.shop.public.raw_orders", lineageResponse{DownstreamEdges: []lineageEdge{edge}})

	result := discover(t, f, nil)

	var found *pluginsdk.LineageEdge
	for i, e := range result.Lineage {
		if e.Type == "DEPENDS_ON" {
			found = &result.Lineage[i]
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "mrn://asset/pipeline/airflow/orders_etl", found.JobMRN)
}

// columnLineageResponse is GET /api/v1/lineage/table/{id} for raw_orders as
// OpenMetadata returns it: the entity and its neighbours by FQN, and the
// edge's column lineage named by column FQN.
const columnLineageResponse = `{
  "entity": {"id": "id-pg.shop.public.raw_orders", "type": "table", "name": "raw_orders",
             "fullyQualifiedName": "pg.shop.public.raw_orders"},
  "nodes": [
    {"id": "id-pg.shop.public.orders", "type": "table", "name": "orders",
     "fullyQualifiedName": "pg.shop.public.orders"}
  ],
  "upstreamEdges": [],
  "downstreamEdges": [
    {
      "fromEntity": "id-pg.shop.public.raw_orders",
      "toEntity": "id-pg.shop.public.orders",
      "lineageDetails": {
        "sqlQuery": "INSERT INTO orders SELECT id, amount * fx AS amount_gbp FROM raw_orders",
        "source": "QueryLineage",
        "columnsLineage": [
          {"fromColumns": ["pg.shop.public.raw_orders.id"], "toColumn": "pg.shop.public.orders.id"},
          {"fromColumns": ["pg.shop.public.raw_orders.amount", "pg.shop.public.raw_orders.fx"],
           "toColumn": "pg.shop.public.orders.amount_gbp", "function": "amount * fx"},
          {"fromColumns": ["pg.shop.public.raw_orders.\"ship.to\".city"], "toColumn": "pg.shop.public.orders.city"},
          {"fromColumns": ["pg.shop.public.customers.name"], "toColumn": "pg.shop.public.orders.customer_name"},
          {"fromColumns": [], "toColumn": "pg.shop.public.orders.loaded_at"}
        ]
      }
    }
  ]
}`

func TestLineage_CarriesColumnLineageOnTheEdge(t *testing.T) {
	var resp lineageResponse
	require.NoError(t, json.Unmarshal([]byte(columnLineageResponse), &resp))

	f := newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.raw_orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		withLineage("table", "id-pg.shop.public.raw_orders", resp)

	result := discover(t, f, nil)

	var found *pluginsdk.LineageEdge
	for i, e := range result.Lineage {
		if e.Type == "DEPENDS_ON" {
			found = &result.Lineage[i]
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"id"}, ToColumn: "id"},
		{FromColumns: []string{"amount", "fx"}, ToColumn: "amount_gbp", Transform: "amount * fx"},
		{FromColumns: []string{"ship.to.city"}, ToColumn: "city"},
	}, found.ColumnLineage, "columns from another table and targets with no source are dropped")
}

func TestLineage_KeepsColumnLineageWhenTheOtherEndOmitsIt(t *testing.T) {
	var withColumns lineageResponse
	require.NoError(t, json.Unmarshal([]byte(columnLineageResponse), &withColumns))
	bare := lineageEdge{FromEntity: "id-pg.shop.public.raw_orders", ToEntity: "id-pg.shop.public.orders"}

	f := newFakeOM().
		with("tables",
			tableEntity("pg", "Postgres", "pg.shop.public.raw_orders", "Regular"),
			tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")).
		withLineage("table", "id-pg.shop.public.raw_orders", withColumns).
		withLineage("table", "id-pg.shop.public.orders", lineageResponse{UpstreamEdges: []lineageEdge{bare}})

	result := discover(t, f, nil)

	require.Len(t, result.Lineage, 1)
	assert.Len(t, result.Lineage[0].ColumnLineage, 3)
}

func TestColumnName_StripsTheTableFQN(t *testing.T) {
	name, ok := columnName("svc.db.schema.orders.id", "svc.db.schema.orders")
	assert.True(t, ok)
	assert.Equal(t, "id", name)

	name, ok = columnName(`svc."my.db".schema.orders."a.b"`, `svc."my.db".schema.orders`)
	assert.True(t, ok)
	assert.Equal(t, "a.b", name)

	_, ok = columnName("svc.db.schema.customers.id", "svc.db.schema.orders")
	assert.False(t, ok, "a column of another table is not this table's")

	name, ok = columnName("svc.db.schema.orders.id", "")
	assert.True(t, ok)
	assert.Equal(t, "id", name, "without the table FQN the last part is the name")
}

func TestLineage_IgnoresEdgesToEntitiesThatWereNotImported(t *testing.T) {
	edge := lineageEdge{FromEntity: "id-pg.shop.public.raw_orders", ToEntity: "id-not-imported"}

	f := newFakeOM().
		with("tables", tableEntity("pg", "Postgres", "pg.shop.public.raw_orders", "Regular")).
		withLineage("table", "id-pg.shop.public.raw_orders", lineageResponse{DownstreamEdges: []lineageEdge{edge}})

	result := discover(t, f, nil)

	assert.Empty(t, result.Lineage, "an edge must never point at an asset that was not created")
}

func TestLineage_CanBeTurnedOff(t *testing.T) {
	result := discover(t, lineageFixture(), pluginsdk.RawConfig{"include_lineage": false})

	assert.Empty(t, result.Lineage)
}

func TestRunHistory_TurnsPipelineExecutionsIntoEvents(t *testing.T) {
	f := newFakeOM().
		with("pipelines", entity("airflow", "Airflow", "airflow.orders_etl")).
		withRuns("airflow.orders_etl",
			map[string]interface{}{
				"timestamp":       1785871852293,
				"executionStatus": "Successful",
				"runId":           "run-1",
				"taskStatus": []map[string]interface{}{
					{"name": "extract", "startTime": 1785871800000, "endTime": 1785871852293},
				},
			})

	result := discover(t, f, nil)

	require.Len(t, result.RunHistory, 1)
	history := result.RunHistory[0]
	assert.Equal(t, "mrn://asset/pipeline/airflow/orders_etl", history.AssetMRN)

	require.Len(t, history.Runs, 2, "an execution produces a start and a terminal event")
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
	assert.Equal(t, "run-1", history.Runs[0].RunID)
	assert.Equal(t, "airflow", history.Runs[0].JobNamespace)
	assert.Equal(t, "orders_etl", history.Runs[0].JobName)
	assert.True(t, history.Runs[0].EventTime.Before(history.Runs[1].EventTime),
		"the start event must come from the earliest task start")
}

func TestRunHistory_MapsAFailedExecution(t *testing.T) {
	f := newFakeOM().
		with("pipelines", entity("airflow", "Airflow", "airflow.orders_etl")).
		withRuns("airflow.orders_etl", map[string]interface{}{
			"timestamp": 1785871852293, "executionStatus": "Failed",
		})

	result := discover(t, f, nil)

	require.Len(t, result.RunHistory, 1)
	assert.Equal(t, "FAIL", result.RunHistory[0].Runs[1].EventType)
}

func TestRunHistory_NamesRunsWithoutAnIdentifier(t *testing.T) {
	// OpenMetadata does not require a run id, but Marmot groups events
	// by one, so a stable identifier is derived from the timestamp.
	f := newFakeOM().
		with("pipelines", entity("airflow", "Airflow", "airflow.orders_etl")).
		withRuns("airflow.orders_etl", map[string]interface{}{
			"timestamp": 1785871852293, "executionStatus": "Successful",
		})

	result := discover(t, f, nil)

	require.Len(t, result.RunHistory, 1)
	assert.Equal(t, "orders_etl-1785871852293", result.RunHistory[0].Runs[0].RunID)
}

func TestRunHistory_CanBeTurnedOff(t *testing.T) {
	f := newFakeOM().
		with("pipelines", entity("airflow", "Airflow", "airflow.orders_etl")).
		withRuns("airflow.orders_etl", map[string]interface{}{
			"timestamp": 1785871852293, "executionStatus": "Successful",
		})

	result := discover(t, f, pluginsdk.RawConfig{"include_run_history": false})

	assert.Empty(t, result.RunHistory)
}

func TestPaging_StopsWhenTheCursorStopsMoving(t *testing.T) {
	// A server that keeps returning the same cursor would page forever.
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   []map[string]interface{}{{"id": "a", "name": "a", "fullyQualifiedName": "s.a"}},
			"paging": map[string]interface{}{"after": "stuck"},
		})
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	entities, err := listAll[entityBase](t.Context(), c, "/v1/tables", "", 100, false)

	require.NoError(t, err)
	assert.Len(t, entities, 2, "one page, then one repeat that ends it")
	assert.Equal(t, 2, pages)
}

func TestListOptional_TreatsOnlyMissingEndpointsAsUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":400,"message":"Invalid field name children"}`, http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	_, _, err := listOptional[entityBase](t.Context(), c, "/v1/containers", "children", 100, false)

	require.Error(t, err, "a bad request must not be mistaken for a missing endpoint")
	assert.Contains(t, err.Error(), "Invalid field name")
}

func TestRunHistory_UsesOpenMetadatasExecutionId(t *testing.T) {
	f := newFakeOM().
		with("pipelines", entity("airflow", "Airflow", "airflow.orders_etl")).
		withRuns("airflow.orders_etl", map[string]interface{}{
			"timestamp": 1785871852293, "executionStatus": "Successful",
			"executionId": "20cbbac8-5d0a-4865-93d2-7781bcb8bed4",
		})

	result := discover(t, f, nil)

	require.Len(t, result.RunHistory, 1)
	assert.Equal(t, "20cbbac8-5d0a-4865-93d2-7781bcb8bed4", result.RunHistory[0].Runs[0].RunID)
}
