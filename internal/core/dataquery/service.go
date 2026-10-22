// Package dataquery lets agents and users query catalogued data sources through
// Marmot. Every query runs as the requesting principal, is checked by the
// configured authorizer, is recorded, and comes back with the catalog's
// view of the data it touched.
package dataquery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
)

var (
	ErrDenied           = errors.New("query denied by policy")
	ErrQueryUnsupported = errors.New("plugin does not support queries")
	// ErrQueryFailed wraps errors the source itself reported while planning
	// or running a statement, such as a syntax error or a missing table.
	ErrQueryFailed = errors.New("query failed")
)

// Clients that run queries, recorded on each query.
const (
	ClientAPI = "api"
	ClientMCP = "mcp"
	ClientCLI = "cli"
)

// QueryRunner dispatches plan and execute calls to long-running plugin
// instances. Implemented by plugin.InstanceManager.
type QueryRunner interface {
	PlanQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error)
	ExecuteQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error)
	Status() []plugin.InstanceStatus
}

// ServedRunner sends queries to plugins that run outside Marmot and serve their own sources. Implemented by pluginserve.Hub.
type ServedRunner interface {
	PlanQuery(ctx context.Context, source string, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error)
	ExecuteQuery(ctx context.Context, source, queryID string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error)
}

// Source is an ingestion source that answers data queries. Config is the
// stored config, with sensitive fields still encrypted, and never leaves the
// service.
type Source struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PluginID      string `json:"plugin_id"`
	IngestOnQuery bool   `json:"ingest_on_query"`
	// Served is set when a plugin running outside Marmot answers this source's queries; Marmot then holds no usable config for it.
	Served bool           `json:"served"`
	Config map[string]any `json:"-"`

	// Activity, filled in when sources are listed.
	TableCount      int        `json:"table_count"`
	QueriesLastWeek int64      `json:"queries_last_week"`
	LastQueriedAt   *time.Time `json:"last_queried_at,omitempty"`
} // @name QuerySource

// SourceProvider resolves queryable sources from ingestion sources. It is
// implemented at the wiring layer over the schedule service so the query domain stays
// decoupled from the ingestion domain.
type SourceProvider interface {
	ListQueryableSources(ctx context.Context, search string, limit, offset int) ([]*Source, int, error)
	GetQueryableSource(ctx context.Context, name string) (*Source, error)
}

// IngestTrigger enqueues an asynchronous catalog refresh for a source. A
// nil trigger disables ingest-on-query.
type IngestTrigger interface {
	TriggerIngest(ctx context.Context, sourceID string) error
}

// Options carries the data query settings from the config file.
type Options struct {
	QueryTimeout   time.Duration
	MaxRowsDefault int64
	MaxRowsCap     int64
	// IngestSourceInterval is the minimum gap between query-triggered
	// refreshes of one source; IngestGlobalInterval is the minimum gap
	// between any two, so query traffic cannot turn into ingestion load.
	IngestSourceInterval time.Duration
	IngestGlobalInterval time.Duration
	// Lineage, when set, catalogs the service accounts that query as Agent assets with observed lineage from the tables they read; ConsumerInterval is the minimum gap between writes of one edge.
	Lineage          ObservedLineage
	ConsumerInterval time.Duration
	// Served routes queries for sources served from outside Marmot. When nil, such sources cannot be queried.
	Served ServedRunner
	// Authorizer decides which tables a caller may query. When nil, anyone
	// holding assets:query may query every queryable source; the cloud
	// distribution supplies one backed by its IAM.
	Authorizer Authorizer
}

// QueryInput is one statement to run against a queryable source.
type QueryInput struct {
	Source    string `json:"source" validate:"required"`
	Statement string `json:"statement" validate:"required"`
	MaxRows   int64  `json:"max_rows,omitempty"`
} // @name QueryRunInput

// QueryColumn describes one result column.
type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
} // @name QueryColumn

// QueryResult carries rows back together with the query's id in the query
// log and the catalog context of everything it touched.
type QueryResult struct {
	QueryID        string         `json:"query_id"`
	Columns        []QueryColumn  `json:"columns"`
	Rows           [][]any        `json:"rows"`
	RowCount       int64          `json:"row_count"`
	Truncated      bool           `json:"truncated"`
	ReferencedMRNs []string       `json:"referenced_mrns"`
	Context        []AssetContext `json:"context,omitempty"`
} // @name QueryResult

// Service is data querying: queryable sources, policy-checked query execution
// and the query log.
type Service interface {
	ListSources(ctx context.Context, filter SourceFilter) ([]*Source, int, error)
	ListSourceTables(ctx context.Context, sourceName, search string, limit int) ([]SourceTable, error)
	InstanceStatus() []plugin.InstanceStatus

	Query(ctx context.Context, principal auth.Principal, input QueryInput, client string) (*QueryResult, error)
	GetQuery(ctx context.Context, id string) (*Query, error)
	ListQueries(ctx context.Context, filter QueryFilter) ([]*Query, int, error)

	Stats(ctx context.Context, filter StatsFilter) (*Stats, error)
}

type service struct {
	repo       Repository
	sources    SourceProvider
	assets     asset.Service
	runner     QueryRunner
	ingest     *ingestThrottle
	consumers  *consumerRecorder
	encryptor  *crypto.Encryptor
	opts       Options
	validator  *validator.Validate
	authorizer Authorizer
}

func NewService(repo Repository, sources SourceProvider, assets asset.Service, runner QueryRunner, ingest IngestTrigger, encryptor *crypto.Encryptor, opts Options) Service {
	authorizer := opts.Authorizer
	if authorizer == nil {
		authorizer = permissionAuthorizer{}
	}
	return &service{
		authorizer: authorizer,
		repo:       repo,
		sources:    sources,
		assets:     assets,
		runner:     runner,
		ingest:     newIngestThrottle(ingest, opts.IngestSourceInterval, opts.IngestGlobalInterval),
		consumers:  newConsumerRecorder(assets, opts.Lineage, opts.ConsumerInterval),
		encryptor:  encryptor,
		opts:       opts,
		validator:  validator.New(),
	}
}

// SourceFilter pages and searches queryable sources. Search matches the
// source name or plugin.
type SourceFilter struct {
	Search string
	Limit  int
	Offset int
}

func (s *service) ListSources(ctx context.Context, filter SourceFilter) ([]*Source, int, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	sources, total, err := s.sources.ListQueryableSources(ctx, strings.TrimSpace(filter.Search), filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	if err := s.repo.SourceActivity(ctx, sources); err != nil {
		log.Warn().Err(err).Msg("Failed to load query source activity")
	}
	return sources, total, nil
}

func (s *service) InstanceStatus() []plugin.InstanceStatus {
	return s.runner.Status()
}

func (s *service) GetQuery(ctx context.Context, id string) (*Query, error) {
	return s.repo.GetQuery(ctx, id)
}

func (s *service) ListQueries(ctx context.Context, filter QueryFilter) ([]*Query, int, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.ListQueries(ctx, filter)
}

// maxStatsSpan bounds a stats window so a dashboard cannot ask for years of
// hourly points.
const maxStatsSpan = 90 * 24 * time.Hour

// Stats summarises activity over a window. The window defaults to the last
// day, hourly; spans over three days default to daily points.
func (s *service) Stats(ctx context.Context, filter StatsFilter) (*Stats, error) {
	now := time.Now().UTC()
	if filter.To.IsZero() || filter.To.After(now.Add(time.Hour)) {
		filter.To = now.Truncate(time.Hour).Add(time.Hour)
	}
	if filter.From.IsZero() {
		filter.From = filter.To.Add(-24 * time.Hour)
	}
	if !filter.From.Before(filter.To) {
		return nil, fmt.Errorf("%w: from must be before to", ErrInvalidInput)
	}
	if filter.To.Sub(filter.From) > maxStatsSpan {
		return nil, fmt.Errorf("%w: the window can span at most 90 days", ErrInvalidInput)
	}
	switch filter.Interval {
	case "":
		filter.Interval = IntervalHour
		if filter.To.Sub(filter.From) > 72*time.Hour {
			filter.Interval = IntervalDay
		}
	case IntervalHour, IntervalDay:
	default:
		return nil, fmt.Errorf("%w: interval must be hour or day", ErrInvalidInput)
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 6
	}
	return s.repo.Stats(ctx, filter)
}

// Query plans the statement through the source's plugin, checks the plan
// with the authorizer, records the attempt, executes, and attaches
// the catalog context of every referenced asset. Denied queries are
// recorded too.
func (s *service) Query(ctx context.Context, principal auth.Principal, input QueryInput, client string) (*QueryResult, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	source, err := s.sources.GetQueryableSource(ctx, input.Source)
	if err != nil {
		return nil, err
	}

	runner, err := s.runnerFor(source)
	if err != nil {
		return nil, err
	}

	maxRows := s.opts.MaxRowsDefault
	if input.MaxRows > 0 {
		maxRows = min(input.MaxRows, s.opts.MaxRowsCap)
	}
	// One row past the limit is requested so a full result can be told apart
	// from a truncated one; execute never returns it.
	req := pluginsdk.QueryRequest{
		Statement: input.Statement,
		MaxRows:   maxRows + 1,
		Identity:  principal.AuditSubject(),
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

	resources, err := s.planResources(queryCtx, source, runner, req)
	if err != nil {
		return nil, err
	}

	decision, err := s.authorizer.Authorize(ctx, principal, source, resources)
	if err != nil {
		return nil, fmt.Errorf("checking access: %w", err)
	}

	record := &Query{
		PrincipalType:  string(principal.Type()),
		PrincipalID:    principal.ID(),
		AuditSubject:   principal.AuditSubject(),
		Client:         client,
		SourceID:       &source.ID,
		SourceName:     source.Name,
		Statement:      input.Statement,
		ReferencedMRNs: resources,
		Decision:       DecisionAllowed,
		DecisionDetail: decisionDetail(decision),
		Status:         StatusRunning,
	}
	if !decision.Allowed {
		record.Decision = DecisionDenied
		record.Status = StatusDenied
	}
	if err := s.repo.CreateQuery(ctx, record); err != nil {
		return nil, err
	}

	if !decision.Allowed {
		s.complete(ctx, record.ID, QueryOutcome{Status: StatusDenied})
		return nil, fmt.Errorf("%w: %s", ErrDenied, decision.Reason)
	}

	result, execErr := s.execute(queryCtx, runner, record.ID, req, maxRows)
	if execErr != nil {
		msg := pluginMessage(execErr)
		s.complete(ctx, record.ID, QueryOutcome{Status: StatusFailed, Error: &msg})
		return nil, fmt.Errorf("%w: %s", ErrQueryFailed, msg)
	}

	assetContext := s.assetContext(ctx, resources)
	s.complete(ctx, record.ID, QueryOutcome{
		Status:       StatusCompleted,
		RowsReturned: &result.RowCount,
		Truncated:    result.Truncated,
		Context:      assetContext,
	})

	// Refresh the source's catalog off the back of real usage, so a
	// query-only source populates itself and busy sources stay current.
	// Async and rate-limited, so it never adds to query latency.
	if source.IngestOnQuery {
		s.ingest.maybeTrigger(source.ID)
	}
	s.consumers.record(principal, resources)

	result.QueryID = record.ID
	result.ReferencedMRNs = resources
	result.Context = assetContext
	return result, nil
}

// sourceRunner plans and executes queries for one source.
type sourceRunner interface {
	plan(ctx context.Context, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error)
	execute(ctx context.Context, queryID string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error)
}

// runnerFor picks how a source's queries run: through a long-running local plugin process with the decrypted stored config, or through the plugin that serves the source from outside Marmot, which holds its own config.
func (s *service) runnerFor(source *Source) (sourceRunner, error) {
	if source.Served {
		if s.opts.Served == nil {
			return nil, fmt.Errorf("%w: serving plugins is not enabled", ErrQueryUnsupported)
		}
		return servedRunner{runner: s.opts.Served, source: source.Name}, nil
	}

	entry, err := plugin.GetRegistry().Get(source.PluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin %q for source %q is not loaded: %w", source.PluginID, source.Name, err)
	}
	if !entry.Meta.SupportsQuery {
		return nil, fmt.Errorf("%w: plugin %q", ErrQueryUnsupported, source.PluginID)
	}

	config := make(map[string]any, len(source.Config))
	for k, v := range source.Config {
		config[k] = v
	}
	if s.encryptor != nil {
		if err := plugin.DecryptConfigForPlugin(source.PluginID, config, s.encryptor); err != nil {
			return nil, fmt.Errorf("decrypting source config: %w", err)
		}
	}
	return localRunner{runner: s.runner, pluginID: source.PluginID, config: config}, nil
}

type localRunner struct {
	runner   QueryRunner
	pluginID string
	config   map[string]any
}

func (l localRunner) plan(ctx context.Context, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	return l.runner.PlanQuery(ctx, l.pluginID, l.config, req)
}

func (l localRunner) execute(ctx context.Context, _ string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	return l.runner.ExecuteQuery(ctx, l.pluginID, l.config, req)
}

type servedRunner struct {
	runner ServedRunner
	source string
}

func (r servedRunner) plan(ctx context.Context, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	return r.runner.PlanQuery(ctx, r.source, req)
}

func (r servedRunner) execute(ctx context.Context, queryID string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	return r.runner.ExecuteQuery(ctx, r.source, queryID, req)
}

// planResources asks the plugin what the statement touches. A plugin that
// cannot plan (Unimplemented, or a nil plan for this statement) falls back to
// the whole-source resource, so the authorizer must allow the entire source. So does
// a plan the plugin marks incomplete, because the statement may read tables the
// plan does not list; the listed tables are kept for the query log.
func (s *service) planResources(ctx context.Context, source *Source, runner sourceRunner, req pluginsdk.QueryRequest) ([]string, error) {
	plan, err := runner.plan(ctx, req)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return []string{SourceMRN(source.Name)}, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrQueryFailed, pluginMessage(err))
	}
	if plan == nil || len(plan.ReferencedMRNs) == 0 {
		return []string{SourceMRN(source.Name)}, nil
	}
	if plan.Incomplete {
		return append(slices.Clone(plan.ReferencedMRNs), SourceMRN(source.Name)), nil
	}
	return plan.ReferencedMRNs, nil
}

func (s *service) execute(ctx context.Context, runner sourceRunner, queryID string, req pluginsdk.QueryRequest, maxRows int64) (*QueryResult, error) {
	stream, err := runner.execute(ctx, queryID, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	result := &QueryResult{Columns: []QueryColumn{}, Rows: [][]any{}}
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return nil, err
		}
		if len(chunk.Columns) > 0 && len(result.Columns) == 0 {
			result.Columns = make([]QueryColumn, len(chunk.Columns))
			for i, c := range chunk.Columns {
				result.Columns[i] = QueryColumn{Name: c.Name, Type: c.Type}
			}
		}
		for _, row := range chunk.Rows {
			if result.RowCount >= maxRows {
				result.Truncated = true
				return result, nil
			}
			result.Rows = append(result.Rows, row)
			result.RowCount++
		}
	}
}

// assetContext snapshots the catalog's view of every referenced asset, so
// agents receive meaning together with rows and the query log keeps what the
// catalog said at the time.
func (s *service) assetContext(ctx context.Context, mrns []string) []AssetContext {
	var catalogued []string
	for _, m := range mrns {
		if !isSourceMRN(m) {
			catalogued = append(catalogued, m)
		}
	}
	if len(catalogued) == 0 || s.assets == nil {
		return nil
	}

	assets, err := s.assets.GetByMRNs(ctx, catalogued)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load asset context for data query")
		return nil
	}

	var contexts []AssetContext
	for _, m := range catalogued {
		a, ok := assets[m]
		if !ok || a == nil {
			continue
		}
		ac := AssetContext{
			MRN:       m,
			AssetID:   a.ID,
			Type:      a.Type,
			Providers: a.Providers,
			Tags:      a.Tags,
		}
		if !a.LastSyncAt.IsZero() {
			lastSync := a.LastSyncAt
			ac.LastSyncAt = &lastSync
		}
		if a.Name != nil {
			ac.Name = *a.Name
		}
		if a.UserDescription != nil && *a.UserDescription != "" {
			ac.Description = *a.UserDescription
		} else if a.Description != nil {
			ac.Description = *a.Description
		}
		contexts = append(contexts, ac)
	}
	return contexts
}

func (s *service) complete(ctx context.Context, id string, outcome QueryOutcome) {
	// Record the outcome even when the caller has gone away mid-query.
	ctx = context.WithoutCancel(ctx)
	if err := s.repo.CompleteQuery(ctx, id, outcome); err != nil {
		log.Error().Err(err).Str("query_id", id).Msg("Failed to record data query outcome")
	}
}

// pluginMessage strips the gRPC envelope from an error a plugin returned, so
// callers see the engine's own message.
func pluginMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "the query timed out"
	}
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.DeadlineExceeded {
			return "the query timed out"
		}
		return st.Message()
	}
	return err.Error()
}

func decisionDetail(d Decision) map[string]any {
	detail := map[string]any{}
	if d.Reason != "" {
		detail["reason"] = d.Reason
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
}
