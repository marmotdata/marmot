package cassandra

import (
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/inf.v0"
)

func TestConvertValue_NilStaysNil(t *testing.T) {
	assert.Nil(t, convertValue(nil))
}

func TestConvertValue_UUIDBecomesString(t *testing.T) {
	id, err := gocql.ParseUUID("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)

	assert.Equal(t, "11111111-1111-1111-1111-111111111111", convertValue(id))
}

func TestConvertValue_TimestampBecomesRFC3339InUTC(t *testing.T) {
	loc := time.FixedZone("plus2", 2*3600)
	ts := time.Date(2026, 3, 4, 7, 6, 7, 0, loc)

	assert.Equal(t, "2026-03-04T05:06:07Z", convertValue(ts))
}

func TestConvertValue_TimeOfDayBecomesClock(t *testing.T) {
	// The CQL time type arrives as nanoseconds since midnight.
	d := time.Hour + 2*time.Minute + 3*time.Second + 4

	assert.Equal(t, "01:02:03.000000004", convertValue(d))
}

func TestConvertValue_DurationBecomesReadable(t *testing.T) {
	d := gocql.Duration{Months: 1, Days: 2, Nanoseconds: int64(3 * time.Hour)}

	assert.Equal(t, "1mo2d3h0m0s", convertValue(d))
}

func TestConvertValue_InetBecomesString(t *testing.T) {
	assert.Equal(t, "10.0.0.1", convertValue(net.ParseIP("10.0.0.1")))
}

func TestConvertValue_BigNumbersBecomeStrings(t *testing.T) {
	varint, ok := new(big.Int).SetString("123456789012345678901234567890", 10)
	require.True(t, ok)

	assert.Equal(t, "123456789012345678901234567890", convertValue(varint))
	assert.Equal(t, "42.50", convertValue(inf.NewDec(4250, 2)))
}

func TestConvertValue_NilBigNumbersStayNil(t *testing.T) {
	assert.Nil(t, convertValue((*big.Int)(nil)))
	assert.Nil(t, convertValue((*inf.Dec)(nil)))
}

func TestConvertValue_BlobBecomesHex(t *testing.T) {
	assert.Equal(t, "0xdeadbeef", convertValue([]byte{0xde, 0xad, 0xbe, 0xef}))
}

func TestConvertValue_ScalarsPassThrough(t *testing.T) {
	assert.Equal(t, "text", convertValue("text"))
	assert.Equal(t, true, convertValue(true))
	assert.Equal(t, int8(7), convertValue(int8(7)))
	assert.Equal(t, int64(9), convertValue(int64(9)))
	assert.Equal(t, 1.5, convertValue(1.5))
}

func TestConvertValue_ListsConvertEachElement(t *testing.T) {
	id, err := gocql.ParseUUID("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)

	assert.Equal(t, []interface{}{"book", "pen"}, convertValue([]string{"book", "pen"}))
	assert.Equal(t, []interface{}{"11111111-1111-1111-1111-111111111111"}, convertValue([]gocql.UUID{id}))
}

func TestConvertValue_MapsGetStringKeys(t *testing.T) {
	id, err := gocql.ParseUUID("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"plan": "gold"}, convertValue(map[string]string{"plan": "gold"}))
	assert.Equal(t, map[string]interface{}{"11111111-1111-1111-1111-111111111111": 3},
		convertValue(map[gocql.UUID]int{id: 3}))
}

func TestConvertValue_UserTypeValuesAreConverted(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	udt := map[string]interface{}{"city": "London", "since": ts}

	assert.Equal(t, map[string]interface{}{"city": "London", "since": "2026-01-02T03:04:05Z"}, convertValue(udt))
}

func TestConvertValue_PointersAreDereferenced(t *testing.T) {
	value := "hello"

	assert.Equal(t, "hello", convertValue(&value))
	assert.Nil(t, convertValue((*string)(nil)))
}

func TestSampleTarget_PrefersMetadata(t *testing.T) {
	name := "other.name"
	a := &pluginsdk.Asset{Name: &name, Metadata: map[string]interface{}{"keyspace": "shop", "table_name": "orders"}}

	keyspace, table := sampleTarget(a)

	assert.Equal(t, "shop", keyspace)
	assert.Equal(t, "orders", table)
}

func TestSampleTarget_FallsBackToTheQualifiedName(t *testing.T) {
	name := "shop.orders"
	a := &pluginsdk.Asset{Name: &name}

	keyspace, table := sampleTarget(a)

	assert.Equal(t, "shop", keyspace)
	assert.Equal(t, "orders", table)
}

func TestSampleTarget_KeyspaceAssetHasNoTable(t *testing.T) {
	name := "shop"
	a := &pluginsdk.Asset{Name: &name, Metadata: map[string]interface{}{"keyspace": "shop"}}

	_, table := sampleTarget(a)

	assert.Empty(t, table)
}
