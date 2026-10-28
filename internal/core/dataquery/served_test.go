package dataquery

import (
	"context"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeServed struct {
	planSource, execSource, queryID string
	runner                          *fakeRunner
}

func (f *fakeServed) PlanQuery(ctx context.Context, source string, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	f.planSource = source
	return f.runner.PlanQuery(ctx, "", nil, req)
}

func (f *fakeServed) ExecuteQuery(ctx context.Context, source, queryID string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	f.execSource, f.queryID = source, queryID
	return f.runner.ExecuteQuery(ctx, "", nil, req)
}

func servedFixture(t *testing.T, served ServedRunner) (Service, *fakeRunner, *memRepo) {
	t.Helper()
	repo := newMemRepo()
	local := &fakeRunner{}
	sources := &fakeSources{sources: map[string]*Source{
		// A served source's stored config is masked and its plugin need not be installed on the server.
		"remote": {ID: "src-r", Name: "remote", PluginID: "not-installed-here", Served: true, Config: map[string]any{"password": "********"}},
	}}
	svc := NewService(repo, sources, &fakeAssets{}, local, &countTrigger{}, nil, Options{
		QueryTimeout:   time.Minute,
		MaxRowsDefault: 100,
		MaxRowsCap:     1000,
		Served:         served,
	})
	return svc, local, repo
}

func TestServedSourceRoutesToServingPlugin(t *testing.T) {
	remote := &fakeRunner{
		plan:   &pluginsdk.QueryPlan{ReferencedMRNs: []string{ordersMRN}},
		chunks: []pluginsdk.QueryResultChunk{{Columns: []pluginsdk.QueryColumn{{Name: "id"}}, Rows: [][]any{{1}}}},
	}
	served := &fakeServed{runner: remote}
	svc, local, repo := servedFixture(t, served)

	res, err := svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "remote", Statement: "SELECT id FROM orders"}, ClientAPI)
	require.NoError(t, err)

	assert.Equal(t, int64(1), res.RowCount)
	assert.Equal(t, "remote", served.planSource)
	assert.Equal(t, "remote", served.execSource)
	assert.Equal(t, repo.only(t).ID, served.queryID, "the serving process is told the query's log id")
	assert.False(t, local.executed, "Marmot never runs a served source's plugin itself")
	assert.Nil(t, local.lastCfg, "Marmot never hands a served source's stored config to a plugin")
}

func TestServedSourceDeniedStillNeverExecutes(t *testing.T) {
	remote := &fakeRunner{plan: &pluginsdk.QueryPlan{ReferencedMRNs: []string{ordersMRN}}}
	svc, _, _ := servedFixture(t, &fakeServed{runner: remote})

	_, err := svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "remote", Statement: "SELECT * FROM orders"}, ClientAPI)
	require.ErrorIs(t, err, ErrDenied)
	assert.False(t, remote.executed, "authorisation applies to served sources exactly as to local ones")
}

func TestServedSourceWithServingDisabled(t *testing.T) {
	svc, local, _ := servedFixture(t, nil)

	_, err := svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "remote", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrQueryUnsupported)
	assert.False(t, local.executed)
}

// planAuthorizer runs the fixture's runner through a service whose authorizer records the resources it is asked about.
func planAuthorizer(t *testing.T, plan *pluginsdk.QueryPlan) (Service, *memRepo, *[]string) {
	t.Helper()
	f := newFixture(t)
	f.runner.plan = plan
	var asked []string
	svc := NewService(f.repo, f.sources, nil, f.runner, nil, nil, Options{
		QueryTimeout: time.Minute, MaxRowsDefault: 10, MaxRowsCap: 10,
		Authorizer: authorizerFunc(func(_ context.Context, _ auth.Principal, _ *Source, resources []string) (Decision, error) {
			asked = resources
			return Decision{Allowed: true}, nil
		}),
	})
	return svc, f.repo, &asked
}

// A statement that may read tables its plan cannot name, such as through query_to_xml, is authorised against the whole source as well as the tables it does name.
func TestIncompletePlanAsksAboutTheWholeSource(t *testing.T) {
	svc, repo, asked := planAuthorizer(t, &pluginsdk.QueryPlan{ReferencedMRNs: []string{ordersMRN}, Incomplete: true})

	statement := "SELECT query_to_xml('select * from payroll', true, false, '') FROM orders"
	_, err := svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "warehouse", Statement: statement}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{ordersMRN, SourceMRN("warehouse")}, *asked)
	assert.Equal(t, []string{ordersMRN, SourceMRN("warehouse")}, repo.only(t).ReferencedMRNs, "the log shows the tables the plan named and the source")
}

func TestCompletePlanAsksOnlyAboutItsTables(t *testing.T) {
	svc, _, asked := planAuthorizer(t, &pluginsdk.QueryPlan{ReferencedMRNs: []string{ordersMRN}})

	_, err := svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "warehouse", Statement: "SELECT lower(name) FROM orders"}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{ordersMRN}, *asked)
}
