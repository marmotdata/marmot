package couchbase

import (
	"sort"

	"github.com/couchbase/gocb/v2"
)

// groupIndexes keys a bucket's indexes by "scope.collection". An index
// created on the bare bucket lives in the default collection and comes
// back from system:indexes with no scope or collection, so those are
// filed under _default._default.
func groupIndexes(indexes []gocb.QueryIndex) map[string][]gocb.QueryIndex {
	grouped := make(map[string][]gocb.QueryIndex)
	for _, index := range indexes {
		scope := index.ScopeName
		if scope == "" {
			scope = "_default"
		}
		collection := index.CollectionName
		if collection == "" {
			collection = "_default"
		}
		key := scope + "." + collection
		grouped[key] = append(grouped[key], index)
	}
	return grouped
}

func hasPrimaryIndex(indexes []gocb.QueryIndex) bool {
	for _, index := range indexes {
		if index.IsPrimary {
			return true
		}
	}
	return false
}

// indexNames lists index names sorted, so the metadata is stable between
// runs regardless of the order the query service returns them in.
func indexNames(indexes []gocb.QueryIndex) []string {
	names := make([]string, 0, len(indexes))
	for _, index := range indexes {
		names = append(names, index.Name)
	}
	sort.Strings(names)
	return names
}
