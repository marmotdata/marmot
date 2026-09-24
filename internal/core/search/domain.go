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

// DomainResolver turns the references of @domain filters (ids, names or
// name paths) into a DomainFilter. It returns ErrUnknownDomain when none of
// them names a domain.
type DomainResolver func(ctx context.Context, refs []string) (*DomainFilter, error)

var ErrUnknownDomain = errors.New("unknown domain")

const domainTokenPattern = `@domain\s*[:=]\s*(?:"([^"]+)"|([^\s"()]+))`

var (
	domainFilterRegex = regexp.MustCompile(`(?i)` + domainTokenPattern)
	domainTokenRegex  = regexp.MustCompile(`(?i)(?:\b(?:AND|OR)\s+)?` + domainTokenPattern)
	danglingRegex     = regexp.MustCompile(`(?i)^\s*(?:AND|OR)\b|\b(?:AND|OR|NOT)\s*$|\(\s*\)`)
	spacesRegex       = regexp.MustCompile(`\s+`)
)

// SetDomainResolver enables @domain filters. Without a resolver the token is
// left in the query, as upstream would treat it.
func (r *PostgresRepository) SetDomainResolver(resolver DomainResolver) {
	r.domainResolver = resolver
}

func extractDomainRefs(query string) []string {
	var refs []string
	for _, m := range domainFilterRegex.FindAllStringSubmatch(query, -1) {
		ref := strings.TrimSpace(m[1] + m[2])
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

// stripDomainFilter removes the tokens with the boolean operator that joined
// them, since the domain filter always narrows the whole query.
func stripDomainFilter(query string) string {
	stripped := domainTokenRegex.ReplaceAllString(query, "")
	for {
		next := strings.TrimSpace(danglingRegex.ReplaceAllString(stripped, ""))
		if next == stripped {
			break
		}
		stripped = next
	}
	return strings.TrimSpace(spacesRegex.ReplaceAllString(stripped, " "))
}

// resolveDomainFilter applies any @domain tokens in filter.Query. empty reports
// a filter on an unknown domain, which matches nothing.
func (r *PostgresRepository) resolveDomainFilter(ctx context.Context, filter *Filter) (empty bool, err error) {
	if r.domainResolver == nil {
		return false, nil
	}
	refs := extractDomainRefs(filter.Query)
	if len(refs) == 0 {
		return false, nil
	}
	resolved, err := r.domainResolver(ctx, refs)
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
