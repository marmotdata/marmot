package presto

import (
	"regexp"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// referencePattern finds the table named right after FROM or JOIN in a
// view definition: one to three dot-separated identifiers, each either
// bare or double-quoted. Subqueries start with "(" and so never match.
var referencePattern = regexp.MustCompile(
	`(?i)\b(?:FROM|JOIN)\s+((?:"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$]*)(?:\s*\.\s*(?:"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$]*)){0,2})`,
)

// identifierPattern splits a matched reference into its parts.
var identifierPattern = regexp.MustCompile(`"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$]*`)

// extractTableReferences returns every table a view definition reads
// from, as identifier parts. Bare identifiers are lowercased the way
// Presto folds them; quoted ones keep their case.
func extractTableReferences(definition string) [][]string {
	var references [][]string
	for _, match := range referencePattern.FindAllStringSubmatch(definition, -1) {
		var parts []string
		for _, ident := range identifierPattern.FindAllString(match[1], -1) {
			parts = append(parts, unquoteIdentifier(ident))
		}
		if len(parts) > 0 {
			references = append(references, parts)
		}
	}
	return references
}

func unquoteIdentifier(ident string) string {
	if strings.HasPrefix(ident, `"`) && strings.HasSuffix(ident, `"`) && len(ident) >= 2 {
		return strings.ReplaceAll(ident[1:len(ident)-1], `""`, `"`)
	}
	return strings.ToLower(ident)
}

// resolveReference completes a reference to a full Presto path using the
// view's own catalog and schema for the parts it leaves out.
func resolveReference(parts []string, catalog, schema string) tableKey {
	switch len(parts) {
	case 3:
		return tableKey{parts[0], parts[1], parts[2]}
	case 2:
		return tableKey{catalog, parts[0], parts[1]}
	default:
		return tableKey{catalog, schema, parts[0]}
	}
}

// viewLineage emits a VIEW_OF edge from each base table to the view that
// reads it, for base tables discovered in this run. References to
// anything else (CTEs, functions, tables in skipped catalogs) resolve to
// nothing and are dropped.
func viewLineage(assets []pluginsdk.Asset) []pluginsdk.LineageEdge {
	mrns := make(map[tableKey]string, len(assets))
	for _, a := range assets {
		if a.Type == "Table" || a.Type == "View" {
			mrns[assetKey(a)] = *a.MRN
		}
	}

	var lineages []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, a := range assets {
		if a.Type != "View" || a.Query == nil {
			continue
		}
		view := assetKey(a)

		for _, parts := range extractTableReferences(*a.Query) {
			base := resolveReference(parts, view.Catalog, view.Schema)
			baseMRN, ok := mrns[base]
			if !ok {
				log.Debug().Str("view", view.String()).Str("reference", base.String()).
					Msg("Skipping VIEW_OF edge, base table not discovered")
				continue
			}
			if baseMRN == *a.MRN {
				continue
			}

			edgeKey := baseMRN + ">" + *a.MRN
			if _, dup := seen[edgeKey]; dup {
				continue
			}
			seen[edgeKey] = struct{}{}

			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: baseMRN,
				Target: *a.MRN,
				Type:   "VIEW_OF",
			})
		}
	}

	return lineages
}
