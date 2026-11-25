package search

import (
	"regexp"
	"strings"
)

// @kind takes any result type, so a type an edition adds to the index (such as the cloud's tasks) can be filtered by name like the catalogue's own.
var kindRegex = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?([a-z][a-z_]*)"?`)

// extractKindFilters parses @kind filters from a query string.
// Returns "__CONTRADICTION__" if multiple kinds are specified (nothing can be multiple types).
func extractKindFilters(queryStr string) []ResultType {
	if queryStr == "" || !strings.Contains(queryStr, "@kind") {
		return nil
	}

	seen := map[ResultType]bool{}
	var kinds []ResultType
	for _, m := range kindRegex.FindAllStringSubmatch(queryStr, -1) {
		k := ResultType(strings.ToLower(m[1]))
		if !seen[k] {
			seen[k] = true
			kinds = append(kinds, k)
		}
	}

	if len(kinds) > 1 {
		return []ResultType{"__CONTRADICTION__"}
	}

	return kinds
}

// stripKindFilter removes @kind filters from a query string.
func stripKindFilter(queryStr string) string {
	if !strings.Contains(queryStr, "@kind") {
		return queryStr
	}

	result := kindRegex.ReplaceAllString(queryStr, "")
	result = strings.TrimSpace(result)

	result = strings.ReplaceAll(result, "AND AND", "AND")
	result = strings.ReplaceAll(result, "OR OR", "OR")
	result = strings.TrimPrefix(result, "AND ")
	result = strings.TrimPrefix(result, "OR ")
	result = strings.TrimSuffix(result, " AND")
	result = strings.TrimSuffix(result, " OR")
	result = strings.TrimSpace(result)
	// Two filters joined by an operator leave the operator alone once both are gone
	if strings.EqualFold(result, "AND") || strings.EqualFold(result, "OR") {
		return ""
	}

	return result
}
