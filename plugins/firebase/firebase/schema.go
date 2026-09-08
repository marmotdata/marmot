package firebase

import (
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/genproto/googleapis/type/latlng"
)

// Firestore stores no schema, so a collection's columns are inferred from a
// sample of its documents, the same way the MongoDB plugin infers a
// collection's fields.

// fieldShape is what one field looked like across the sampled documents.
type fieldShape struct {
	types   map[string]struct{}
	present int
}

// inferColumns turns sampled documents into one column per field. Documents
// in a collection are not required to agree, so a field missing from any
// document, or null in any of them, is nullable, and a field seen with two
// different types is "mixed". Columns come back sorted by name so the schema
// does not churn between runs.
func inferColumns(documents []map[string]any) []pluginsdk.Column {
	shapes := make(map[string]*fieldShape)

	for _, document := range documents {
		for name, value := range document {
			shape, ok := shapes[name]
			if !ok {
				shape = &fieldShape{types: make(map[string]struct{})}
				shapes[name] = shape
			}
			shape.present++
			shape.types[firestoreValueType(value)] = struct{}{}
		}
	}

	names := make([]string, 0, len(shapes))
	for name := range shapes {
		names = append(names, name)
	}
	sort.Strings(names)

	columns := make([]pluginsdk.Column, 0, len(names))
	for _, name := range names {
		shape := shapes[name]
		_, sawNull := shape.types["null"]
		columns = append(columns, pluginsdk.Column{
			Name:     name,
			DataType: columnDataType(shape.types),
			Nullable: sawNull || shape.present < len(documents),
		})
	}

	return columns
}

// columnDataType collapses the types one field was seen with into one name.
// Null on its own is all that is known about a field that was never set;
// beside a real type it only makes the column nullable, and two real types
// make it mixed.
func columnDataType(types map[string]struct{}) string {
	var seen []string
	for name := range types {
		if name == "null" {
			continue
		}
		seen = append(seen, name)
	}

	switch len(seen) {
	case 0:
		return "null"
	case 1:
		return seen[0]
	default:
		return "mixed"
	}
}

// firestoreValueType names the Firestore type of a value. The Go client
// converts the wire types into native Go values before handing them over, so
// the mapping is by Go type rather than by the name of the wire field.
func firestoreValueType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int64:
		return "integer"
	case float64:
		return "double"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case time.Time:
		return "timestamp"
	case *latlng.LatLng:
		return "geopoint"
	case *firestore.DocumentRef:
		return "reference"
	case firestore.Vector32, firestore.Vector64:
		return "vector"
	case []any:
		return "array"
	case map[string]any:
		return "map"
	default:
		return "unknown"
	}
}
