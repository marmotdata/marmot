package firebase

import (
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/type/latlng"
)

func columnByName(t *testing.T, documents []map[string]any, name string) pluginsdk.Column {
	t.Helper()

	for _, column := range inferColumns(documents) {
		if column.Name == name {
			return column
		}
	}

	require.FailNowf(t, "column not found", "no column named %q", name)
	return pluginsdk.Column{}
}

func TestInferColumns_EmptySampleHasNoColumns(t *testing.T) {
	assert.Empty(t, inferColumns(nil))
}

func TestInferColumns_CollectionOfEmptyDocumentsHasNoColumns(t *testing.T) {
	assert.Empty(t, inferColumns([]map[string]any{{}, {}}))
}

func TestInferColumns_SortsColumnsByName(t *testing.T) {
	documents := []map[string]any{
		{"total": 10.5, "customer": "alice", "id": int64(1)},
	}

	columns := inferColumns(documents)

	require.Len(t, columns, 3)
	assert.Equal(t, "customer", columns[0].Name)
	assert.Equal(t, "id", columns[1].Name)
	assert.Equal(t, "total", columns[2].Name)
}

func TestInferColumns_IdenticalDocumentsGiveRequiredColumns(t *testing.T) {
	documents := []map[string]any{
		{"id": int64(1), "name": "alice"},
		{"id": int64(2), "name": "bob"},
	}

	columns := inferColumns(documents)

	require.Len(t, columns, 2)
	assert.False(t, columns[0].Nullable)
	assert.False(t, columns[1].Nullable)
}

func TestInferColumns_AbsentFieldIsNullable(t *testing.T) {
	documents := []map[string]any{
		{"id": int64(1), "nickname": "ali"},
		{"id": int64(2)},
	}

	nickname := columnByName(t, documents, "nickname")

	assert.True(t, nickname.Nullable)
	assert.Equal(t, "string", nickname.DataType)
}

func TestInferColumns_NullValueIsNullable(t *testing.T) {
	documents := []map[string]any{
		{"nickname": "ali"},
		{"nickname": nil},
	}

	nickname := columnByName(t, documents, "nickname")

	assert.True(t, nickname.Nullable)
	assert.Equal(t, "string", nickname.DataType, "a null beside a string does not make the field mixed")
}

func TestInferColumns_FieldThatIsAlwaysNullHasTheNullType(t *testing.T) {
	documents := []map[string]any{
		{"deleted_at": nil},
		{"deleted_at": nil},
	}

	deletedAt := columnByName(t, documents, "deleted_at")

	assert.Equal(t, "null", deletedAt.DataType)
	assert.True(t, deletedAt.Nullable)
}

func TestInferColumns_ConflictingTypesAreMixed(t *testing.T) {
	documents := []map[string]any{
		{"quantity": int64(3)},
		{"quantity": "three"},
	}

	quantity := columnByName(t, documents, "quantity")

	assert.Equal(t, "mixed", quantity.DataType)
	assert.False(t, quantity.Nullable, "the field was present and set in every document")
}

func TestInferColumns_NestedMapIsOneColumn(t *testing.T) {
	documents := []map[string]any{
		{"address": map[string]any{"city": "Amsterdam", "zip": "1011"}},
	}

	columns := inferColumns(documents)

	require.Len(t, columns, 1)
	assert.Equal(t, "address", columns[0].Name)
	assert.Equal(t, "map", columns[0].DataType)
}

func TestInferColumns_ArrayIsOneColumn(t *testing.T) {
	documents := []map[string]any{
		{"tags": []any{"new", "priority"}},
	}

	tags := columnByName(t, documents, "tags")

	assert.Equal(t, "array", tags.DataType)
}

func TestInferColumns_ReferenceIsItsOwnType(t *testing.T) {
	documents := []map[string]any{
		{"customer": (*firestore.DocumentRef)(nil)},
	}

	customer := columnByName(t, documents, "customer")

	assert.Equal(t, "reference", customer.DataType)
}

func TestFirestoreValueType_MapsTheGoTypesTheClientReturns(t *testing.T) {
	assert.Equal(t, "null", firestoreValueType(nil))
	assert.Equal(t, "boolean", firestoreValueType(true))
	assert.Equal(t, "integer", firestoreValueType(int64(7)))
	assert.Equal(t, "double", firestoreValueType(1.5))
	assert.Equal(t, "string", firestoreValueType("hello"))
	assert.Equal(t, "bytes", firestoreValueType([]byte{1, 2}))
	assert.Equal(t, "timestamp", firestoreValueType(time.Now()))
	assert.Equal(t, "geopoint", firestoreValueType(&latlng.LatLng{Latitude: 52.4, Longitude: 4.9}))
	assert.Equal(t, "array", firestoreValueType([]any{1}))
	assert.Equal(t, "map", firestoreValueType(map[string]any{"a": 1}))
	assert.Equal(t, "vector", firestoreValueType(firestore.Vector64{0.1, 0.2}))
}

// Firestore adds value types over time, so anything unrecognised is labelled
// rather than dropped, which would silently lose the column.
func TestFirestoreValueType_UnknownGoTypeIsLabelledUnknown(t *testing.T) {
	assert.Equal(t, "unknown", firestoreValueType(struct{ A int }{A: 1}))
}

func TestColumnDataType_NoTypesAtAllIsNull(t *testing.T) {
	assert.Equal(t, "null", columnDataType(map[string]struct{}{}))
}

func TestColumnDataType_TwoRealTypesAreMixed(t *testing.T) {
	types := map[string]struct{}{"string": {}, "integer": {}, "null": {}}

	assert.Equal(t, "mixed", columnDataType(types))
}
