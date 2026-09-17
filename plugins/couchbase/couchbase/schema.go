package couchbase

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// documentColumn is one field of the inferred document schema. Documents
// in a collection need not share a shape, so alongside the usual column
// fields it records how often the field was seen in the sample.
type documentColumn struct {
	pluginsdk.Column
	// Occurrence is the fraction of sampled documents holding the field,
	// between 0 and 1.
	Occurrence float64 `json:"occurrence"`
}

// fieldStats accumulates what the sample says about one field.
type fieldStats struct {
	types map[string]struct{}
	count int
}

func newFieldStats() *fieldStats {
	return &fieldStats{types: make(map[string]struct{})}
}

// columnsFromDocuments merges sampled documents into a column list: every
// top-level field, plus the fields of object values one level deep as
// parent.child. A field missing from some documents is nullable.
func columnsFromDocuments(docs []map[string]interface{}) []documentColumn {
	if len(docs) == 0 {
		return nil
	}

	fields := make(map[string]*fieldStats)
	for _, doc := range docs {
		for key, value := range doc {
			stats := fields[key]
			if stats == nil {
				stats = newFieldStats()
				fields[key] = stats
			}
			stats.types[jsonType(value)] = struct{}{}
			stats.count++

			nested, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			for childKey, childValue := range nested {
				name := key + "." + childKey
				childStats := fields[name]
				if childStats == nil {
					childStats = newFieldStats()
					fields[name] = childStats
				}
				childStats.types[jsonType(childValue)] = struct{}{}
				childStats.count++
			}
		}
	}

	return buildColumns(fields, len(docs))
}

// columnsFromInfer parses the rows of an INFER statement. The result is
// one row holding a list of schema "flavours", each a JSON Schema object
// with a "#docs" count and "properties"; the same field can appear in
// several flavours, so counts are summed and types unioned across them.
func columnsFromInfer(rows []json.RawMessage) ([]documentColumn, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	var flavours []inferFlavour
	if err := json.Unmarshal(rows[0], &flavours); err != nil {
		return nil, fmt.Errorf("decoding INFER result: %w", err)
	}

	fields := make(map[string]*fieldStats)
	totalDocs := 0
	for _, flavour := range flavours {
		totalDocs += flavour.Docs.total()
		mergeInferProperties(fields, "", flavour.Properties, true)
	}

	if totalDocs == 0 {
		return nil, nil
	}
	return buildColumns(fields, totalDocs), nil
}

func mergeInferProperties(fields map[string]*fieldStats, prefix string, properties map[string]inferProperty, descend bool) {
	for key, prop := range properties {
		name := prefix + key
		stats := fields[name]
		if stats == nil {
			stats = newFieldStats()
			fields[name] = stats
		}
		for _, t := range prop.Type.values() {
			stats.types[t] = struct{}{}
		}
		stats.count += prop.Docs.total()

		if descend && len(prop.Properties) > 0 {
			mergeInferProperties(fields, name+".", prop.Properties, false)
		}
	}
}

// inferFlavour is one entry of an INFER result: a JSON Schema for one
// shape of document found in the sample.
type inferFlavour struct {
	Docs       inferCount               `json:"#docs"`
	Properties map[string]inferProperty `json:"properties"`
}

type inferProperty struct {
	Type       inferType                `json:"type"`
	Docs       inferCount               `json:"#docs"`
	Properties map[string]inferProperty `json:"properties"`
}

// inferType is a JSON Schema type, which INFER writes as a string for one
// type and a list for a field seen with several.
type inferType []string

func (t *inferType) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*t = inferType{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*t = inferType(many)
	return nil
}

func (t inferType) values() []string { return []string(t) }

// inferCount is an INFER "#docs" value, a number for one type and a list
// of per-type numbers for a field seen with several.
type inferCount []float64

func (c *inferCount) UnmarshalJSON(data []byte) error {
	var single float64
	if err := json.Unmarshal(data, &single); err == nil {
		*c = inferCount{single}
		return nil
	}
	var many []float64
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*c = inferCount(many)
	return nil
}

func (c inferCount) total() int {
	sum := 0.0
	for _, n := range c {
		sum += n
	}
	return int(sum)
}

// buildColumns turns per-field statistics into a sorted column list. A
// nested field sorts right after its parent because "." orders before
// every character that can follow it in a field name.
func buildColumns(fields map[string]*fieldStats, totalDocs int) []documentColumn {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	columns := make([]documentColumn, 0, len(names))
	for _, name := range names {
		stats := fields[name]
		count := stats.count
		if count > totalDocs {
			count = totalDocs
		}
		columns = append(columns, documentColumn{
			Column: pluginsdk.Column{
				Name:     name,
				DataType: joinTypes(stats.types),
				Nullable: count < totalDocs,
			},
			Occurrence: math.Round(float64(count)/float64(totalDocs)*100) / 100,
		})
	}

	return columns
}

// joinTypes renders the set of observed types as one badge, joined with
// "|" when a field was seen with more than one.
func joinTypes(types map[string]struct{}) string {
	names := make([]string, 0, len(types))
	for t := range types {
		names = append(names, t)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

// jsonType names the JSON type of a value decoded with encoding/json.
func jsonType(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, json.Number, int, int64:
		return "number"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	}
	return "unknown"
}
