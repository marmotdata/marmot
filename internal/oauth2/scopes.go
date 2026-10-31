package oauth2

import "fmt"

// ScopeDescription says in plain words what allowing a scope lets a client do, for the consent screen and the list of connected apps.
type ScopeDescription struct {
	Scope   string `json:"scope"`
	Summary string `json:"summary"`
} // @name OAuthScopeDescription

// scopeSummaries covers every scope Marmot advertises. Tokens are not narrowed by scope, so openid is described as what it really grants: the person's own access.
var scopeSummaries = map[string]string{
	"openid": "Use Marmot as you, with the permissions your roles give you. Everything it does is recorded under your name.",
}

// DescribeScopes returns a description for each scope, falling back to naming an unknown scope rather than hiding it.
func DescribeScopes(scopes []string) []ScopeDescription {
	out := make([]ScopeDescription, 0, len(scopes))
	for _, scope := range scopes {
		summary, ok := scopeSummaries[scope]
		if !ok {
			summary = fmt.Sprintf("Use the %q scope.", scope)
		}
		out = append(out, ScopeDescription{Scope: scope, Summary: summary})
	}
	return out
}
