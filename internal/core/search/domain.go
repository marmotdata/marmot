package search

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DomainFilter narrows results to entities owned by a set of domain subtrees.
// It is fork-only; see DOMAINS.md.
type DomainFilter struct {
	// Paths are the materialized paths of the selected domains; each matches
	// its whole subtree.
	Paths []string
	// Unassigned also matches entities with no membership row.
	Unassigned bool
}

// DomainResolver turns the domain ids of an @domain filter into a
// DomainFilter. It returns ErrUnknownDomain when an id does not exist.
type DomainResolver func(ctx context.Context, ids []string) (*DomainFilter, error)

var ErrUnknownDomain = errors.New("unknown domain")

var (
	domainFilterRegex = regexp.MustCompile(`(?i)@domain\s*[:=]\s*"?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})"?`)
	spacesRegex       = regexp.MustCompile(`\s+`)
)

// SetDomainResolver enables @domain filters. Without a resolver the token is
// left in the query, as upstream would treat it.
func (r *PostgresRepository) SetDomainResolver(resolver DomainResolver) {
	r.domainResolver = resolver
}

func extractDomainIDs(query string) []string {
	var ids []string
	for _, m := range domainFilterRegex.FindAllStringSubmatch(query, -1) {
		ids = append(ids, strings.ToLower(m[1]))
	}
	return ids
}

func stripDomainFilter(query string) string {
	stripped := domainFilterRegex.ReplaceAllString(query, "")
	return strings.TrimSpace(spacesRegex.ReplaceAllString(stripped, " "))
}

// resolveDomainFilter applies any @domain tokens in filter.Query. empty reports
// a filter on an unknown domain, which matches nothing.
func (r *PostgresRepository) resolveDomainFilter(ctx context.Context, filter *Filter) (empty bool, err error) {
	if r.domainResolver == nil {
		return false, nil
	}
	ids := extractDomainIDs(filter.Query)
	if len(ids) == 0 {
		return false, nil
	}
	resolved, err := r.domainResolver(ctx, ids)
	if errors.Is(err, ErrUnknownDomain) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("resolving domain filter: %w", err)
	}
	filter.Domain = resolved
	filter.Query = stripDomainFilter(filter.Query)
	return false, nil
}

var domainMemberships = []struct{ resultType, table, column string }{
	{"asset", "asset_domains", "asset_id"},
	{"data_product", "data_product_domains", "data_product_id"},
	{"glossary", "glossary_term_domains", "glossary_term_id"},
}

// appendDomainClauses restricts results to the filter's subtrees. The member
// sets are uncorrelated subqueries, so Postgres hashes each once instead of
// probing it per search_index row. Teams belong to no domain and never match.
func appendDomainClauses(f *DomainFilter, clauses []string, params []interface{}) ([]string, []interface{}) {
	if f == nil {
		return clauses, params
	}
	params = append(params, f.Paths)
	pathsParam := len(params)

	var ors []string
	for _, m := range domainMemberships {
		cond := fmt.Sprintf(`entity_id IN (
			SELECT mm.%[2]s::text FROM %[1]s mm JOIN domains d ON d.id = mm.domain_id
			 WHERE d.path LIKE ANY (SELECT p || '%%' FROM unnest($%[3]d::text[]) p))`,
			m.table, m.column, pathsParam)
		if f.Unassigned {
			cond = fmt.Sprintf("(%s OR entity_id NOT IN (SELECT %s::text FROM %s))", cond, m.column, m.table)
		}
		ors = append(ors, fmt.Sprintf("(type = '%s' AND %s)", m.resultType, cond))
	}
	return append(clauses, "("+strings.Join(ors, " OR ")+")"), params
}
