package search

import (
	"regexp"
	"strings"
)

var (
	kindGlossaryRegex    = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?glossary"?`)
	kindAssetRegex       = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?asset"?`)
	kindTeamRegex        = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?team"?`)
	kindDataProductRegex = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?data_product"?`)
	kindStripRegex       = regexp.MustCompile(`(?i)@kind\s*[:=]\s*"?(glossary|asset|team|data_product)"?`)
)

// extractKindFilters parses @kind filters from a query string.
// Returns "__CONTRADICTION__" if multiple kinds are specified (nothing can be multiple types).
func extractKindFilters(queryStr string) []ResultType {
	if queryStr == "" || !strings.Contains(queryStr, "@kind") {
		return nil
	}

	var kinds []ResultType

	if kindGlossaryRegex.MatchString(queryStr) {
		kinds = append(kinds, ResultTypeGlossary)
	}
	if kindAssetRegex.MatchString(queryStr) {
		kinds = append(kinds, ResultTypeAsset)
	}
	if kindTeamRegex.MatchString(queryStr) {
		kinds = append(kinds, ResultTypeTeam)
	}
	if kindDataProductRegex.MatchString(queryStr) {
		kinds = append(kinds, ResultTypeDataProduct)
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

	result := kindStripRegex.ReplaceAllString(queryStr, "")
	result = strings.TrimSpace(result)

	result = strings.ReplaceAll(result, "AND AND", "AND")
	result = strings.ReplaceAll(result, "OR OR", "OR")
	result = strings.TrimPrefix(result, "AND ")
	result = strings.TrimPrefix(result, "OR ")
	result = strings.TrimSuffix(result, " AND")
	result = strings.TrimSuffix(result, " OR")

	return strings.TrimSpace(result)
}

// searchTypeIncluded checks if a result type should be included in search.
// Returns true if no filter is specified (include all) or if target is in the list.
func searchTypeIncluded(types []ResultType, target ResultType) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
