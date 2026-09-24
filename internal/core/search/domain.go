package search

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DomainFilter narrows results by domain subtrees. It is fork-only; see
// DOMAINS.md.
type DomainFilter struct {
	// Paths are the materialized paths of the selected domains; each matches
	// its whole subtree.
	Paths []string
	// Unassigned also matches entities with no membership row.
	Unassigned bool
	// Exclude removes the entities in these subtrees, from NOT @domain tokens.
	Exclude *DomainFilter
}

func (f *DomainFilter) selects() bool {
	return f != nil && (len(f.Paths) > 0 || f.Unassigned)
}

// DomainResolver turns the references of @domain filters (ids, names or
// name paths) into a DomainFilter. It returns ErrUnknownDomain when none of
// them names a domain.
type DomainResolver func(ctx context.Context, refs []string) (*DomainFilter, error)

var ErrUnknownDomain = errors.New("unknown domain")

const domainTokenPattern = `(?:\b(NOT)\s+)?@domain\s*[:=]\s*(?:"([^"]+)"|([^\s"()]+))`

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

// extractDomainRefs splits the @domain references into those that select and
// those a NOT excludes.
func extractDomainRefs(query string) (include, exclude []string) {
	for _, m := range domainFilterRegex.FindAllStringSubmatch(query, -1) {
		ref := strings.TrimSpace(m[2] + m[3])
		if ref == "" {
			continue
		}
		if m[1] != "" {
			exclude = append(exclude, ref)
		} else {
			include = append(include, ref)
		}
	}
	return include, exclude
}

// stripDomainFilter removes the tokens with the boolean operator that joined
// them, since the domain filter always applies to the whole query.
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
// a selection of unknown domains, which matches nothing; excluding an unknown
// domain excludes nothing.
func (r *PostgresRepository) resolveDomainFilter(ctx context.Context, filter *Filter) (empty bool, err error) {
	if r.domainResolver == nil {
		return false, nil
	}
	include, exclude := extractDomainRefs(filter.Query)
	if len(include) == 0 && len(exclude) == 0 {
		return false, nil
	}
	resolved := &DomainFilter{}
	if len(include) > 0 {
		resolved, err = r.domainResolver(ctx, include)
		if errors.Is(err, ErrUnknownDomain) {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("resolving domain filter: %w", err)
		}
	}
	if len(exclude) > 0 {
		excluded, err := r.domainResolver(ctx, exclude)
		switch {
		case errors.Is(err, ErrUnknownDomain):
		case err != nil:
			return false, fmt.Errorf("resolving domain filter: %w", err)
		default:
			resolved.Exclude = excluded
		}
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

// appendDomainClauses restricts results to the filter's subtrees and drops
// the excluded ones. The member sets are uncorrelated subqueries, so Postgres
// hashes each once instead of probing it per search_index row. Teams belong
// to no domain: a selection never matches them and an exclusion never drops
// them.
func appendDomainClauses(f *DomainFilter, clauses []string, params []interface{}) ([]string, []interface{}) {
	if f == nil {
		return clauses, params
	}
	if f.selects() {
		var members string
		members, params = domainMembers(f, params)
		clauses = append(clauses, members)
	}
	if f.Exclude.selects() {
		var members string
		members, params = domainMembers(f.Exclude, params)
		clauses = append(clauses, "NOT "+members)
	}
	return clauses, params
}

func domainMembers(f *DomainFilter, params []interface{}) (string, []interface{}) {
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
	return "(" + strings.Join(ors, " OR ") + ")", params
}
