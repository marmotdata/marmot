package lineage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventRecorder stands in for the lineage service and records each OpenLineage event it is handed, failing the ones failFor picks.
type eventRecorder struct {
	lineage.Service
	mu      sync.Mutex
	events  []*lineage.RunEvent
	failFor func(*lineage.RunEvent) error
}

func (s *eventRecorder) ProcessOpenLineageEvent(_ context.Context, e *lineage.RunEvent, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failFor != nil {
		if err := s.failFor(e); err != nil {
			return err
		}
	}
	s.events = append(s.events, e)
	return nil
}

func olEvent(eventType, runID string) map[string]any {
	return map[string]any{
		"eventType": eventType,
		"eventTime": "2026-11-18T10:00:00Z",
		"producer":  "https://github.com/marmotdata/marmot/tests",
		"schemaURL": "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/RunEvent",
		"run":       map[string]any{"runId": runID},
		"job":       map[string]any{"namespace": "shop", "name": "load"},
	}
}

func post(t *testing.T, handler http.HandlerFunc, body []byte, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lineage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func gzipped(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestOpenLineageEndpointTransport(t *testing.T) {
	valid := marshal(t, olEvent("START", "3f5e83fa-3480-44ff-99c5-ff943904e5e8"))
	cases := []struct {
		name     string
		body     []byte
		header   map[string]string
		want     int
		wantText string
		recorded int
	}{
		{name: "plain JSON event", body: valid, want: http.StatusOK, recorded: 1},
		{name: "gzip body", body: gzipped(t, valid), header: map[string]string{"Content-Encoding": "gzip"}, want: http.StatusOK, recorded: 1},
		{name: "x-gzip body", body: gzipped(t, valid), header: map[string]string{"Content-Encoding": "x-gzip"}, want: http.StatusOK, recorded: 1},
		{name: "gzip header on a body that is not gzip", body: valid, header: map[string]string{"Content-Encoding": "gzip"}, want: http.StatusBadRequest, wantText: "gzip"},
		{name: "unsupported encoding", body: valid, header: map[string]string{"Content-Encoding": "br"}, want: http.StatusUnsupportedMediaType, wantText: "br"},
		{name: "invalid JSON", body: []byte(`{"eventType":`), want: http.StatusBadRequest, wantText: "not valid JSON"},
		{name: "missing runId", body: marshal(t, olEvent("START", "")), want: http.StatusBadRequest, wantText: "run.runId is required"},
		{name: "empty body", body: nil, want: http.StatusBadRequest, wantText: "empty"},
		{name: "over the size limit", body: bytes.Repeat([]byte(" "), maxOpenLineageBody+1), want: http.StatusRequestEntityTooLarge},
		{name: "gzip that inflates past the size limit", body: gzipped(t, bytes.Repeat([]byte(" "), maxOpenLineageBody+1)), header: map[string]string{"Content-Encoding": "gzip"}, want: http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &eventRecorder{}
			h := &Handler{lineageService: svc}
			rec := post(t, h.ingestOpenLineageEvent, c.body, c.header)
			assert.Equal(t, c.want, rec.Code, rec.Body.String())
			if c.wantText != "" {
				assert.Contains(t, rec.Body.String(), c.wantText)
			}
			assert.Len(t, svc.events, c.recorded)
		})
	}
}

func TestOpenLineageEndpointTakesALargeEvent(t *testing.T) {
	// A Spark event with a big logical plan facet runs to megabytes.
	e := olEvent("COMPLETE", "3f5e83fa-3480-44ff-99c5-ff943904e5e8")
	e["run"].(map[string]any)["facets"] = map[string]any{
		"spark.logicalPlan": map[string]any{"_producer": "p", "_schemaURL": "s", "plan": strings.Repeat("x", 6<<20)},
	}
	svc := &eventRecorder{}
	h := &Handler{lineageService: svc}
	body := marshal(t, e)
	rec := post(t, h.ingestOpenLineageEvent, gzipped(t, body), map[string]string{"Content-Encoding": "gzip"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, svc.events, 1)
}

func TestOpenLineageEndpointSeparatesClientAndServerErrors(t *testing.T) {
	body := marshal(t, olEvent("START", "3f5e83fa-3480-44ff-99c5-ff943904e5e8"))
	t.Run("processing failure is a 500 so the client retries", func(t *testing.T) {
		h := &Handler{lineageService: &eventRecorder{failFor: func(*lineage.RunEvent) error { return errors.New("database down") }}}
		rec := post(t, h.ingestOpenLineageEvent, body, nil)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotContains(t, rec.Body.String(), "database down")
	})
	t.Run("an event the service cannot file is a 400", func(t *testing.T) {
		h := &Handler{lineageService: &eventRecorder{failFor: func(*lineage.RunEvent) error { return &lineage.EventError{Reason: "dataset name is not valid"} }}}
		rec := post(t, h.ingestOpenLineageEvent, body, nil)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "dataset name is not valid")
	})
}

func TestBatchEndpointTakesOpenLineageEvents(t *testing.T) {
	svc := &eventRecorder{failFor: func(e *lineage.RunEvent) error {
		if e.Run.RunID == "boom" {
			return errors.New("database down")
		}
		return nil
	}}
	h := &Handler{lineageService: svc}
	batch := []any{
		olEvent("START", "run-1"),
		olEvent("COMPLETE", "run-1"),
		map[string]any{"eventTime": "2026-11-18T10:00:00Z", "job": map[string]any{"namespace": "shop"}},
		olEvent("COMPLETE", "boom"),
		map[string]any{"eventTime": "2026-11-18T10:00:00Z", "schemaURL": "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/DatasetEvent", "dataset": map[string]any{"namespace": "kafka://b:9092", "name": "orders"}},
	}
	rec := post(t, h.batchCreateLineage, gzipped(t, marshal(t, batch)), map[string]string{"Content-Encoding": "gzip"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp OpenLineageBatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "partial_success", resp.Status)
	assert.Equal(t, OpenLineageBatchSummary{Received: 5, Successful: 3, Failed: 2, Retriable: 1, NonRetriable: 1}, resp.Summary)
	require.Len(t, resp.FailedEvents, 2)
	assert.Equal(t, 2, resp.FailedEvents[0].Index)
	assert.False(t, resp.FailedEvents[0].Retriable)
	assert.Contains(t, resp.FailedEvents[0].Reason, "job.name is required")
	assert.Equal(t, OpenLineageFailedEvent{Index: 3, Reason: "Server error", Retriable: true}, resp.FailedEvents[1])

	require.Len(t, svc.events, 3)
	assert.Equal(t, []string{"START", "COMPLETE"}, []string{svc.events[0].EventType, svc.events[1].EventType}, "events are processed in the order sent")
	assert.Equal(t, lineage.KindDataset, svc.events[2].Kind())
}

func TestBatchEndpointReportsSuccessForACleanBatch(t *testing.T) {
	svc := &eventRecorder{}
	h := &Handler{lineageService: svc}
	rec := post(t, h.batchCreateLineage, marshal(t, []any{olEvent("START", "r"), olEvent("COMPLETE", "r")}), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp OpenLineageBatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, OpenLineageBatchSummary{Received: 2, Successful: 2}, resp.Summary)
	assert.Empty(t, resp.FailedEvents)
}

func TestBatchEndpointStillTakesEdges(t *testing.T) {
	svc := &recordingService{}
	h := &Handler{lineageService: svc}
	edges := []lineage.LineageEdge{{Source: "mrn://asset/table/postgresql/a", Target: "mrn://asset/table/postgresql/b"}}
	rec := post(t, h.batchCreateLineage, marshal(t, edges), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var results []BatchLineageResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	require.Len(t, results, 1)
	assert.Equal(t, "created", results[0].Status)

	rec = post(t, h.batchCreateLineage, []byte(`{"not":"an array"}`), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpenLineageEndpointHandlesConcurrentRequests(t *testing.T) {
	svc := &eventRecorder{}
	h := &Handler{lineageService: svc}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := post(t, h.ingestOpenLineageEvent, marshal(t, olEvent("START", "3f5e83fa-3480-44ff-99c5-ff943904e5e8")), nil)
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}
	wg.Wait()
	assert.Len(t, svc.events, 32)
}
