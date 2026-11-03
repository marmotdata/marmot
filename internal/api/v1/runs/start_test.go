package runs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/plugin"
)

type startOnlyRuns struct {
	runs.Service
	started []string
}

func (s *startOnlyRuns) StartRun(_ context.Context, pipelineName, sourceName, _ string, _ plugin.RawPluginConfig) (*plugin.Run, error) {
	s.started = append(s.started, pipelineName+"/"+sourceName)
	return &plugin.Run{RunID: "run-1", PipelineName: pipelineName, SourceName: sourceName}, nil
}

// A CLI run brings its own plugin, so it starts while Marmot is still loading its own rather than being turned away with a 503.
func TestStartRun_DoesNotWaitForServerPlugins(t *testing.T) {
	if plugin.GetLoadState().Ready() {
		t.Skip("plugin load state already marked ready")
	}
	svc := &startOnlyRuns{}
	h := &Handler{runService: svc}
	p := auth.NewServiceAccountPrincipal("sa-1", "ingest", nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs/start",
		strings.NewReader(`{"pipeline_name":"local-duckdb","source_name":"duckdb"}`)).
		WithContext(context.WithValue(context.Background(), common.PrincipalContextKey, p))
	rec := httptest.NewRecorder()

	h.startRun(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"local-duckdb/duckdb"}, svc.started)
}
