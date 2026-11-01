package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceManagerReusesAndRestarts(t *testing.T) {
	registerFakeQuerier(t)

	manager := NewInstanceManager(time.Minute)
	defer manager.Stop()

	ctx := context.Background()
	config := map[string]any{"name": "warehouse"}
	req := pluginsdk.QueryRequest{Statement: "SELECT 1", Identity: "user:alice"}

	plan, err := manager.PlanQuery(ctx, "fakequerier", config, req)
	require.NoError(t, err)
	assert.Equal(t, []string{"mrn://table/fake/orders"}, plan.ReferencedMRNs)

	rows := drain(t, manager, config, req)
	require.Len(t, rows, 3)
	assert.Equal(t, "user:alice", rows[2][1])

	status := manager.Status()
	require.Len(t, status, 1)
	assert.Equal(t, 0, status[0].ActiveQueries, "closing the stream must release the instance")
	startedAt := status[0].StartedAt

	// A second query reuses the same process.
	drain(t, manager, config, req)
	assert.Equal(t, startedAt, manager.Status()[0].StartedAt)

	// A dead process is noticed and replaced on the next call.
	manager.mu.Lock()
	manager.instances["fakequerier"].process.Kill()
	manager.mu.Unlock()

	drain(t, manager, config, req)
	status = manager.Status()
	require.Len(t, status, 1)
	assert.Equal(t, 1, status[0].Restarts)
}

func TestInstanceManagerReapsIdle(t *testing.T) {
	registerFakeQuerier(t)

	manager := NewInstanceManager(time.Millisecond)
	defer manager.Stop()

	_, err := manager.PlanQuery(context.Background(), "fakequerier", map[string]any{"name": "w"}, pluginsdk.QueryRequest{Statement: "SELECT 1"})
	require.NoError(t, err)
	require.Len(t, manager.Status(), 1)

	time.Sleep(5 * time.Millisecond)
	manager.reapIdle()
	assert.Empty(t, manager.Status())
}

func TestInstanceManagerRejectsUnknownPlugin(t *testing.T) {
	manager := NewInstanceManager(time.Minute)
	defer manager.Stop()

	_, err := manager.PlanQuery(context.Background(), "does-not-exist", nil, pluginsdk.QueryRequest{})
	require.Error(t, err)
}

var (
	fakeQuerierOnce sync.Once
	fakeQuerierErr  error
)

// registerFakeQuerier builds the fake querier once per test binary and
// registers it. The registry is process-global, so the binary must outlive
// any single test and lives in its own temp dir rather than t.TempDir.
func registerFakeQuerier(t *testing.T) {
	t.Helper()
	fakeQuerierOnce.Do(func() {
		dir, err := os.MkdirTemp("", "marmot-fakequerier")
		if err != nil {
			fakeQuerierErr = err
			return
		}
		path := filepath.Join(dir, "fakequerier")
		cmd := exec.Command("go", "build", "-o", path, ".")
		cmd.Dir = "testdata/fakequerier"
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeQuerierErr = fmt.Errorf("building fake querier: %v\n%s", err, out)
			return
		}
		if _, err := GetRegistry().Get("fakequerier"); err == nil {
			return
		}
		_, fakeQuerierErr = registerExternalPlugin(path)
	})
	require.NoError(t, fakeQuerierErr)
}

func drain(t *testing.T, manager *InstanceManager, config map[string]any, req pluginsdk.QueryRequest) [][]any {
	t.Helper()

	stream, err := manager.ExecuteQuery(context.Background(), "fakequerier", config, req)
	require.NoError(t, err)
	defer stream.Close()

	var rows [][]any
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return rows
		}
		require.NoError(t, err)
		rows = append(rows, chunk.Rows...)
	}
}
