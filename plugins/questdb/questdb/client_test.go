package questdb

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQuoteIdent_WrapsAPlainName(t *testing.T) {
	assert.Equal(t, `"trades"`, quoteIdent("trades"))
}

func TestQuoteIdent_KeepsSpacesAndDotsInsideTheQuotes(t *testing.T) {
	assert.Equal(t, `"sys.text_import_log"`, quoteIdent("sys.text_import_log"))
	assert.Equal(t, `"weird name"`, quoteIdent("weird name"))
}

func TestQuoteIdent_DoublesEmbeddedQuotes(t *testing.T) {
	assert.Equal(t, `"say ""hi"""`, quoteIdent(`say "hi"`))
}

func TestQuoteLiteral_WrapsAPlainName(t *testing.T) {
	assert.Equal(t, `'trades'`, quoteLiteral("trades"))
}

func TestQuoteLiteral_DoublesEmbeddedQuotes(t *testing.T) {
	assert.Equal(t, `'o''brien'`, quoteLiteral("o'brien"))
}

func TestIsUnsupportedFunction_MatchesQuestDBsMessage(t *testing.T) {
	// The exact text QuestDB 10 returns for a table function it lacks.
	err := errors.New("ERROR: unknown function name: materialized_views() (SQLSTATE 00000)")

	assert.True(t, isUnsupportedFunction(err))
}

func TestIsUnsupportedFunction_MatchesThePostgresPhrasing(t *testing.T) {
	assert.True(t, isUnsupportedFunction(errors.New("function does not exist")))
}

func TestIsUnsupportedFunction_IgnoresOtherErrors(t *testing.T) {
	assert.False(t, isUnsupportedFunction(nil))
	assert.False(t, isUnsupportedFunction(errors.New("table does not exist [table=x]")))
	assert.False(t, isUnsupportedFunction(errors.New("connection refused")))
}

func TestRow_IntegerReadsEveryWidthPgxProduces(t *testing.T) {
	r := row{"a": int16(1), "b": int32(2), "c": int64(3), "d": 4, "e": float64(5)}

	assert.Equal(t, int64(1), r.integer("a"))
	assert.Equal(t, int64(2), r.integer("b"))
	assert.Equal(t, int64(3), r.integer("c"))
	assert.Equal(t, int64(4), r.integer("d"))
	assert.Equal(t, int64(5), r.integer("e"))
}

func TestRow_IntegerIsZeroForNullMissingOrText(t *testing.T) {
	r := row{"null": nil, "text": "7"}

	assert.Equal(t, int64(0), r.integer("null"))
	assert.Equal(t, int64(0), r.integer("missing"))
	assert.Equal(t, int64(0), r.integer("text"))
}

func TestRow_StrIsEmptyForNullOrMissing(t *testing.T) {
	r := row{"null": nil, "name": "trades"}

	assert.Equal(t, "", r.str("null"))
	assert.Equal(t, "", r.str("missing"))
	assert.Equal(t, "trades", r.str("name"))
}

func TestRow_StrRendersNonStrings(t *testing.T) {
	assert.Equal(t, "42", row{"n": int32(42)}.str("n"))
}

func TestRow_HasDistinguishesNullFromMissing(t *testing.T) {
	r := row{"null": nil}

	assert.True(t, r.has("null"))
	assert.False(t, r.has("missing"))
}

func TestRow_BooleanIsFalseUnlessTrue(t *testing.T) {
	r := row{"yes": true, "no": false, "null": nil, "text": "true"}

	assert.True(t, r.boolean("yes"))
	assert.False(t, r.boolean("no"))
	assert.False(t, r.boolean("null"))
	assert.False(t, r.boolean("text"))
	assert.False(t, r.boolean("missing"))
}

func TestRow_TimestampReportsWhetherItIsATime(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	r := row{"ts": now, "null": nil}

	got, ok := r.timestamp("ts")
	assert.True(t, ok)
	assert.Equal(t, now, got)

	_, ok = r.timestamp("null")
	assert.False(t, ok)
}
