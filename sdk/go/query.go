package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/query"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// QuerySource is an ingestion source that answers queries through Marmot.
type QuerySource = models.QuerySource

// QuerySourceList is a page of queryable sources.
type QuerySourceList = models.QueryListSourcesResponse

// QuerySourceTable is a catalogued table a source can reach, with a preview
// statement in the source's own dialect.
type QuerySourceTable = models.QuerySourceTable

// QueryResult carries rows, the query's id in the log and the catalog
// context of every table it read.
type QueryResult = models.QueryResult

// QueryRecord is one query from the query log.
type QueryRecord = models.QueryRecord

// QueryHistory is a page of the query log.
type QueryHistory = models.QueryHistoryResponse

// QueryStats summarises query activity over a window.
type QueryStats = models.QueryStats

// QueryRunInput is one read-only statement to run against a source.
type QueryRunInput struct {
	Source    string
	Statement string
	MaxRows   int64
}

// QueryHistoryOptions filters QueryService.History.
type QueryHistoryOptions struct {
	Source   string
	Decision string
	Status   string
	MRN      string
	Limit    int64
	Offset   int64
}

// QueryService runs read-only queries against catalogued sources and reports on them.
type QueryService struct {
	gen *apiclient.Marmot
}

// Sources pages through queryable sources, optionally searching by name or
// plugin.
func (s *QueryService) Sources(ctx context.Context, search string, limit, offset int64) (*QuerySourceList, error) {
	p := query.NewGetAPIV1QuerySourcesParams().WithContext(ctx)
	if search != "" {
		p = p.WithQ(&search)
	}
	if limit > 0 {
		p = p.WithLimit(&limit)
	}
	if offset > 0 {
		p = p.WithOffset(&offset)
	}
	resp, err := s.gen.Query.GetAPIV1QuerySources(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Tables lists the catalogued tables a source can reach, busiest first.
func (s *QueryService) Tables(ctx context.Context, source, search string, limit int64) ([]*QuerySourceTable, error) {
	p := query.NewGetAPIV1QuerySourcesNameTablesParams().WithContext(ctx).WithName(source)
	if search != "" {
		p = p.WithQ(&search)
	}
	if limit > 0 {
		p = p.WithLimit(&limit)
	}
	resp, err := s.gen.Query.GetAPIV1QuerySourcesNameTables(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Run runs one read-only statement against a source as the caller.
func (s *QueryService) Run(ctx context.Context, input QueryRunInput) (*QueryResult, error) {
	q := query.NewPostAPIV1QueryRunParams().WithContext(ctx).WithRequest(&models.QueryRunInput{
		Source:    &input.Source,
		Statement: &input.Statement,
		MaxRows:   input.MaxRows,
	})
	resp, err := s.gen.Query.PostAPIV1QueryRun(q, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// History pages through the query log. Callers without access:view only see
// their own queries.
func (s *QueryService) History(ctx context.Context, opts QueryHistoryOptions) (*QueryHistory, error) {
	p := query.NewGetAPIV1QueryHistoryParams().WithContext(ctx)
	if opts.Source != "" {
		p = p.WithSource(&opts.Source)
	}
	if opts.Decision != "" {
		p = p.WithDecision(&opts.Decision)
	}
	if opts.Status != "" {
		p = p.WithStatus(&opts.Status)
	}
	if opts.MRN != "" {
		p = p.WithMrn(&opts.MRN)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Query.GetAPIV1QueryHistory(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches one query from the log, with the catalog context recorded
// when it ran.
func (s *QueryService) Get(ctx context.Context, id string) (*QueryRecord, error) {
	resp, err := s.gen.Query.GetAPIV1QueryHistoryID(query.NewGetAPIV1QueryHistoryIDParams().WithContext(ctx).WithID(id), nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Stats summarises activity between from and to, both RFC 3339 and optional.
func (s *QueryService) Stats(ctx context.Context, from, to string) (*QueryStats, error) {
	p := query.NewGetAPIV1QueryStatsParams().WithContext(ctx)
	if from != "" {
		p = p.WithFrom(&from)
	}
	if to != "" {
		p = p.WithTo(&to)
	}
	resp, err := s.gen.Query.GetAPIV1QueryStats(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
