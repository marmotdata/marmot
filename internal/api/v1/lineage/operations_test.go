package lineage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingService captures the column lineage handed to CreateDeclaredLineage.
type recordingService struct {
	lineage.Service
	columnLineage [][]lineage.ColumnEdge
}

func (s *recordingService) CreateDeclaredLineage(_ context.Context, _, _, _, _, _ string, columnLineage []lineage.ColumnEdge) (string, error) {
	s.columnLineage = append(s.columnLineage, columnLineage)
	return "edge-1", nil
}

func (s *recordingService) EdgeExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestCreateDirectLineageStoresColumnLineage(t *testing.T) {
	columns := []lineage.ColumnEdge{{FromColumns: []string{"id"}, ToColumn: "customer_id", Transform: "identity"}}
	edge := lineage.LineageEdge{Source: "postgres://db/public/a", Target: "postgres://db/public/b", ColumnLineage: columns}

	tests := []struct {
		name    string
		handler func(h *Handler) http.HandlerFunc
		body    any
	}{
		{name: "direct", handler: func(h *Handler) http.HandlerFunc { return h.createDirectLineage }, body: edge},
		{name: "batch", handler: func(h *Handler) http.HandlerFunc { return h.batchCreateLineage }, body: []lineage.LineageEdge{edge}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &recordingService{}
			h := &Handler{lineageService: svc}

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			tt.handler(h)(rec, httptest.NewRequest(http.MethodPost, "/api/v1/lineage/"+tt.name, bytes.NewReader(body)))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Len(t, svc.columnLineage, 1)
			assert.Equal(t, columns, svc.columnLineage[0])
		})
	}
}
