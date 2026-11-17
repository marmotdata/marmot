package runs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/plugin"
)

type getOnlyRuns struct{ runs.Service }

func (getOnlyRuns) GetRun(context.Context, string) (*plugin.Run, error) {
	return &plugin.Run{ID: "r1", Config: plugin.RawPluginConfig{"host": "db.internal", "user": "svc", "tags": []interface{}{"pg"}}}, nil
}

func TestGetRun_ShowsConnectionSettingsOnlyToIngestionManagers(t *testing.T) {
	cases := []struct {
		name  string
		perms []string
		want  map[string]interface{}
	}{
		{"viewer", []string{"ingestion:view"}, map[string]interface{}{"tags": []interface{}{"pg"}}},
		{"manager", []string{"ingestion:view", "ingestion:manage"}, map[string]interface{}{"host": "db.internal", "user": "svc", "tags": []interface{}{"pg"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := auth.NewServiceAccountPrincipal("sa-1", "reader", nil, tc.perms)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/r1", nil).
				WithContext(context.WithValue(context.Background(), common.PrincipalContextKey, p))
			req.SetPathValue("id", "r1")
			rec := httptest.NewRecorder()

			(&Handler{runService: getOnlyRuns{}}).getRun(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var got struct {
				Config map[string]interface{} `json:"config"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, tc.want, got.Config)
		})
	}
}
