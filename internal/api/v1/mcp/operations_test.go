package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/mcp"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`

// newTestHandler wires a handler with no backing services. The initialize
// handshake never calls a tool, so nil services are fine for these tests.
func newTestHandler() *Handler {
	return &Handler{
		mcpServer: mcp.NewServer(nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil),
		config:    &config.Config{},
	}
}

func postInitialize(t *testing.T, h *Handler, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", strings.NewReader(initializeRequest))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.handleMCP(rec, req)
	return rec
}

func TestHandleMCP_AcceptsServiceAccountPrincipal(t *testing.T) {
	p := auth.NewServiceAccountPrincipal("sa-1", "etl-copilot", []string{"agent"},
		[]string{"assets:view", "glossary:view", "teams:view"})
	ctx := context.WithValue(context.Background(), common.PrincipalContextKey, p)

	rec := postInitialize(t, newTestHandler(), ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "marmot-catalog")
}

func TestHandleMCP_AcceptsUserPrincipal(t *testing.T) {
	p := auth.NewUserPrincipal(&user.User{ID: "u-1", Username: "alice", Active: true})
	ctx := context.WithValue(context.Background(), common.PrincipalContextKey, p)

	rec := postInitialize(t, newTestHandler(), ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandleMCP_RejectsMissingPrincipal(t *testing.T) {
	rec := postInitialize(t, newTestHandler(), context.Background())

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
