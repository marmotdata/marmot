package spline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// The JSON below was copied from a Spline 1.0.0-RC3 server (absaoss/spline-rest-server)
// seeded through its producer API, with the ids shortened. Keeping the real
// shapes means the unit tests fail if the plugin stops reading the response
// the way the server actually writes it.

const (
	planDaily  = "11111111-1111-1111-1111-111111111111"
	planEvents = "22222222-2222-2222-2222-222222222222"
	planNight  = "33333333-3333-3333-3333-333333333333"
)

// fakeSpline is an httptest server that answers the three consumer endpoints
// the plugin calls.
type fakeSpline struct {
	server *httptest.Server
	// Requests counts calls per path, so tests can assert a plan is fetched
	// once even when several runs share it.
	Requests map[string]int
	// PageSizeSeen is the page size the plugin asked for.
	PageSizeSeen string
	// IgnorePaging makes every page return page one, like a gateway that does
	// not honour pageNum.
	IgnorePaging bool
}

func newFakeSpline(t *testing.T, events []map[string]any) *fakeSpline {
	t.Helper()

	fake := &fakeSpline{Requests: map[string]int{}}

	mux := http.NewServeMux()

	mux.HandleFunc("/about/version", func(w http.ResponseWriter, r *http.Request) {
		fake.Requests[r.URL.Path]++
		writeJSON(w, map[string]any{
			"build": map[string]any{"version": "1.0.0-RC3", "revision": "da036c4"},
		})
	})

	mux.HandleFunc("/consumer/execution-events", func(w http.ResponseWriter, r *http.Request) {
		fake.Requests[r.URL.Path]++
		fake.PageSizeSeen = r.URL.Query().Get("pageSize")

		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if pageSize <= 0 {
			pageSize = 10
		}
		pageNum, _ := strconv.Atoi(r.URL.Query().Get("pageNum"))
		if pageNum <= 0 || fake.IgnorePaging {
			pageNum = 1
		}

		from := (pageNum - 1) * pageSize
		to := from + pageSize
		if from > len(events) {
			from = len(events)
		}
		if to > len(events) {
			to = len(events)
		}

		writeJSON(w, map[string]any{
			"items":          events[from:to],
			"totalCount":     len(events),
			"pageNum":        pageNum,
			"pageSize":       pageSize,
			"totalDateRange": []int64{},
		})
	})

	mux.HandleFunc("/consumer/lineage-detailed", func(w http.ResponseWriter, r *http.Request) {
		fake.Requests[r.URL.Path]++
		plan, ok := lineageDetailedFixtures[r.URL.Query().Get("execId")]
		if !ok {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(plan))
	})

	mux.HandleFunc("/consumer/attribute-lineage-and-impact", func(w http.ResponseWriter, r *http.Request) {
		fake.Requests[r.URL.Path]++
		graph, ok := attributeLineageFixtures[r.URL.Query().Get("attributeId")]
		if !ok {
			writeJSON(w, map[string]any{
				"lineage": map[string]any{"nodes": []any{}, "edges": []any{}},
				"impact":  map[string]any{"nodes": []any{}, "edges": []any{}},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(graph))
	})

	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeSpline) URL() string { return f.server.URL }

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// recentMillis is a timestamp inside the default seven day window.
func recentMillis(minutesAgo int) int64 {
	return time.Now().Add(-time.Duration(minutesAgo) * time.Minute).UnixMilli()
}

// fixtureEvents are four runs of three applications, one of which ran twice
// and one of which failed.
func fixtureEvents() []map[string]any {
	return []map[string]any{
		{
			"durationNs":       nil,
			"error":            map[string]any{"message": "Job aborted due to stage failure"},
			"dataSourceName":   "nightly",
			"dataSourceUri":    "s3a://marmot-lake/reports/nightly",
			"executionEventId": planNight + ":mtsfab3y",
			"executionPlanId":  planNight,
			"frameworkName":    "spark 3.5.0",
			"applicationName":  "nightly-report",
			"applicationId":    "application_1699000000000_0004",
			"timestamp":        recentMillis(5),
			"dataSourceType":   nil,
			"append":           false,
			"extra":            map[string]any{"appName": "nightly-report", "appId": "application_1699000000000_0004"},
			"labels":           map[string]any{},
		},
		{
			"durationNs":       9500000000,
			"error":            nil,
			"dataSourceName":   "warehouse:public.order_summary",
			"dataSourceUri":    "jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary",
			"executionEventId": planDaily + ":mtsf3vmm",
			"executionPlanId":  planDaily,
			"frameworkName":    "spark 3.5.0",
			"applicationName":  "daily-etl",
			"applicationId":    "application_1699000000000_0002",
			"timestamp":        recentMillis(10),
			"dataSourceType":   nil,
			"append":           false,
			"extra":            map[string]any{"appName": "daily-etl", "appId": "application_1699000000000_0002"},
			"labels":           map[string]any{},
		},
		{
			"durationNs":       4200000000,
			"error":            nil,
			"dataSourceName":   "orders",
			"dataSourceUri":    "hive://sales/orders",
			"executionEventId": planEvents + ":mtsee5pa",
			"executionPlanId":  planEvents,
			"frameworkName":    "spark 3.5.0",
			"applicationName":  "events-loader",
			"applicationId":    "application_1699000000000_0003",
			"timestamp":        recentMillis(30),
			"dataSourceType":   nil,
			"append":           true,
			"extra":            map[string]any{"appName": "events-loader", "appId": "application_1699000000000_0003"},
			"labels":           map[string]any{},
		},
		{
			"durationNs":       12000000000,
			"error":            nil,
			"dataSourceName":   "warehouse:public.order_summary",
			"dataSourceUri":    "jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary",
			"executionEventId": planDaily + ":mtsdbkta",
			"executionPlanId":  planDaily,
			"frameworkName":    "spark 3.5.0",
			"applicationName":  "daily-etl",
			"applicationId":    "application_1699000000000_0001",
			"timestamp":        recentMillis(60),
			"dataSourceType":   nil,
			"append":           false,
			"extra":            map[string]any{"appName": "daily-etl", "appId": "application_1699000000000_0001"},
			"labels":           map[string]any{},
		},
	}
}

var lineageDetailedFixtures = map[string]string{
	planDaily: `{
  "executionPlan": {
    "_id": "11111111-1111-1111-1111-111111111111",
    "name": "daily-etl",
    "systemInfo": {"name": "spark", "version": "3.5.0"},
    "agentInfo": {"name": "spline", "version": "1.0.0"},
    "extra": {
      "__spline_msg_size": [1384, 1384],
      "appName": "daily-etl",
      "attributes": [
        {"id": "11111111-1111-1111-1111-111111111111:attr-total", "name": "total", "dataTypeId": "dt-double"},
        {"id": "11111111-1111-1111-1111-111111111111:attr-order-id", "name": "order_id", "dataTypeId": "dt-long"},
        {"id": "11111111-1111-1111-1111-111111111111:attr-customer-name", "name": "customer_name", "dataTypeId": "dt-string"},
        {"id": "11111111-1111-1111-1111-111111111111:attr-revenue", "name": "revenue", "dataTypeId": "dt-double"}
      ]
    },
    "inputs": [
      {"sourceType": null, "source": "jdbc:postgresql://pg.internal:5432/warehouse:public.orders"},
      {"sourceType": null, "source": "jdbc:postgresql://pg.internal:5432/warehouse:public.customers"}
    ],
    "output": {"sourceType": null, "source": "jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary"}
  },
  "graph": {
    "nodes": [
      {"_id": "11111111-1111-1111-1111-111111111111:op-1", "_type": "Read", "name": "LogicalRelation", "properties": {}},
      {"_id": "11111111-1111-1111-1111-111111111111:op-0", "_type": "Write", "name": "SaveIntoDataSourceCommand", "properties": {}}
    ],
    "edges": [
      {"source": "11111111-1111-1111-1111-111111111111:op-1", "target": "11111111-1111-1111-1111-111111111111:op-0"}
    ]
  }
}`,
	planEvents: `{
  "executionPlan": {
    "_id": "22222222-2222-2222-2222-222222222222",
    "name": "events-loader",
    "systemInfo": {"name": "spark", "version": "3.5.0"},
    "agentInfo": {"name": "spline", "version": "1.0.0"},
    "extra": {
      "appName": "events-loader",
      "attributes": [
        {"id": "22222222-2222-2222-2222-222222222222:attr-event-id", "name": "event_id", "dataTypeId": "dt-string"}
      ]
    },
    "inputs": [{"sourceType": null, "source": "s3://marmot-lake/raw/events"}],
    "output": {"sourceType": null, "source": "hive://sales/orders"}
  },
  "graph": {"nodes": [], "edges": []}
}`,
	planNight: `{
  "executionPlan": {
    "_id": "33333333-3333-3333-3333-333333333333",
    "name": "nightly-report",
    "systemInfo": {"name": "spark", "version": "3.5.0"},
    "agentInfo": {"name": "spline", "version": "1.0.0"},
    "extra": {
      "appName": "nightly-report",
      "attributes": [
        {"id": "33333333-3333-3333-3333-333333333333:attr-order-id", "name": "order_id", "dataTypeId": "dt-long"}
      ]
    },
    "inputs": [{"sourceType": null, "source": "jdbc:postgresql://pg.internal:5432/warehouse:public.orders"}],
    "output": {"sourceType": null, "source": "s3a://marmot-lake/reports/nightly"}
  },
  "graph": {"nodes": [], "edges": []}
}`,
}

var attributeLineageFixtures = map[string]string{
	// revenue was computed from total and order_id. The edges point from the
	// derived column to the columns it came from.
	planDaily + ":attr-revenue": `{
  "lineage": {
    "nodes": [
      {"_id": "11111111-1111-1111-1111-111111111111:attr-total", "name": "total", "originOpId": "11111111-1111-1111-1111-111111111111:op-1", "transOpIds": []},
      {"_id": "11111111-1111-1111-1111-111111111111:attr-order-id", "name": "order_id", "originOpId": "11111111-1111-1111-1111-111111111111:op-1", "transOpIds": []},
      {"_id": "11111111-1111-1111-1111-111111111111:attr-revenue", "name": "revenue", "originOpId": "11111111-1111-1111-1111-111111111111:op-3", "transOpIds": []}
    ],
    "edges": [
      {"source": "11111111-1111-1111-1111-111111111111:attr-revenue", "target": "11111111-1111-1111-1111-111111111111:attr-total"},
      {"source": "11111111-1111-1111-1111-111111111111:attr-revenue", "target": "11111111-1111-1111-1111-111111111111:attr-order-id"}
    ]
  },
  "impact": {"nodes": [], "edges": []}
}`,
	// total is read straight from the input, so it has no sources.
	planDaily + ":attr-total": `{
  "lineage": {
    "nodes": [
      {"_id": "11111111-1111-1111-1111-111111111111:attr-total", "name": "total", "originOpId": "11111111-1111-1111-1111-111111111111:op-1", "transOpIds": []}
    ],
    "edges": []
  },
  "impact": {"nodes": [], "edges": []}
}`,
}
