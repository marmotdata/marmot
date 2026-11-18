package lineage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// runsOf returns the job's run history as the asset page shows it, newest first.
func (h *olHarness) runsOf(job string) []*asset.RunHistory {
	h.t.Helper()
	list, _, err := h.assets.GetRunHistory(h.ctx, h.asset(job).ID, 50, 0)
	require.NoError(h.t, err)
	return list
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func timedEvent(eventType, runID, when string, runFacets map[string]any) map[string]any {
	ev := runEvent(eventType, runID, "nightly", []ds{pgTable("a")}, []ds{pgTable("b")})
	ev["eventTime"] = when
	if runFacets != nil {
		ev["run"].(map[string]any)["facets"] = runFacets
	}
	return ev
}

func TestRunLifecycle(t *testing.T) {
	cases := []struct {
		name   string
		events []map[string]any
		status string
		start  string
		end    string
	}{
		{"START then COMPLETE", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil)}, "COMPLETE", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"COMPLETE without START", []map[string]any{timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil)}, "COMPLETE", "", "2026-11-18T10:05:00Z"},
		{"COMPLETE delivered before START", []map[string]any{timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil), timedEvent("START", "r", "2026-11-18T10:00:00Z", nil)}, "COMPLETE", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"RUNNING events in between", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("RUNNING", "r", "2026-11-18T10:01:00Z", nil), timedEvent("RUNNING", "r", "2026-11-18T10:02:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil)}, "COMPLETE", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"still running", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("RUNNING", "r", "2026-11-18T10:01:00Z", nil)}, "RUNNING", "2026-11-18T10:00:00Z", ""},
		{"FAIL", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("FAIL", "r", "2026-11-18T10:05:00Z", nil)}, "FAIL", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"ABORT", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("ABORT", "r", "2026-11-18T10:05:00Z", nil)}, "ABORT", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"FAIL then COMPLETE, as Spark sends a failed job", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("FAIL", "r", "2026-11-18T10:05:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00.002Z", nil)}, "FAIL", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00.002Z"},
		{"OTHER before START", []map[string]any{timedEvent("OTHER", "r", "2026-11-18T09:59:00Z", nil), timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil)}, "COMPLETE", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
		{"replayed events", []map[string]any{timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("START", "r", "2026-11-18T10:00:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil), timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil)}, "COMPLETE", "2026-11-18T10:00:00Z", "2026-11-18T10:05:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			for _, ev := range c.events {
				h.sendJSON(ev)
			}
			history := h.runsOf(jobMRN("nightly"))
			require.Len(t, history, 1)
			r := history[0]
			assert.Equal(t, c.status, r.Status)
			if c.start == "" {
				assert.Nil(t, r.StartTime)
			} else if assert.NotNil(t, r.StartTime) {
				assert.True(t, at(c.start).Equal(*r.StartTime), "start %v", r.StartTime)
			}
			if c.end == "" {
				assert.Nil(t, r.EndTime)
			} else if assert.NotNil(t, r.EndTime) {
				assert.True(t, at(c.end).Equal(*r.EndTime), "end %v", r.EndTime)
			}
			distinct := map[string]bool{}
			for _, ev := range c.events {
				distinct[ev["eventType"].(string)+ev["eventTime"].(string)] = true
			}
			rows := h.query(`SELECT count(*) AS n FROM run_history`)
			assert.EqualValues(t, len(distinct), rows[0]["n"], "a replayed event is stored once")
			// The lineage graph marks the job with the same outcome.
			status := map[string]string{"COMPLETE": "succeeded", "FAIL": "failed", "ABORT": "failed", "RUNNING": "running"}[c.status]
			resp, err := h.lineage.GetAssetLineage(h.ctx, h.asset(jobMRN("nightly")).ID, 1, "both")
			require.NoError(t, err)
			for _, n := range resp.Nodes {
				if *n.Asset.MRN == jobMRN("nightly") {
					assert.Equal(t, status, n.LastRunStatus)
				}
			}
		})
	}
}

func TestRunFacetsAccumulateAcrossARun(t *testing.T) {
	h := newHarness(t)
	h.sendJSON(timedEvent("START", "r", "2026-11-18T10:00:00Z", map[string]any{
		"processing_engine": facet("https://openlineage.io/spec/facets/1-1-1/ProcessingEngineRunFacet.json", map[string]any{"name": "DuckDB", "version": "v1.5.6"}),
		"nominalTime":       facet("https://openlineage.io/spec/facets/1-0-1/NominalTimeRunFacet.json", map[string]any{"nominalStartTime": "2026-11-18T10:00:00Z"}),
	}))
	h.sendJSON(timedEvent("FAIL", "r", "2026-11-18T10:05:00Z", map[string]any{
		"errorMessage": facet("https://openlineage.io/spec/facets/1-0-1/ErrorMessageRunFacet.json", map[string]any{"message": "division by zero", "programmingLanguage": "SQL"}),
	}))
	rows := h.query(`
		SELECT jsonb_object_agg(f.key, f.value ORDER BY rh.event_time) AS facets
		FROM run_history rh, jsonb_each(rh.run_facets) f WHERE rh.run_id = 'r'`)
	facets := rows[0]["facets"].(map[string]any)
	assert.Contains(t, facets, "processing_engine")
	assert.Contains(t, facets, "nominalTime")
	assert.Equal(t, "division by zero", facets["errorMessage"].(map[string]any)["message"])
	assert.Equal(t, "FAIL", h.runsOf(jobMRN("nightly"))[0].Status)
}

func TestJobTypeComesFromMergedJobFacets(t *testing.T) {
	h := newHarness(t)
	start := timedEvent("START", "r", "2026-11-18T10:00:00Z", nil)
	start["job"].(map[string]any)["facets"] = map[string]any{
		"jobType": facet("https://openlineage.io/spec/facets/2-0-3/JobTypeJobFacet.json", map[string]any{"processingType": "STREAMING", "integration": "FLINK", "jobType": "JOB"}),
	}
	h.sendJSON(start)
	h.sendJSON(timedEvent("COMPLETE", "r", "2026-11-18T10:05:00Z", nil))
	history := h.runsOf("mrn://asset/job/flink/shop.nightly")
	require.Len(t, history, 1)
	assert.Equal(t, "STREAMING", history[0].Type, "a later event without the facet keeps the job type")
}

func TestJobEventFilesStaticLineageWithoutARun(t *testing.T) {
	h := newHarness(t)
	h.sendJSON(map[string]any{
		"eventTime": "2026-11-18T10:00:00Z", "producer": producer, "schemaURL": jobURL,
		"job":     map[string]any{"namespace": "shop", "name": "nightly", "facets": map[string]any{"sql": facet("https://openlineage.io/spec/facets/1-1-0/SQLJobFacet.json", map[string]any{"query": "insert into b select * from a", "dialect": "postgres"})}},
		"inputs":  []any{pgTable("a")},
		"outputs": []any{pgTable("b")},
	})
	a, b, j := pgMRN("a"), pgMRN("b"), jobMRN("nightly")
	requireGraph(t, graph{Nodes: []string{a, b, j}, Edges: []string{e(a, "CONSUMES", j), e(j, "PRODUCES", b)}}, h.lineageOf(b, 10, "both"))
	assert.Empty(t, h.runsOf(j))
	job := h.asset(j)
	require.NotNil(t, job.Query)
	assert.Equal(t, "insert into b select * from a", *job.Query)
	assert.Equal(t, "postgres", job.Metadata["sql_dialect"])
}

func TestDatasetEventFilesOneDataset(t *testing.T) {
	h := newHarness(t)
	h.sendJSON(map[string]any{
		"eventTime": "2026-11-18T10:00:00Z", "producer": producer, "schemaURL": dsURL,
		"dataset": withFacets(pgTable("orders"), map[string]any{
			"documentation": facet("https://openlineage.io/spec/facets/1-1-0/DocumentationDatasetFacet.json", map[string]any{"description": "Customer orders"}),
			"schema":        facet("https://openlineage.io/spec/facets/1-2-0/SchemaDatasetFacet.json", map[string]any{"fields": []any{map[string]any{"name": "id", "type": "integer"}}}),
		}),
	})
	assert.Equal(t, []string{pgMRN("orders")}, h.allMRNs())
	a := h.asset(pgMRN("orders"))
	assert.True(t, a.IsStub)
	require.NotNil(t, a.Description)
	assert.Equal(t, "Customer orders", *a.Description)
	assert.JSONEq(t, `[{"column_name":"id","data_type":"integer"}]`, a.Schema["columns"])
	assert.Empty(t, h.allEdges())
	assert.Empty(t, h.query(`SELECT 1 FROM run_history`))
}

func TestFacetsArePersisted(t *testing.T) {
	h := newHarness(t)
	input := withFacets(pgTable("orders"), map[string]any{
		"documentation":      facet("https://openlineage.io/spec/facets/1-1-0/DocumentationDatasetFacet.json", map[string]any{"description": "Customer orders", "contentType": "text/markdown"}),
		"ownership":          facet("https://openlineage.io/spec/facets/1-0-1/OwnershipDatasetFacet.json", map[string]any{"owners": []any{map[string]any{"name": "team:orders", "type": "MAINTAINER"}, map[string]any{"name": "user:ada"}}}),
		"tags":               facet("https://openlineage.io/spec/facets/1-0-0/TagsDatasetFacet.json", map[string]any{"tags": []any{map[string]any{"key": "pii", "value": "true", "source": "MANUAL"}, map[string]any{"key": "tier", "value": "gold"}, map[string]any{"key": "sensitive", "value": "true", "field": "email"}}}),
		"datasetType":        facet("https://openlineage.io/spec/facets/1-0-1/DatasetTypeDatasetFacet.json", map[string]any{"datasetType": "TABLE", "subType": "EXTERNAL"}),
		"storage":            facet("https://openlineage.io/spec/facets/1-0-1/StorageDatasetFacet.json", map[string]any{"storageLayer": "iceberg", "fileFormat": "parquet"}),
		"version":            facet("https://openlineage.io/spec/facets/1-0-1/DatasetVersionDatasetFacet.json", map[string]any{"datasetVersion": "42"}),
		"dataSource":         facet("https://openlineage.io/spec/facets/1-0-1/DatasourceDatasetFacet.json", map[string]any{"name": "shop", "uri": "postgres://reader:secret@pg:5432/shop"}),
		"catalog":            facet("https://openlineage.io/spec/facets/1-1-0/CatalogDatasetFacet.json", map[string]any{"framework": "iceberg", "type": "rest", "name": "lake"}),
		"hierarchy":          facet("https://openlineage.io/spec/facets/1-0-0/HierarchyDatasetFacet.json", map[string]any{"hierarchy": []any{map[string]any{"type": "DATABASE", "name": "shop"}}}),
		"dataQualityMetrics": facet("https://openlineage.io/spec/facets/1-0-1/DataQualityMetricsDatasetFacet.json", map[string]any{"rowCount": 1000, "bytes": 65536, "fileCount": 4, "columnMetrics": map[string]any{}}),
		"acme_lineage":       facet("https://acme.example/facets/AcmeDatasetFacet.json", map[string]any{"anything": []any{1, "two"}}),
	})
	input["inputFacets"] = map[string]any{
		"subset":                facet("https://openlineage.io/spec/facets/1-0-0/BaseSubsetDatasetFacet.json", map[string]any{"inputCondition": map[string]any{"type": "location", "locations": []any{"s3://lake/orders/2026"}}}),
		"inputStatistics":       facet("https://openlineage.io/spec/facets/1-0-0/InputStatisticsInputDatasetFacet.json", map[string]any{"rowCount": 990, "size": 60000, "fileCount": 4}),
		"dataQualityAssertions": facet("https://openlineage.io/spec/facets/1-1-0/DataQualityAssertionsDatasetFacet.json", map[string]any{"assertions": []any{map[string]any{"assertion": "not_null", "success": true, "column": "id"}}}),
	}
	output := pgTable("summary")
	output["outputFacets"] = map[string]any{
		"outputStatistics": facet("https://openlineage.io/spec/facets/1-0-2/OutputStatisticsOutputDatasetFacet.json", map[string]any{"rowCount": 12, "size": 2048, "fileCount": 1}),
	}
	ev := runEvent("COMPLETE", "r", "summarise", []ds{input}, []ds{output})
	ev["job"].(map[string]any)["facets"] = map[string]any{
		"sql":                facet("https://openlineage.io/spec/facets/1-1-0/SQLJobFacet.json", map[string]any{"query": "select 1", "dialect": "postgres"}),
		"jobType":            facet("https://openlineage.io/spec/facets/2-0-3/JobTypeJobFacet.json", map[string]any{"processingType": "BATCH", "integration": "DBT", "jobType": "JOB"}),
		"documentation":      facet("https://openlineage.io/spec/facets/1-1-0/DocumentationJobFacet.json", map[string]any{"description": "Daily summary"}),
		"ownership":          facet("https://openlineage.io/spec/facets/1-0-1/OwnershipJobFacet.json", map[string]any{"owners": []any{map[string]any{"name": "team:analytics"}}}),
		"sourceCodeLocation": facet("https://openlineage.io/spec/facets/1-1-0/SourceCodeLocationJobFacet.json", map[string]any{"type": "git", "url": "https://github.com/acme/etl/blob/main/summary.sql", "repoUrl": "https://github.com/acme/etl", "path": "summary.sql", "branch": "main", "version": "abc123"}),
		"tags":               facet("https://openlineage.io/spec/facets/1-0-0/TagsJobFacet.json", map[string]any{"tags": []any{map[string]any{"key": "team", "value": "analytics"}}}),
		"lineage":            facet("https://openlineage.io/spec/facets/1-0-0/LineageJobFacet.json", map[string]any{"entries": []any{map[string]any{"namespace": pgNS, "name": "shop.public.summary", "type": "DATASET", "inputs": []any{map[string]any{"namespace": pgNS, "name": "shop.public.orders", "type": "DATASET"}}}}}),
	}
	ev["run"].(map[string]any)["facets"] = map[string]any{
		"parent":               facet("https://openlineage.io/spec/facets/1-2-0/ParentRunFacet.json", map[string]any{"run": map[string]any{"runId": "3f5e83fa-3480-44ff-99c5-ff943904e5e8"}, "job": map[string]any{"namespace": "airflow", "name": "nightly_dag"}}),
		"environmentVariables": facet("https://openlineage.io/spec/facets/1-0-0/EnvironmentVariablesRunFacet.json", map[string]any{"environmentVariables": []any{map[string]any{"name": "ENV", "value": "prod"}}}),
		"externalQuery":        facet("https://openlineage.io/spec/facets/1-0-2/ExternalQueryRunFacet.json", map[string]any{"externalQueryId": "q-1", "source": "bigquery"}),
		"acme_run":             facet("https://acme.example/facets/AcmeRunFacet.json", map[string]any{"cluster": "blue"}),
	}
	h.sendJSON(ev)

	orders := h.asset(pgMRN("orders"))
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Customer orders", *orders.Description)
	assert.Equal(t, "team:orders", orders.Metadata["owner"])
	assert.Equal(t, []any{"team:orders", "user:ada"}, orders.Metadata["owners"])
	assert.Subset(t, orders.Tags, []string{"openlineage", "pii", "tier:gold"})
	assert.NotContains(t, orders.Tags, "sensitive", "a column tag is not an asset tag")
	assert.Equal(t, "TABLE", orders.Metadata["dataset_type"])
	assert.Equal(t, "EXTERNAL", orders.Metadata["dataset_subtype"])
	assert.Equal(t, "iceberg", orders.Metadata["storage_layer"])
	assert.Equal(t, "parquet", orders.Metadata["file_format"])
	assert.Equal(t, "42", orders.Metadata["version"])
	assert.Equal(t, "postgres://pg:5432/shop", orders.Metadata["data_source"], "credentials are stripped")
	assert.Equal(t, "iceberg", orders.Metadata["catalog_framework"])
	assert.EqualValues(t, 1000, orders.Metadata["row_count"])
	assert.EqualValues(t, 65536, orders.Metadata["size_bytes"])
	assert.EqualValues(t, 990, orders.Metadata["input_row_count"])
	assert.EqualValues(t, 60000, orders.Metadata["input_size_bytes"])
	assert.NotContains(t, orders.Metadata, "openlineage_role")

	summary := h.asset(pgMRN("summary"))
	assert.EqualValues(t, 12, summary.Metadata["output_row_count"])
	assert.EqualValues(t, 2048, summary.Metadata["output_size_bytes"])
	assert.EqualValues(t, 1, summary.Metadata["output_file_count"])

	job := h.asset("mrn://asset/job/dbt/shop.summarise")
	require.NotNil(t, job.Description)
	assert.Equal(t, "Daily summary", *job.Description)
	require.NotNil(t, job.Query)
	assert.Equal(t, "select 1", *job.Query)
	assert.Equal(t, "sql", *job.QueryLanguage)
	assert.Equal(t, "postgres", job.Metadata["sql_dialect"])
	assert.Equal(t, "BATCH", job.Metadata["processing_type"])
	assert.Equal(t, "DBT", job.Metadata["integration"])
	assert.Equal(t, "team:analytics", job.Metadata["owner"])
	assert.Equal(t, "https://github.com/acme/etl/blob/main/summary.sql", job.Metadata["source_code_url"])
	assert.Equal(t, "https://github.com/acme/etl", job.Metadata["source_code_repo_url"])
	assert.Equal(t, "main", job.Metadata["source_code_branch"])
	assert.Contains(t, job.Tags, "team:analytics")

	// A later event without these facets keeps what the earlier one said.
	h.sendJSON(runEvent("COMPLETE", "r2", "summarise", []ds{pgTable("orders")}, []ds{pgTable("summary")}))
	again := h.asset(pgMRN("orders"))
	require.NotNil(t, again.Description)
	assert.Equal(t, "Customer orders", *again.Description)
	assert.Equal(t, "team:orders", again.Metadata["owner"])
	assert.Contains(t, again.Tags, "pii")
}

func TestSourceCodeFacetKeepsItsLanguage(t *testing.T) {
	h := newHarness(t)
	ev := runEvent("COMPLETE", "r", "py", nil, nil)
	ev["job"].(map[string]any)["facets"] = map[string]any{
		"sourceCode": facet("https://openlineage.io/spec/facets/1-0-1/SourceCodeJobFacet.json", map[string]any{"language": "Scala", "sourceCode": "spark.read"}),
	}
	h.sendJSON(ev)
	job := h.asset(jobMRN("py"))
	assert.Equal(t, "scala", *job.QueryLanguage)
}

func TestDeletedFacetIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.sendJSON(map[string]any{"eventTime": "2026-11-18T10:00:00Z", "producer": producer, "schemaURL": dsURL, "dataset": withFacets(pgTable("orders"), map[string]any{
		"documentation": facet("https://openlineage.io/spec/facets/1-1-0/DocumentationDatasetFacet.json", map[string]any{"description": "Old text", "_deleted": true}),
	})})
	assert.Nil(t, h.asset(pgMRN("orders")).Description)
}

func TestLegacyAssetsAreAdopted(t *testing.T) {
	h := newHarness(t)
	ctx := h.ctx
	// What 0.12 filed duck_lineage's attached table as, and a job before full job names.
	legacyDataset := "mrn://asset/dataset/openlineage/duckdb.pg.public.orders"
	legacyJob := "mrn://asset/task/airflow/prod.load"
	for _, m := range []struct{ mrn, typ, provider string }{{legacyDataset, "Dataset", "OpenLineage"}, {legacyJob, "Task", "Airflow"}} {
		mm, name := m.mrn, "x"
		_, err := h.assets.Create(ctx, asset.CreateInput{Name: &name, MRN: &mm, Type: m.typ, Providers: []string{m.provider}, CreatedBy: "openlineage", Writer: "openlineage", Role: asset.RoleReference, IsStub: m.typ == "Dataset"})
		require.NoError(t, err)
	}
	before := map[string]string{legacyDataset: h.asset(legacyDataset).ID, legacyJob: h.asset(legacyJob).ID}

	h.sendJSON(map[string]any{
		"eventType": "COMPLETE", "eventTime": "2026-11-18T10:00:00Z", "producer": "https://github.com/ilum-cloud/duck_lineage", "schemaURL": runURL,
		"run": map[string]any{"runId": "r"}, "job": map[string]any{"namespace": "duckdb", "name": "COPY_orders_8aff966c"},
		"inputs": []any{map[string]any{"namespace": "duckdb", "name": "pg.public.orders", "facets": map[string]any{
			"catalog": facet("https://openlineage.io/spec/facets/1-0-0/CatalogDatasetFacet.json", map[string]any{"framework": "duckdb", "name": "pg", "type": "postgres", "metadataUri": "postgres://pg:5432/shop"}),
		}}},
	})
	airflow := runEvent("COMPLETE", "r2", "dag_a.load", nil, nil)
	airflow["producer"] = "https://github.com/apache/airflow/tree/providers-openlineage/2.3.0"
	airflow["job"] = map[string]any{"namespace": "prod", "name": "dag_a.load", "facets": map[string]any{"jobType": facet("https://openlineage.io/spec/facets/2-0-3/JobTypeJobFacet.json", map[string]any{"processingType": "BATCH", "integration": "AIRFLOW", "jobType": "TASK"})}}
	h.sendJSON(airflow)

	moved := h.asset(pgMRN("orders"))
	assert.Equal(t, before[legacyDataset], moved.ID, "the old stub moved, keeping its id")
	job := h.asset("mrn://asset/task/airflow/prod.dag_a.load")
	assert.Equal(t, before[legacyJob], job.ID)
	aliases, err := h.assets.Aliases(ctx, moved.ID)
	require.NoError(t, err)
	assert.Contains(t, aliases, legacyDataset)

	// A second DAG's task with the same id is a separate job, not the one adopted above.
	airflow2 := runEvent("COMPLETE", "r3", "dag_b.load", nil, nil)
	airflow2["producer"] = airflow["producer"]
	airflow2["job"] = map[string]any{"namespace": "prod", "name": "dag_b.load", "facets": airflow["job"].(map[string]any)["facets"]}
	h.sendJSON(airflow2)
	assert.NotEqual(t, job.ID, h.asset("mrn://asset/task/airflow/prod.dag_b.load").ID)
}

func TestLegacyAssetWithOtherWritersIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	legacy := "mrn://asset/dataset/openlineage/duckdb.pg.public.orders"
	name := "orders"
	_, err := h.assets.Create(h.ctx, asset.CreateInput{Name: &name, MRN: &legacy, Type: "Dataset", Providers: []string{"OpenLineage"}, CreatedBy: "ada"})
	require.NoError(t, err)
	h.sendJSON(map[string]any{
		"eventType": "COMPLETE", "eventTime": "2026-11-18T10:00:00Z", "producer": "https://github.com/ilum-cloud/duck_lineage", "schemaURL": runURL,
		"run": map[string]any{"runId": "r"}, "job": map[string]any{"namespace": "duckdb", "name": "COPY"},
		"inputs": []any{map[string]any{"namespace": "duckdb", "name": "pg.public.orders", "facets": map[string]any{
			"catalog": facet("https://openlineage.io/spec/facets/1-0-0/CatalogDatasetFacet.json", map[string]any{"framework": "duckdb", "name": "pg", "type": "postgres", "metadataUri": "postgres://pg:5432/shop"}),
		}}},
	})
	assert.Contains(t, h.allMRNs(), legacy, "an asset a person wrote to is not moved")
	assert.Contains(t, h.allMRNs(), pgMRN("orders"))
}

func TestSymlinks(t *testing.T) {
	symlinked := func(name string) ds {
		return withFacets(ds{"namespace": "s3://lake", "name": "warehouse/" + name}, map[string]any{
			"symlinks": facet("https://openlineage.io/spec/facets/1-0-1/SymlinksDatasetFacet.json", map[string]any{"identifiers": []any{
				map[string]any{"namespace": "hive://metastore:9083", "name": "lake." + name, "type": "TABLE"},
			}}),
		})
	}
	hiveMRN := func(name string) string { return "mrn://asset/table/hive/lake." + name }

	t.Run("an event files against the asset the symlink names", func(t *testing.T) {
		h := newHarness(t)
		m := mrn.Asset("Table", "Hive", "lake", "orders").String()
		h.pluginRun("hive", "hive", []runs.CreateAssetInput{{Name: "orders", MRN: &m, Type: "Table", Providers: []string{"Hive"}}}, nil)
		run(h, "r", "j1", []ds{symlinked("orders")}, []ds{pgTable("b")})

		assert.NotContains(t, h.allMRNs(), `mrn://asset/bucket/s3/lake.warehouse-orders`, "no second asset for the files")
		requireGraph(t, graph{
			Nodes: []string{hiveMRN("orders"), jobMRN("j1"), pgMRN("b")},
			Edges: []string{e(hiveMRN("orders"), "CONSUMES", jobMRN("j1")), e(jobMRN("j1"), "PRODUCES", pgMRN("b"))},
		}, h.lineageOf(pgMRN("b"), 10, "upstream"))
		resolved, err := h.assets.ResolveMRN(h.ctx, "mrn://asset/bucket/s3/lake.warehouse-orders")
		require.NoError(t, err)
		assert.Equal(t, hiveMRN("orders"), resolved, "the files' MRN leads to the table")
	})
	t.Run("the symlink becomes an alias when neither exists", func(t *testing.T) {
		h := newHarness(t)
		run(h, "r", "j1", []ds{symlinked("orders")}, []ds{pgTable("b")})
		resolved, err := h.assets.ResolveMRN(h.ctx, hiveMRN("orders"))
		require.NoError(t, err)
		assert.Equal(t, "mrn://asset/bucket/s3/lake.warehouse-orders", resolved)
	})
	t.Run("a stub made before the symlink was known is merged into the table", func(t *testing.T) {
		h := newHarness(t)
		run(h, "r1", "j1", []ds{{"namespace": "s3://lake", "name": "warehouse/orders"}}, []ds{pgTable("b")})
		m := mrn.Asset("Table", "Hive", "lake", "orders").String()
		h.pluginRun("hive", "hive", []runs.CreateAssetInput{{Name: "orders", MRN: &m, Type: "Table", Providers: []string{"Hive"}}}, nil)
		run(h, "r2", "j1", []ds{symlinked("orders")}, []ds{pgTable("b")})
		assert.NotContains(t, h.allMRNs(), `mrn://asset/bucket/s3/lake.warehouse-orders`)
		requireGraph(t, graph{
			Nodes: []string{hiveMRN("orders"), jobMRN("j1"), pgMRN("b")},
			Edges: []string{e(hiveMRN("orders"), "CONSUMES", jobMRN("j1")), e(jobMRN("j1"), "PRODUCES", pgMRN("b"))},
		}, h.lineageOf(pgMRN("b"), 10, "upstream"))
	})
}

func TestAirflowDAGContainsTaskAndDbtProjectContainsModel(t *testing.T) {
	h := newHarness(t)
	airflowFacets := func(jobType string) map[string]any {
		return map[string]any{"jobType": facet("https://openlineage.io/spec/facets/2-0-3/JobTypeJobFacet.json", map[string]any{"processingType": "BATCH", "integration": "AIRFLOW", "jobType": jobType})}
	}
	task := runEvent("COMPLETE", "t1", "", []ds{pgTable("a")}, []ds{pgTable("b")})
	task["producer"] = "https://github.com/apache/airflow/tree/providers-openlineage/2.3.0"
	task["job"] = map[string]any{"namespace": "prod", "name": "orders_etl.load", "facets": airflowFacets("TASK")}
	task["run"] = map[string]any{"runId": "t1", "facets": map[string]any{
		"parent": facet("https://openlineage.io/spec/facets/1-2-0/ParentRunFacet.json", map[string]any{"run": map[string]any{"runId": "d1"}, "job": map[string]any{"namespace": "prod", "name": "orders_etl"}}),
	}}
	h.sendJSON(task)

	dag, taskMRN := "mrn://asset/dag/airflow/prod.orders_etl", "mrn://asset/task/airflow/prod.orders_etl.load"
	requireGraph(t, graph{
		Nodes: []string{dag, taskMRN, pgMRN("a"), pgMRN("b")},
		Edges: []string{e(dag, "CONTAINS", taskMRN), e(pgMRN("a"), "CONSUMES", taskMRN), e(taskMRN, "PRODUCES", pgMRN("b"))},
	}, h.lineageOf(taskMRN, 10, "both"))
}

// A rename or drop of a table its own plugin catalogues: the rename moves the plugin's asset, keeping its id, and the drop keeps it, marked dropped, for the plugin's next run to remove.
func TestLifecycleOnPluginOwnedTables(t *testing.T) {
	h := newHarness(t)
	table := func(name string) runs.CreateAssetInput {
		m := mrn.Asset("Table", "PostgreSQL", "shop", "public", name).String()
		return runs.CreateAssetInput{Name: name, MRN: &m, Type: "Table", Providers: []string{"PostgreSQL"}}
	}
	h.pluginRun("shop-pg", "postgresql", []runs.CreateAssetInput{table("old_name"), table("doomed")}, nil)
	before := h.asset(pgMRN("old_name")).ID

	lifecycle := func(state string, prev string) map[string]any {
		f := map[string]any{"lifecycleStateChange": state}
		if prev != "" {
			f["previousIdentifier"] = map[string]any{"namespace": pgNS, "name": "shop.public." + prev}
		}
		return map[string]any{"lifecycleStateChange": facet("https://openlineage.io/spec/facets/1-0-1/LifecycleStateChangeDatasetFacet.json", f)}
	}
	renamed := withFacets(pgTable("new_name"), lifecycle("RENAME", "old_name"))
	h.sendJSON(runEvent("START", "r1", "rename", nil, []ds{renamed}))
	assert.NotContains(t, h.allMRNs(), pgMRN("new_name"), "the new name is not created before the rename has finished")
	h.sendJSON(runEvent("COMPLETE", "r1", "rename", nil, []ds{renamed}))
	moved := h.asset(pgMRN("new_name"))
	assert.Equal(t, before, moved.ID, "the plugin's asset moved")
	assert.False(t, moved.IsStub)
	assert.NotContains(t, h.allMRNs(), pgMRN("old_name"))
	resolved, err := h.assets.ResolveMRN(h.ctx, pgMRN("old_name"))
	require.NoError(t, err)
	assert.Equal(t, pgMRN("new_name"), resolved)

	dropped := withFacets(pgTable("doomed"), lifecycle("DROP", ""))
	run(h, "r2", "drop", nil, []ds{dropped})
	kept := h.asset(pgMRN("doomed"))
	assert.False(t, kept.IsStub)
	assert.Equal(t, "DROP", kept.Metadata["lifecycle_state"])
	assert.NotContains(t, h.allEdges(), e(jobMRN("drop"), "PRODUCES", pgMRN("doomed")), "a drop does not link the job to the table")

	// A drop of a table nobody has seen creates nothing.
	run(h, "r3", "drop", nil, []ds{withFacets(pgTable("never_seen"), lifecycle("DROP", ""))})
	assert.NotContains(t, h.allMRNs(), pgMRN("never_seen"))
}
