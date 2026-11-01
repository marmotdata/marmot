package athena

import (
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// tableIndex remembers the tables a run discovered so the tables a saved
// query reads can be turned into lineage edges. The server drops an edge
// whose endpoint has no asset behind it, so a name that resolves to nothing
// is skipped rather than guessed at.
type tableIndex struct {
	byQualified map[string]string
	byName      map[string]string
}

func newTableIndex() *tableIndex {
	return &tableIndex{
		byQualified: make(map[string]string),
		byName:      make(map[string]string),
	}
}

func (i *tableIndex) add(database, table, mrnValue string) {
	i.byQualified[qualifiedKey(database, table)] = mrnValue
	i.byName[strings.ToLower(table)] = mrnValue
}

// resolve finds the asset a query's table reference points at. An
// unqualified name is tried against the query's own database first, because
// that is what Athena itself would resolve it to.
func (i *tableIndex) resolve(ref tableRef, defaultDatabase string) (string, bool) {
	if ref.Database != "" {
		mrnValue, ok := i.byQualified[qualifiedKey(ref.Database, ref.Table)]
		return mrnValue, ok
	}

	if defaultDatabase != "" {
		if mrnValue, ok := i.byQualified[qualifiedKey(defaultDatabase, ref.Table)]; ok {
			return mrnValue, true
		}
	}

	mrnValue, ok := i.byName[strings.ToLower(ref.Table)]
	return mrnValue, ok
}

// queryEdges builds one FEEDS edge per table a saved query reads.
func (i *tableIndex) queryEdges(query, defaultDatabase, targetMRN string) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, ref := range extractQueryTables(query) {
		source, ok := i.resolve(ref, defaultDatabase)
		if !ok || source == targetMRN {
			continue
		}
		if _, duplicate := seen[source]; duplicate {
			continue
		}
		seen[source] = struct{}{}

		edges = append(edges, edge(source, targetMRN, "FEEDS"))
	}

	return edges
}

// qualifiedKey is case insensitive because Athena lower-cases the names it
// stores, while a query can spell them any way.
func qualifiedKey(database, table string) string {
	return strings.ToLower(database) + "." + strings.ToLower(table)
}
