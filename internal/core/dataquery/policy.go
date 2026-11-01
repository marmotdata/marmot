package dataquery

import (
	"context"
	"slices"
	"strings"

	"github.com/marmotdata/marmot/internal/core/auth"
)

// Decision is the outcome of a policy check: whether the query may run and,
// if not, why.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	// Rule is the outcome reason recorded with the query (see ReasonRole); it defaults to role when allowed and permission when denied.
	Rule string `json:"rule,omitempty"`
	// Permission and Policy name what refused the query, Needed what would allow it, and Resources the MRNs that were refused, so a caller can ask for exactly that.
	Permission string   `json:"permission,omitempty"`
	Policy     string   `json:"policy,omitempty"`
	Needed     string   `json:"needed,omitempty"`
	Resources  []string `json:"resources,omitempty"`
} // @name QueryDecision

// Authorizer decides who may query and what. OSS Marmot admits anyone holding assets:query to every queryable source; the cloud distribution supplies one backed by its IAM, which decides table by table.
type Authorizer interface {
	// Admits reports whether the principal may use data queries at all, which gates the query routes and MCP tools before any statement is planned.
	Admits(ctx context.Context, principal auth.Principal) bool
	// Authorize decides whether a principal may run a planned query. resources are the MRNs of every table the plan names, plus the whole-source resource when the plan cannot name everything the statement reads (see SourceMRN).
	Authorize(ctx context.Context, principal auth.Principal, source *Source, resources []string) (Decision, error)
	// Visible says which sources and tables the principal may query, so listings only offer what a query would be allowed to read.
	Visible(ctx context.Context, principal auth.Principal) (Visibility, error)
}

// Visibility is the part of the catalog a principal may query. A zero Visibility admits nothing.
type Visibility struct {
	// All admits every table of every queryable source.
	All bool
	// SourceIDs are the sources whose every table may be queried.
	SourceIDs []string
	// Assets, when set, returns a SQL condition over an asset id expression that admits the tables which may be queried one by one, numbering its parameters from next.
	Assets func(idExpr string, next int) (sql string, args []any)
}

// coversSource reports whether every table of a source is visible.
func (v Visibility) coversSource(sourceID string) bool {
	return v.All || slices.Contains(v.SourceIDs, sourceID)
}

// permissionAuthorizer is the OSS Authorizer: holding assets:query admits a principal to every queryable source.
type permissionAuthorizer struct{}

func (permissionAuthorizer) Admits(_ context.Context, principal auth.Principal) bool {
	return principal != nil && (principal.IsAdmin() || principal.HasPermission("assets", "query"))
}

// queryPermission is the permission OSS checks, as it is spelled in a role.
const queryPermission = "assets:query"

func (a permissionAuthorizer) Authorize(ctx context.Context, principal auth.Principal, source *Source, _ []string) (Decision, error) {
	if a.Admits(ctx, principal) {
		return Decision{Allowed: true, Rule: ReasonRole}, nil
	}
	return permissionRefusal(source), nil
}

// permissionRefusal is the decision for a caller without assets:query, which covers every queryable source.
func permissionRefusal(source *Source) Decision {
	needed := queryPermission
	if source != nil && source.Name != "" {
		needed += " on " + source.Name
	}
	return Decision{
		Allowed:    false,
		Reason:     "querying data needs the assets:query permission",
		Rule:       ReasonPermission,
		Permission: queryPermission,
		Needed:     needed,
	}
}

func (a permissionAuthorizer) Visible(ctx context.Context, principal auth.Principal) (Visibility, error) {
	return Visibility{All: a.Admits(ctx, principal)}, nil
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
