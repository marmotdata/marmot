package lineage

import "context"

// EdgeGuard vets an edge before it is written, whoever asks for it: the API,
// ingestion runs, or the edges an OpenLineage event creates internally. A
// refused edge is not written; callers that tolerate partial results, such as
// OpenLineage, log it and carry on.
type EdgeGuard func(ctx context.Context, sourceMRN, targetMRN string) error

func WithEdgeGuard(guard EdgeGuard) ServiceOption {
	return func(s *service) {
		s.edgeGuard = guard
	}
}
