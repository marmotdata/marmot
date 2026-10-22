package dataquery

import (
	"context"
	"strings"

	"github.com/marmotdata/marmot/internal/core/auth"
)

// Decision is the outcome of a policy check: whether the query may run and,
// if not, why.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
} // @name QueryDecision

// Authorizer decides whether a principal may run a planned query. resources are
// the MRNs of every table the plan names, plus the whole-source resource when the
// plan cannot name everything the statement reads (see SourceMRN). OSS Marmot
// admits anyone holding assets:query to every queryable source; the cloud
// distribution supplies one backed by its IAM, which decides table by table.
type Authorizer interface {
	Authorize(ctx context.Context, principal auth.Principal, source *Source, resources []string) (Decision, error)
}

// permissionAuthorizer is the OSS Authorizer: holding assets:query, which the
// query routes also require, admits a principal to every queryable source.
type permissionAuthorizer struct{}

func (permissionAuthorizer) Authorize(_ context.Context, principal auth.Principal, _ *Source, _ []string) (Decision, error) {
	if principal.HasPermission("assets", "query") {
		return Decision{Allowed: true}, nil
	}
	return Decision{Allowed: false, Reason: "querying data needs the assets:query permission"}, nil
}

// SourceMRN names a whole source. A statement the engine cannot plan is
// checked against it, so an authorizer must allow the entire source.
func SourceMRN(sourceName string) string {
	return "mrn://source/" + strings.ToLower(sourceName)
}

// isSourceMRN reports whether an MRN is a whole-source fallback resource
// rather than a catalogued asset.
func isSourceMRN(mrn string) bool {
	return strings.HasPrefix(mrn, "mrn://source/")
}
