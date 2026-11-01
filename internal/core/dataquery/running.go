package dataquery

import (
	"context"
	"sync"
	"sync/atomic"
)

// runningQueries holds the cancel handle and rows received so far of every query executing on this replica. Each replica only knows its own, so cancelling or watching a query works on the replica that runs it; the caller's own connection always reaches that one.
type runningQueries struct {
	mu      sync.Mutex
	queries map[string]*runningQuery
}

type runningQuery struct {
	cancel context.CancelCauseFunc
	rows   atomic.Int64
}

func newRunningQueries() *runningQueries {
	return &runningQueries{queries: map[string]*runningQuery{}}
}

// add registers a query and returns the counter execution keeps its received rows in.
func (r *runningQueries) add(id string, cancel context.CancelCauseFunc) *atomic.Int64 {
	q := &runningQuery{cancel: cancel}
	r.mu.Lock()
	r.queries[id] = q
	r.mu.Unlock()
	return &q.rows
}

func (r *runningQueries) remove(id string) {
	r.mu.Lock()
	delete(r.queries, id)
	r.mu.Unlock()
}

// cancel stops a running query, reporting whether this replica was running it.
func (r *runningQueries) cancel(id string) bool {
	r.mu.Lock()
	q, ok := r.queries[id]
	r.mu.Unlock()
	if ok {
		q.cancel(errCancelled)
	}
	return ok
}

// fillProgress reports the rows a running query has received so far, when it runs here.
func (r *runningQueries) fillProgress(q *Query) {
	if q == nil || q.Status != StatusRunning {
		return
	}
	r.mu.Lock()
	running, ok := r.queries[q.ID]
	r.mu.Unlock()
	if ok {
		rows := running.rows.Load()
		q.RowsReturned = &rows
	}
}
