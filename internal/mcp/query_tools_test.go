package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/pkg/config"
)

func TestCanQuery(t *testing.T) {
	ctx := context.Background()
	assert.False(t, (&ToolContext{queryService: stubQuery{}}).canQuery(ctx), "no principal")
	assert.False(t, (&ToolContext{principal: stubPrincipal{admin: true}}).canQuery(ctx), "no query service")
	assert.False(t, (&ToolContext{queryService: stubQuery{}, principal: stubPrincipal{permissions: map[string]bool{"assets:view": true}}}).canQuery(ctx))
	assert.True(t, (&ToolContext{queryService: stubQuery{}, principal: stubPrincipal{permissions: map[string]bool{"assets:query": true}}}).canQuery(ctx))
	assert.True(t, (&ToolContext{queryService: stubQuery{}, principal: stubPrincipal{admin: true}}).canQuery(ctx))
}

func TestQueryDataRefusesWithoutPermission(t *testing.T) {
	tc := &ToolContext{principal: stubPrincipal{}}
	res, _, err := tc.queryData(context.Background(), nil, QueryDataInput{Source: "warehouse", Statement: "SELECT 1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestQueryToolsOnlyOfferedToQueryingPrincipals(t *testing.T) {
	tools := func(s *Server, p stubPrincipal) []string {
		server := s.CreateMCPServer(context.Background(), p)
		client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
		ct, st := mcpsdk.NewInMemoryTransports()
		_, err := server.Connect(context.Background(), st, nil)
		require.NoError(t, err)
		session, err := client.Connect(context.Background(), ct, nil)
		require.NoError(t, err)
		defer session.Close()

		res, err := session.ListTools(context.Background(), nil)
		require.NoError(t, err)
		var names []string
		for _, tool := range res.Tools {
			names = append(names, tool.Name)
		}
		return names
	}

	var svc dataquery.Service = stubQuery{}
	withQuery := NewServer(nil, nil, nil, nil, nil, nil, nil, svc, &config.Config{}, nil)
	withoutQuery := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil)
	runner := stubPrincipal{permissions: map[string]bool{"assets:query": true}}

	assert.Contains(t, tools(withQuery, runner), "query_data")
	assert.NotContains(t, tools(withQuery, stubPrincipal{}), "query_data")
	assert.NotContains(t, tools(withoutQuery, runner), "query_data")
}

func TestRenderQueryResult(t *testing.T) {
	out := renderQueryResult(&dataquery.QueryResult{
		QueryID:   "q-1",
		Columns:   []dataquery.QueryColumn{{Name: "id"}, {Name: "note"}},
		Rows:      [][]any{{1, "a|b\nc"}, {2, nil}},
		RowCount:  2,
		Truncated: true,
		Context: []dataquery.AssetContext{
			{MRN: "mrn://table/postgresql/orders", Name: "orders", Type: "Table", Description: "One row per order", Tags: []string{"pii"}},
		},
	})

	assert.Contains(t, out, "2 row(s), truncated")
	assert.Contains(t, out, "| id | note |")
	assert.Contains(t, out, `| 1 | a\|b c |`, "pipes are escaped and newlines flattened")
	assert.Contains(t, out, "**orders** (Table): One row per order · tags: pii")
	assert.Contains(t, out, "q-1")
}

// stubQuery satisfies dataquery.Service for tool registration tests.
type stubQuery struct{ dataquery.Service }

// Admits mirrors the OSS authorizer, which lets in anyone holding assets:query.
func (stubQuery) Admits(_ context.Context, p auth.Principal) bool {
	return p != nil && (p.IsAdmin() || p.HasPermission("assets", "query"))
}
