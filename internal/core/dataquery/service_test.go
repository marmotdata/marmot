package dataquery

import (
	"context"
	"errors"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/notification"
	"github.com/marmotdata/marmot/internal/plugin"
)

const testPluginID = "query-test-querier"

type nopSource struct{}

func (nopSource) Validate(c plugin.RawPluginConfig) (plugin.RawPluginConfig, error) { return c, nil }
func (nopSource) Discover(context.Context, plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	return &plugin.DiscoveryResult{}, nil
}

func init() {
	_ = plugin.GetRegistry().Register(pluginsdk.Meta{ID: testPluginID, Name: "Query test", SupportsQuery: true}, nopSource{})
	_ = plugin.GetRegistry().Register(pluginsdk.Meta{ID: "query-test-noquery", Name: "No query"}, nopSource{})
}

type fakeSources struct {
	sources    map[string]*Source
	lastSearch string
	lastLimit  int
}

func (f *fakeSources) ListQueryableSources(_ context.Context, search string, limit, offset int) ([]*Source, int, error) {
	f.lastSearch, f.lastLimit = search, limit
	out := []*Source{}
	for _, s := range f.sources {
		out = append(out, s)
	}
	return out, len(out), nil
}

func (f *fakeSources) GetQueryableSource(_ context.Context, name string) (*Source, error) {
	s, ok := f.sources[name]
	if !ok {
		return nil, ErrSourceNotFound
	}
	return s, nil
}

type fakeRunner struct {
	plan      *pluginsdk.QueryPlan
	planErr   error
	chunks    []pluginsdk.QueryResultChunk
	execErr   error
	lastReq   pluginsdk.QueryRequest
	lastCfg   map[string]any
	executed  bool
	streamsOK bool
	// block, when set, makes execution wait until its context ends, standing in for a slow source.
	block chan struct{}
}

func (r *fakeRunner) PlanQuery(_ context.Context, _ string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	r.lastReq, r.lastCfg = req, config
	return r.plan, r.planErr
}

func (r *fakeRunner) ExecuteQuery(ctx context.Context, _ string, _ map[string]any, _ pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	r.executed = true
	if r.execErr != nil {
		return nil, r.execErr
	}
	if r.block != nil {
		return &blockingStream{ctx: ctx}, nil
	}
	return &fakeStream{runner: r, chunks: r.chunks}, nil
}

func (r *fakeRunner) Status() []plugin.InstanceStatus { return nil }

type blockingStream struct{ ctx context.Context }

func (s *blockingStream) Recv() (*pluginsdk.QueryResultChunk, error) {
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}
func (s *blockingStream) Close() error { return nil }

type fakeStream struct {
	runner *fakeRunner
	chunks []pluginsdk.QueryResultChunk
	i      int
}

func (s *fakeStream) Recv() (*pluginsdk.QueryResultChunk, error) {
	if s.i >= len(s.chunks) {
		return nil, io.EOF
	}
	c := s.chunks[s.i]
	s.i++
	return &c, nil
}

func (s *fakeStream) Close() error {
	s.runner.streamsOK = true
	return nil
}

type memRepo struct {
	mu      sync.Mutex
	queries map[string]*Query
	seq     int

	lastStats  StatsFilter
	withTables []string
	facts      map[string]TableFacts
	requests   []*AccessRequest
}

func newMemRepo() *memRepo {
	return &memRepo{queries: map[string]*Query{}}
}

func (m *memRepo) CreateQuery(_ context.Context, q *Query) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	q.ID = "query-" + strconv.Itoa(m.seq)
	q.StartedAt = time.Now()
	cp := *q
	m.queries[q.ID] = &cp
	return nil
}
func (m *memRepo) CompleteQuery(_ context.Context, id string, o QueryOutcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.queries[id]
	q.Status, q.RowsReturned, q.Truncated, q.Error, q.Context = o.Status, o.RowsReturned, o.Truncated, o.Error, o.Context
	if o.Reason != "" {
		q.OutcomeReason = o.Reason
	}
	return nil
}
func (m *memRepo) GetQuery(_ context.Context, id string) (*Query, error) {
	q, ok := m.queries[id]
	if !ok {
		return nil, ErrQueryNotFound
	}
	return q, nil
}
func (m *memRepo) ListQueries(context.Context, QueryFilter) ([]*Query, int, error) {
	return nil, 0, nil
}
func (m *memRepo) Stats(_ context.Context, f StatsFilter) (*Stats, error) {
	m.lastStats = f
	return &Stats{From: f.From, To: f.To, Interval: f.Interval}, nil
}
func (m *memRepo) ListSourceTables(context.Context, SourceTablesFilter) ([]SourceTable, map[string]map[string]any, error) {
	return nil, nil, nil
}
func (m *memRepo) SourceActivity(context.Context, []*Source) error                  { return nil }
func (m *memRepo) EachQuery(context.Context, QueryFilter, func(*Query) error) error { return nil }
func (m *memRepo) TableFacts(_ context.Context, ids []string) (map[string]TableFacts, error) {
	return m.facts, nil
}
func (m *memRepo) CreateAccessRequest(_ context.Context, req *AccessRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req.ID = "request-" + strconv.Itoa(len(m.requests)+1)
	req.CreatedAt = time.Now()
	cp := *req
	m.requests = append(m.requests, &cp)
	return nil
}
func (m *memRepo) RecentAccessRequest(_ context.Context, principalType, principalID, sourceName, needed string, since time.Time) (*AccessRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.requests) - 1; i >= 0; i-- {
		r := m.requests[i]
		if r.PrincipalType == principalType && r.PrincipalID == principalID && r.SourceName == sourceName && r.Needed == needed && !r.CreatedAt.Before(since) {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}
func (m *memRepo) SourcesWithTables(_ context.Context, ids []string, _ func(string, int) (string, []any)) ([]string, error) {
	return m.withTables, nil
}

func (m *memRepo) only(t *testing.T) *Query {
	t.Helper()
	require.Len(t, m.queries, 1)
	for _, q := range m.queries {
		return q
	}
	return nil
}

// fakeAssets embeds the interface so only GetByMRNs needs implementing.
type fakeAssets struct {
	asset.Service
	byMRN map[string]*asset.Asset
}

func (f *fakeAssets) GetByMRNs(_ context.Context, mrns []string) (map[string]*asset.Asset, error) {
	out := map[string]*asset.Asset{}
	for _, m := range mrns {
		if a, ok := f.byMRN[m]; ok {
			out[m] = a
		}
	}
	return out, nil
}

type countTrigger struct {
	mu    sync.Mutex
	calls []string
}

func (c *countTrigger) TriggerIngest(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, id)
	return nil
}

func (c *countTrigger) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

const ordersMRN = "mrn://table/postgresql/shop/orders"

func strp(s string) *string { return &s }

type fixture struct {
	svc     Service
	repo    *memRepo
	runner  *fakeRunner
	trigger *countTrigger
	sources *fakeSources
	assets  *fakeAssets
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := newMemRepo()
	runner := &fakeRunner{
		plan: &pluginsdk.QueryPlan{ReferencedMRNs: []string{ordersMRN}},
		chunks: []pluginsdk.QueryResultChunk{
			{Columns: []pluginsdk.QueryColumn{{Name: "id", Type: "integer"}}, Rows: [][]any{{1}, {2}}},
			{Rows: [][]any{{3}}},
		},
	}
	sources := &fakeSources{sources: map[string]*Source{
		"warehouse": {ID: "src-1", Name: "warehouse", PluginID: testPluginID, IngestOnQuery: true, Config: map[string]any{"host": "db"}},
		"noquery":   {ID: "src-2", Name: "noquery", PluginID: "query-test-noquery"},
	}}
	assets := &fakeAssets{byMRN: map[string]*asset.Asset{
		ordersMRN: {ID: "asset-1", Name: strp("orders"), Type: "Table", Providers: []string{"PostgreSQL"},
			Description: strp("from the plugin"), UserDescription: strp("One row per order"), Tags: []string{"pii"}},
	}}
	trigger := &countTrigger{}
	svc := NewService(repo, sources, assets, runner, trigger, nil, Options{
		QueryTimeout:         time.Minute,
		MaxRowsDefault:       100,
		MaxRowsCap:           1000,
		IngestSourceInterval: time.Hour,
		IngestGlobalInterval: 0,
	})
	return &fixture{svc: svc, repo: repo, runner: runner, trigger: trigger, sources: sources, assets: assets}
}

func TestQueryDeniedWithoutQueryPermission(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "warehouse", Statement: "SELECT * FROM orders", Agent: "claude-code"}, ClientMCP)
	require.ErrorIs(t, err, ErrDenied)
	assert.False(t, f.runner.executed, "a denied query must never execute")

	var denied *DeniedError
	require.ErrorAs(t, err, &denied)
	assert.Equal(t, ReasonPermission, denied.Refusal.Rule)
	assert.Equal(t, "assets:query", denied.Refusal.Permission)
	assert.Equal(t, "assets:query on warehouse", denied.Refusal.Needed)
	assert.Equal(t, []string{}, denied.Refusal.Roles)
	assert.NotEmpty(t, denied.Refusal.QueryID)
	assert.Contains(t, err.Error(), "query denied by policy: querying data needs the assets:query permission", "the message callers have always read is kept")

	q := f.repo.only(t)
	assert.Equal(t, DecisionDenied, q.Decision)
	assert.Equal(t, StatusDenied, q.Status)
	assert.Equal(t, ReasonPermission, q.OutcomeReason)
	assert.Equal(t, ClientMCP, q.Client)
	assert.Empty(t, q.ReferencedMRNs, "a caller who may not query is refused before the statement is planned")
	assert.Contains(t, q.DecisionDetail["reason"], "assets:query")
	assert.Equal(t, "assets:query on warehouse", q.DecisionDetail["needed"])
	require.NotNil(t, q.ServiceAccount)
	assert.Equal(t, "test-agent", *q.ServiceAccount)
	require.NotNil(t, q.Agent)
	assert.Equal(t, "claude-code", *q.Agent)
}

func TestQueryDeniedBeforeSourceLookup(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "no-such-source", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrDenied, "an unknown source reads as a refusal, so its existence is not confirmed")
	assert.Nil(t, f.repo.only(t).SourceID)
}

func TestQueryContextCarriesTableFacts(t *testing.T) {
	f := newFixture(t)
	a := f.assets.byMRN[ordersMRN]
	a.Tags = []string{"certified"}
	a.Metadata = map[string]any{"row_count": float64(1284019)}
	a.Schema = map[string]string{"columns": `[{"column_name":"id"},{"column_name":"email","tags":["PII.Sensitive"]},{"column_name":"total"}]`}
	f.repo.facts = map[string]TableFacts{"asset-1": {Owners: []string{"Analytics Engineering"}, PersonalData: []string{"phone"}}}

	res, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT id FROM orders"}, ClientAPI)
	require.NoError(t, err)
	require.Len(t, res.Context, 1)
	c := res.Context[0]
	assert.Equal(t, []string{"Analytics Engineering"}, c.Owners)
	require.NotNil(t, c.RowCount)
	assert.Equal(t, int64(1284019), *c.RowCount)
	require.NotNil(t, c.ColumnsRead)
	assert.Equal(t, 1, *c.ColumnsRead, "only id came back")
	assert.True(t, c.Certified)
	assert.Equal(t, []PersonalDataColumn{{Name: "email"}, {Name: "phone"}}, c.PersonalData)
	assert.Equal(t, ReasonRole, f.repo.only(t).OutcomeReason)
}

func TestQueryCancel(t *testing.T) {
	f := newFixture(t)
	f.runner.block = make(chan struct{})

	done := make(chan error, 1)
	go func() {
		_, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT pg_sleep(60)"}, ClientEditor)
		done <- err
	}()

	var id string
	require.Eventually(t, func() bool {
		f.repo.mu.Lock()
		defer f.repo.mu.Unlock()
		for qid, q := range f.repo.queries {
			if q.Status == StatusRunning {
				id = qid
				return true
			}
		}
		return false
	}, time.Second, 5*time.Millisecond)
	require.Eventually(t, func() bool { return f.svc.Cancel(context.Background(), id) == nil }, time.Second, 5*time.Millisecond)

	err := <-done
	require.ErrorIs(t, err, ErrCancelled)
	q := f.repo.only(t)
	assert.Equal(t, StatusCancelled, q.Status)
	assert.Equal(t, ReasonCancelled, q.OutcomeReason)
	assert.Equal(t, ClientEditor, q.Client)
	assert.ErrorIs(t, f.svc.Cancel(context.Background(), id), ErrNotRunning, "a finished query cannot be cancelled")
}

// otherPrincipal is a different service account from fakePrincipal.
type otherPrincipal struct{ fakePrincipal }

func (otherPrincipal) ID() string { return "00000000-0000-0000-0000-000000000043" }

type recordingNotifier struct {
	inputs []notification.CreateNotificationInput
}

func (r *recordingNotifier) CreateSync(_ context.Context, input notification.CreateNotificationInput) (int, error) {
	r.inputs = append(r.inputs, input)
	return len(input.Recipients), nil
}

func TestRequestAccessNotifiesGrantorsOnce(t *testing.T) {
	f := newFixture(t)
	notifier := &recordingNotifier{}
	f.svc.(*service).opts.Notifier = notifier
	f.svc.(*service).opts.Grantors = func(_ context.Context, principalType string) ([]notification.Recipient, error) {
		assert.Equal(t, "service_account", principalType)
		return []notification.Recipient{{Type: notification.RecipientTypeUser, ID: "admin-1"}}, nil
	}

	_, err := f.svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "warehouse", Statement: "SELECT 1"}, ClientAPI)
	var denied *DeniedError
	require.ErrorAs(t, err, &denied)

	req, err := f.svc.RequestAccess(context.Background(), fakePrincipal{}, AccessRequestInput{QueryID: denied.Refusal.QueryID, Message: "for the churn report"})
	require.NoError(t, err)
	assert.Equal(t, "assets:query on warehouse", req.Needed)
	assert.Equal(t, 1, req.Notified)
	assert.False(t, req.Repeat)
	require.Len(t, notifier.inputs, 1)
	assert.Equal(t, notification.TypeAccessRequest, notifier.inputs[0].Type)
	assert.Equal(t, "/query-audit?id="+denied.Refusal.QueryID, notifier.inputs[0].Data["link"])

	again, err := f.svc.RequestAccess(context.Background(), fakePrincipal{}, AccessRequestInput{Source: "warehouse"})
	require.NoError(t, err)
	assert.True(t, again.Repeat, "the same access asked for again within a day is not sent twice")
	assert.Len(t, notifier.inputs, 1)

	_, err = f.svc.RequestAccess(context.Background(), otherPrincipal{}, AccessRequestInput{QueryID: denied.Refusal.QueryID})
	assert.ErrorIs(t, err, ErrQueryNotFound, "someone else's query cannot be the basis of a request")
}

func TestQueryAllowedReturnsRowsAndContext(t *testing.T) {
	f := newFixture(t)
	p := fakePrincipal{canQuery: true}

	res, err := f.svc.Query(context.Background(), p, QueryInput{Source: "warehouse", Statement: "SELECT id FROM orders"}, ClientAPI)
	require.NoError(t, err)

	assert.Equal(t, int64(3), res.RowCount)
	assert.False(t, res.Truncated)
	assert.Equal(t, []QueryColumn{{Name: "id", Type: "integer"}}, res.Columns)
	require.Len(t, res.Context, 1)
	assert.Equal(t, "One row per order", res.Context[0].Description, "the user description wins over the plugin's")
	assert.Equal(t, "asset-1", res.Context[0].AssetID)
	assert.True(t, f.runner.streamsOK, "the stream must be closed")

	assert.Equal(t, "service_account:test-agent", f.runner.lastReq.Identity, "the plugin learns who is asking")
	assert.Equal(t, int64(101), f.runner.lastReq.MaxRows, "one row past the limit detects truncation")

	q := f.repo.only(t)
	assert.Equal(t, res.QueryID, q.ID)
	assert.Equal(t, StatusCompleted, q.Status)
	assert.Equal(t, int64(3), *q.RowsReturned)
	require.Len(t, q.Context, 1, "the snapshot is stored with the query")

	require.Eventually(t, func() bool { return f.trigger.count() == 1 }, time.Second, 5*time.Millisecond)
}

func TestQueryTruncatesAtMaxRows(t *testing.T) {
	f := newFixture(t)

	res, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT id FROM orders", MaxRows: 2}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, int64(2), res.RowCount)
	assert.True(t, res.Truncated)
	assert.True(t, f.repo.only(t).Truncated)
}

func TestQueryExactlyAtLimitIsNotTruncated(t *testing.T) {
	f := newFixture(t)

	res, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT id FROM orders", MaxRows: 3}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, int64(3), res.RowCount)
	assert.False(t, res.Truncated, "three rows with a limit of three is complete")
}

func TestQueryMaxRowsIsCapped(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT 1", MaxRows: 1_000_000}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, int64(1001), f.runner.lastReq.MaxRows)
}

func TestQueryPlanUnimplementedFallsBackToSource(t *testing.T) {
	f := newFixture(t)
	f.runner.plan = nil
	f.runner.planErr = status.Error(codes.Unimplemented, "no planner")

	res, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT 1"}, ClientAPI)
	require.NoError(t, err)
	assert.Equal(t, []string{"mrn://source/warehouse"}, res.ReferencedMRNs)
	assert.Empty(t, res.Context)
}

func TestQueryExecutionFailureIsRecorded(t *testing.T) {
	f := newFixture(t)
	f.runner.execErr = errors.New("relation does not exist")

	_, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrQueryFailed)

	q := f.repo.only(t)
	assert.Equal(t, StatusFailed, q.Status)
	require.NotNil(t, q.Error)
	assert.Contains(t, *q.Error, "relation does not exist")
	assert.Zero(t, f.trigger.count(), "failed queries do not refresh the catalog")
}

func TestQueryPlanErrorIsReadable(t *testing.T) {
	f := newFixture(t)
	f.runner.planErr = status.Error(codes.Unknown, `relation "nope" does not exist`)

	_, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse", Statement: "SELECT * FROM nope"}, ClientAPI)
	require.ErrorIs(t, err, ErrQueryFailed)
	assert.Equal(t, `query failed: relation "nope" does not exist`, err.Error())
}

func TestQueryRejectsUnknownAndUnsupportedSources(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "nope", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrSourceNotFound)

	_, err = f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "noquery", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrQueryUnsupported)

	_, err = f.svc.Query(context.Background(), fakePrincipal{admin: true}, QueryInput{Source: "warehouse"}, ClientAPI)
	require.ErrorIs(t, err, ErrInvalidInput)

	assert.Empty(t, f.repo.queries, "requests rejected before planning are not recorded")
}

func TestQueryUsesTheConfiguredAuthorizer(t *testing.T) {
	f := newFixture(t)
	called := false
	svc := NewService(f.repo, &fakeSources{sources: map[string]*Source{
		"warehouse": {ID: "src-1", Name: "warehouse", PluginID: testPluginID},
	}}, nil, f.runner, nil, nil, Options{
		QueryTimeout: time.Minute, MaxRowsDefault: 10, MaxRowsCap: 10,
		Authorizer: authorizerFunc(func(context.Context, auth.Principal, *Source, []string) (Decision, error) {
			called = true
			return Decision{Allowed: false, Reason: "the IAM says no"}, nil
		}),
	})

	_, err := svc.Query(context.Background(), fakePrincipal{}, QueryInput{Source: "warehouse", Statement: "SELECT 1"}, ClientAPI)
	require.ErrorIs(t, err, ErrDenied)
	assert.True(t, called)
}

type authorizerFunc func(context.Context, auth.Principal, *Source, []string) (Decision, error)

func (f authorizerFunc) Admits(context.Context, auth.Principal) bool { return true }

func (f authorizerFunc) Authorize(ctx context.Context, p auth.Principal, s *Source, r []string) (Decision, error) {
	return f(ctx, p, s, r)
}

func (f authorizerFunc) Visible(context.Context, auth.Principal) (Visibility, error) {
	return Visibility{All: true}, nil
}

// visibilityAuthorizer hands every principal the same fixed visibility.
type visibilityAuthorizer struct{ v Visibility }

func (a visibilityAuthorizer) Admits(context.Context, auth.Principal) bool { return true }

func (a visibilityAuthorizer) Authorize(context.Context, auth.Principal, *Source, []string) (Decision, error) {
	return Decision{Allowed: true}, nil
}

func (a visibilityAuthorizer) Visible(context.Context, auth.Principal) (Visibility, error) {
	return a.v, nil
}

func restrictedFixture(t *testing.T, v Visibility) *fixture {
	t.Helper()
	f := newFixture(t)
	f.sources.sources["finance"] = &Source{ID: "src-3", Name: "finance", PluginID: testPluginID}
	f.svc = NewService(f.repo, f.sources, nil, f.runner, nil, nil, Options{QueryTimeout: time.Minute, Authorizer: visibilityAuthorizer{v}})
	return f
}

func TestListSourcesNarrowsToVisibleSources(t *testing.T) {
	assets := func(string, int) (string, []any) { return "TRUE", nil }
	f := restrictedFixture(t, Visibility{SourceIDs: []string{"src-1"}, Assets: assets})
	f.repo.withTables = []string{"src-3"}

	sources, total, err := f.svc.ListSources(context.Background(), SourceFilter{Principal: fakePrincipal{}})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	var names []string
	for _, s := range sources {
		names = append(names, s.Name)
	}
	assert.ElementsMatch(t, []string{"warehouse", "finance"}, names, "a whole-source grant and a source with one visible table are listed; the rest are not")
}

func TestListSourcesWithNoVisibilityIsEmpty(t *testing.T) {
	f := restrictedFixture(t, Visibility{})

	sources, total, err := f.svc.ListSources(context.Background(), SourceFilter{Principal: fakePrincipal{}})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, sources)
}

func TestListSourceTablesHidesUnreachableSource(t *testing.T) {
	assets := func(string, int) (string, []any) { return "TRUE", nil }
	f := restrictedFixture(t, Visibility{Assets: assets})

	_, err := f.svc.ListSourceTables(context.Background(), fakePrincipal{}, "finance", "", 0)
	assert.ErrorIs(t, err, ErrSourceNotFound, "a source with no table the principal may query is reported as missing")

	f.repo.withTables = []string{"src-3"}
	_, err = f.svc.ListSourceTables(context.Background(), fakePrincipal{}, "finance", "", 0)
	assert.NoError(t, err)
}

func TestListSourcesDefaultsAndTrims(t *testing.T) {
	f := newFixture(t)

	_, total, err := f.svc.ListSources(context.Background(), SourceFilter{Search: "  ware  ", Limit: 5000})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, "ware", f.sources.lastSearch)
	assert.Equal(t, 50, f.sources.lastLimit, "an oversized page falls back to the default")
}
