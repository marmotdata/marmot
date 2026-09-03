package gateway

import (
	"regexp"
	"strings"
	"sync"

	"github.com/marmotdata/marmot/internal/core/auth"
)

// ActionQuery is the grant action the query path checks for.
const ActionQuery = "query"

// Decision is the outcome of a policy check: whether the query may run and
// which grant matched each referenced resource, or why it was denied.
type Decision struct {
	Allowed       bool              `json:"allowed"`
	Reason        string            `json:"reason,omitempty"`
	MatchedGrants map[string]string `json:"matched_grants,omitempty"`
} // @name GatewayDecision

// Decide is the query gateway's policy decision function: a query is allowed
// only when every referenced resource matches at least one live grant held by
// the principal. Admin principals pass unconditionally, mirroring the root
// token pattern used by Principal.HasPermission. The caller supplies live
// grants only (not expired, not revoked); Decide is a pure function so both
// the direct query path and a future brokered policy endpoint share it.
func Decide(principal auth.Principal, grants []*Grant, resources []string) Decision {
	if principal.IsAdmin() {
		return Decision{Allowed: true}
	}

	matched := make(map[string]string, len(resources))
	for _, resource := range resources {
		grantID, ok := matchResource(grants, resource)
		if !ok {
			return Decision{
				Allowed: false,
				Reason:  "no grant covers " + resource,
			}
		}
		matched[resource] = grantID
	}
	return Decision{Allowed: true, MatchedGrants: matched}
}

func matchResource(grants []*Grant, resource string) (string, bool) {
	for _, g := range grants {
		if !grantHasAction(g, ActionQuery) {
			continue
		}
		if MatchSelector(g.ResourceSelector, resource) {
			return g.ID, true
		}
	}
	return "", false
}

func grantHasAction(g *Grant, action string) bool {
	for _, a := range g.Actions {
		if a == action {
			return true
		}
	}
	return false
}

var (
	selectorCacheMu sync.RWMutex
	selectorCache   = map[string]*regexp.Regexp{}
)

// MatchSelector reports whether an MRN matches a grant selector. Selectors
// are MRN globs matched case-insensitively: `**` spans path segments, `*`
// matches within one segment and everything else is literal, so
// mrn://postgresql/ecommerce/** covers every asset under that service while
// mrn://postgresql/*/orders covers one table across services.
func MatchSelector(selector, mrn string) bool {
	re, err := compileSelector(selector)
	if err != nil {
		return false
	}
	return re.MatchString(strings.ToLower(mrn))
}

func compileSelector(selector string) (*regexp.Regexp, error) {
	selectorCacheMu.RLock()
	re, ok := selectorCache[selector]
	selectorCacheMu.RUnlock()
	if ok {
		return re, nil
	}

	var b strings.Builder
	b.WriteString("^")
	rest := strings.ToLower(selector)
	for rest != "" {
		switch {
		case strings.HasPrefix(rest, "**"):
			b.WriteString(".*")
			rest = rest[2:]
		case strings.HasPrefix(rest, "*"):
			b.WriteString("[^/]*")
			rest = rest[1:]
		default:
			b.WriteString(regexp.QuoteMeta(rest[:1]))
			rest = rest[1:]
		}
	}
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, err
	}

	selectorCacheMu.Lock()
	selectorCache[selector] = re
	selectorCacheMu.Unlock()
	return re, nil
}

// TargetMRN is the resource a query is checked against when the engine
// cannot plan the statement: a grant must cover the whole target.
func TargetMRN(targetName string) string {
	return "mrn://target/" + strings.ToLower(targetName)
}
