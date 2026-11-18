package lineage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	runSchemaURL     = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/RunEvent"
	jobSchemaURL     = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/JobEvent"
	datasetSchemaURL = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/DatasetEvent"
	testProducer     = "https://github.com/marmotdata/marmot/tests"
)

// facet builds a facet carrying the base fields every facet needs.
func facet(schema string, fields map[string]any) map[string]any {
	out := map[string]any{"_producer": testProducer, "_schemaURL": schema}
	for k, v := range fields {
		out[k] = v
	}
	return out
}

func runEventJSON(eventType string) map[string]any {
	return map[string]any{
		"eventType": eventType,
		"eventTime": "2026-11-18T10:00:00.000Z",
		"producer":  testProducer,
		"schemaURL": runSchemaURL,
		"run":       map[string]any{"runId": "3f5e83fa-3480-44ff-99c5-ff943904e5e8"},
		"job":       map[string]any{"namespace": "shop", "name": "load_orders"},
		"inputs":    []any{map[string]any{"namespace": "postgres://pg:5432", "name": "shop.public.orders"}},
		"outputs":   []any{map[string]any{"namespace": "postgres://pg:5432", "name": "shop.public.orders_copy"}},
	}
}

func TestParseEventAcceptsEveryRunEventType(t *testing.T) {
	for _, eventType := range []string{"START", "RUNNING", "COMPLETE", "ABORT", "FAIL", "OTHER"} {
		t.Run(eventType, func(t *testing.T) {
			raw := mustJSON(t, runEventJSON(eventType))
			requireSpecValid(t, raw)
			ev, err := ParseEvent(raw)
			require.NoError(t, err)
			assert.Equal(t, KindRun, ev.Kind())
			assert.Equal(t, eventType, ev.EventType)
		})
	}
}

func TestParseEventNormalisesTheEventType(t *testing.T) {
	cases := map[string]string{"complete": "COMPLETE", " Fail ": "FAIL", "": "OTHER", "PAUSED": "OTHER"}
	for in, want := range cases {
		e := runEventJSON(in)
		if in == "" {
			delete(e, "eventType")
		}
		ev, err := ParseEvent(mustJSON(t, e))
		require.NoError(t, err, in)
		assert.Equal(t, want, ev.EventType, "eventType %q", in)
	}
}

func TestParseEventWorksOutTheKind(t *testing.T) {
	jobOnly := map[string]any{
		"eventTime": "2026-11-18T10:00:00Z", "producer": testProducer,
		"job":    map[string]any{"namespace": "shop", "name": "nightly"},
		"inputs": []any{map[string]any{"namespace": "kafka://broker:9092", "name": "orders"}},
	}
	datasetOnly := map[string]any{
		"eventTime": "2026-11-18T10:00:00Z", "producer": testProducer,
		"dataset": map[string]any{"namespace": "postgres://pg:5432", "name": "shop.public.orders"},
	}
	withURL := func(m map[string]any, u string) map[string]any {
		c := map[string]any{}
		for k, v := range m {
			c[k] = v
		}
		c["schemaURL"] = u
		return c
	}
	cases := []struct {
		name string
		body map[string]any
		want EventKind
	}{
		{"run event by $defs schemaURL", runEventJSON("START"), KindRun},
		{"job event by $defs schemaURL", withURL(jobOnly, jobSchemaURL), KindJob},
		{"dataset event by $defs schemaURL", withURL(datasetOnly, datasetSchemaURL), KindDataset},
		{"job event by the older #/definitions/ schemaURL", withURL(jobOnly, "https://openlineage.io/spec/1-0-5/OpenLineage.json#/definitions/JobEvent"), KindJob},
		{"job event without a schemaURL", jobOnly, KindJob},
		{"dataset event without a schemaURL", datasetOnly, KindDataset},
		{"run event with an unknown schemaURL", withURL(runEventJSON("START"), "https://example.com/custom.json"), KindRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := mustJSON(t, c.body)
			if _, ok := c.body["schemaURL"]; ok && c.body["schemaURL"] != "https://example.com/custom.json" {
				requireSpecValid(t, raw)
			}
			ev, err := ParseEvent(raw)
			require.NoError(t, err)
			assert.Equal(t, c.want, ev.Kind())
		})
	}
}

func TestParseEventRejectsWhatItCannotFile(t *testing.T) {
	without := func(path ...string) map[string]any {
		e := runEventJSON("START")
		m := e
		for _, p := range path[:len(path)-1] {
			m = m[p].(map[string]any)
		}
		delete(m, path[len(path)-1])
		return e
	}
	set := func(k string, v any) map[string]any {
		e := runEventJSON("START")
		e[k] = v
		return e
	}
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", "empty"},
		{"not JSON", "{not json", "not valid JSON"},
		{"an array", `[{"eventTime":"2026-11-18T10:00:00Z"}]`, "JSON object"},
		{"nothing to file", `{"eventTime":"2026-11-18T10:00:00Z","producer":"x"}`, "no run, job or dataset"},
		{"no eventTime", string(mustJSON(t, without("eventTime"))), "eventTime is required"},
		{"unreadable eventTime", string(mustJSON(t, set("eventTime", "yesterday"))), "not an ISO 8601"},
		{"eventTime not a string", string(mustJSON(t, set("eventTime", 1731924000))), "does not match"},
		{"no runId", string(mustJSON(t, without("run", "runId"))), "run.runId is required"},
		{"no job", string(mustJSON(t, without("job"))), "job is required"},
		{"no job name", string(mustJSON(t, without("job", "name"))), "job.name is required"},
		{"no job namespace", string(mustJSON(t, without("job", "namespace"))), "job.namespace is required"},
		{"input without a name", string(mustJSON(t, set("inputs", []any{map[string]any{"namespace": "x"}}))), "inputs[0].name is required"},
		{"output without a namespace", string(mustJSON(t, set("outputs", []any{map[string]any{"name": "x"}}))), "outputs[0].namespace is required"},
		{"dataset event without a name", `{"eventTime":"2026-11-18T10:00:00Z","schemaURL":"` + datasetSchemaURL + `","dataset":{"namespace":"x"}}`, "dataset.name is required"},
		{"dataset event without a dataset", `{"eventTime":"2026-11-18T10:00:00Z","schemaURL":"` + datasetSchemaURL + `"}`, "dataset is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseEvent([]byte(c.body))
			require.Error(t, err)
			assert.True(t, IsEventError(err), "a bad event must be a client error: %v", err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func TestParseEventAcceptsUnusualButValidEvents(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
		check  func(*testing.T, *RunEvent)
	}{
		{"eventTime without an offset is UTC", func(e map[string]any) { e["eventTime"] = "2026-11-18T10:00:00.123456" }, func(t *testing.T, ev *RunEvent) {
			assert.Equal(t, time.Date(2026, 11, 18, 10, 0, 0, 123456000, time.UTC), ev.EventTime)
		}},
		{"eventTime with an offset", func(e map[string]any) { e["eventTime"] = "2026-11-18T12:00:00+02:00" }, func(t *testing.T, ev *RunEvent) {
			assert.Equal(t, time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), ev.EventTime)
		}},
		{"runId that is not a UUID", func(e map[string]any) { e["run"] = map[string]any{"runId": "scheduled__2026-11-18T00:00:00"} }, func(t *testing.T, ev *RunEvent) {
			assert.Equal(t, "scheduled__2026-11-18T00:00:00", ev.Run.RunID)
		}},
		{"no producer or schemaURL", func(e map[string]any) { delete(e, "producer"); delete(e, "schemaURL") }, func(t *testing.T, ev *RunEvent) {
			assert.Equal(t, KindRun, ev.Kind())
		}},
		{"unknown top-level fields and custom facets", func(e map[string]any) {
			e["x-vendor"] = map[string]any{"anything": true}
			e["run"].(map[string]any)["facets"] = map[string]any{
				"acme_runtime": facet("https://acme.example/facets/RuntimeRunFacet.json", map[string]any{"cluster": "blue", "nested": map[string]any{"a": []any{1, 2}}}),
			}
			e["job"].(map[string]any)["facets"] = map[string]any{
				"acme_owner": facet("https://acme.example/facets/OwnerJobFacet.json", map[string]any{"team": "data"}),
			}
		}, func(t *testing.T, ev *RunEvent) {
			assert.Contains(t, ev.Run.Facets, "acme_runtime")
			assert.Contains(t, ev.Job.Facets, "acme_owner")
		}},
		{"no inputs or outputs", func(e map[string]any) { delete(e, "inputs"); delete(e, "outputs") }, func(t *testing.T, ev *RunEvent) {
			assert.Empty(t, ev.Inputs)
			assert.Empty(t, ev.Outputs)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := runEventJSON("COMPLETE")
			c.mutate(e)
			raw := mustJSON(t, e)
			if _, ok := e["schemaURL"]; ok && c.name != "runId that is not a UUID" && c.name != "eventTime without an offset is UTC" {
				requireSpecValid(t, raw)
			}
			ev, err := ParseEvent(raw)
			require.NoError(t, err)
			c.check(t, ev)
		})
	}
}

func TestRunEventRoundTripsThroughJSON(t *testing.T) {
	ev, err := ParseEvent(mustJSON(t, runEventJSON("START")))
	require.NoError(t, err)
	out, err := json.Marshal(ev)
	require.NoError(t, err)
	again, err := ParseEvent(out)
	require.NoError(t, err)
	assert.Equal(t, ev.EventTime, again.EventTime)
	assert.Equal(t, ev.Run.RunID, again.Run.RunID)
}
