package asset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestSearchSurvivesTsqueryCharacters(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db, quietRecorder{})

	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, description, created_by)
		VALUES ('a1', 'notes', 'mrn://asset/table/pg/notes', 'table', '{postgresql}', 'Rows that we don''t delete and the plugin''s output', 'test'),
		       ('a2', 'other', 'mrn://asset/table/pg/other', 'table', '{postgresql}', 'Nothing relevant', 'test')`)
	require.NoError(t, err)

	for _, q := range []string{"don't", "plugin's", "don't delete", "it's don't", "a:b", "a & b", "a | b", "!a", "(a)", `a\b`, "a:*", "'", "''", "don't'"} {
		_, _, _, err := repo.Search(ctx, SearchFilter{Query: q, Limit: 10}, true)
		if err != nil {
			assert.ErrorIs(t, err, ErrInvalidQuery, q)
			assert.NotContains(t, err.Error(), "syntax error", q)
		}
	}

	// "don't" alone is all stop words to the english configuration, so it parses and matches nothing; with a real word beside it the row is found.
	for _, q := range []string{"plugin's", "don't delete"} {
		got, total, _, err := repo.Search(ctx, SearchFilter{Query: q, Limit: 10}, false)
		require.NoError(t, err, q)
		require.Equal(t, 1, total, q)
		assert.Equal(t, "a1", got[0].ID, q)
	}
}

func TestSearchMatchesTheUserDescription(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db, quietRecorder{})

	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, description, user_description, created_by)
		VALUES ('a1', 'orders', 'mrn://asset/table/pg/orders', 'table', '{postgresql}', 'Raw rows', 'Quarterly settlement ledger', 'test'),
		       ('a2', 'refunds', 'mrn://asset/table/pg/refunds', 'table', '{postgresql}', 'Raw rows', NULL, 'test')`)
	require.NoError(t, err)

	got, total, _, err := repo.Search(ctx, SearchFilter{Query: "settlement", Limit: 10}, false)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, "a1", got[0].ID)

	_, err = db.Exec(ctx, `UPDATE assets SET user_description = 'Chargeback register' WHERE id = 'a2'`)
	require.NoError(t, err)

	var n int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM search_index WHERE type = 'asset' AND search_text @@ websearch_to_tsquery('english', 'chargeback')`).Scan(&n))
	assert.Equal(t, 1, n)
}
