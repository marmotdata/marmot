package plugin

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// TestInstanceManagerTrino exercises the full direct query path against a
// live Trino from testenv (docker compose up trino) through a locally built
// trino plugin binary. Gated so CI without the stack skips it:
//
//	MARMOT_TEST_TRINO=1 go test ./internal/plugin/ -run TestInstanceManagerTrino -v
func TestInstanceManagerTrino(t *testing.T) {
	if os.Getenv("MARMOT_TEST_TRINO") != "1" {
		t.Skip("set MARMOT_TEST_TRINO=1 to run against the testenv Trino")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(home, ".marmot", "plugins", "marmot-plugin-trino")
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("trino plugin binary not found at %s", binary)
	}

	if _, err := GetRegistry().Get("trino"); err != nil {
		if _, err := registerExternalPlugin(binary); err != nil {
			t.Fatalf("registering trino plugin: %v", err)
		}
	}

	manager := NewInstanceManager(time.Minute)
	defer manager.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := map[string]any{"host": "localhost", "port": 8090, "user": "marmot"}

	plan, err := manager.PlanQuery(ctx, "trino", config, pluginsdk.QueryRequest{
		Statement: "SELECT status, count(*) FROM postgresql.public.orders GROUP BY status",
	})
	if err != nil {
		t.Fatalf("PlanQuery: %v", err)
	}
	if plan == nil || len(plan.ReferencedMRNs) == 0 {
		t.Fatalf("expected referenced MRNs, got %+v", plan)
	}
	if plan.ReferencedMRNs[0] != "mrn://table/postgresql/orders" {
		t.Fatalf("expected mrn://table/postgresql/orders, got %v", plan.ReferencedMRNs)
	}

	stream, err := manager.ExecuteQuery(ctx, "trino", config, pluginsdk.QueryRequest{
		Statement: "SELECT status, count(*) AS orders FROM postgresql.public.orders GROUP BY status",
		MaxRows:   100,
	})
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	defer stream.Close()

	var columns []pluginsdk.QueryColumn
	var rows int
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Recv: %v", err)
		}
		if chunk.Columns != nil {
			columns = chunk.Columns
		}
		rows += len(chunk.Rows)
	}
	if len(columns) != 2 || columns[0].Name != "status" {
		t.Fatalf("unexpected columns: %+v", columns)
	}
	if rows == 0 {
		t.Fatal("expected rows from seeded orders table")
	}

	// The second call must reuse the instance rather than spawning again.
	if _, err := manager.PlanQuery(ctx, "trino", config, pluginsdk.QueryRequest{Statement: "SELECT 1"}); err != nil {
		t.Fatalf("second PlanQuery: %v", err)
	}
	statuses := manager.Status()
	if len(statuses) != 1 || statuses[0].Restarts != 0 {
		t.Fatalf("expected one instance with no restarts, got %+v", statuses)
	}
}
