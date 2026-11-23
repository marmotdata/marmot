package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokeniseKeepsInWordApostrophes(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"don't", []string{"don't"}},
		{"plugin's output", []string{"plugin's", "output"}},
		{"it's don't", []string{"it's", "don't"}},
		{`@name: 'two words'`, []string{"@name", ":", "'two words'"}},
	}
	for _, tc := range tests {
		got, err := NewTokeniser().Tokenise(tc.in)
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestTokeniseStillRejectsUnclosedQuotes(t *testing.T) {
	for _, in := range []string{"'abc", "abc'", `"abc`, "don't 'abc"} {
		_, err := NewTokeniser().Tokenise(in)
		assert.Error(t, err, in)
	}
}

func TestParseFreeTextWithApostrophe(t *testing.T) {
	q, err := NewParser().Parse("don't")
	require.NoError(t, err)
	assert.Equal(t, "don't", q.FreeText)
}
