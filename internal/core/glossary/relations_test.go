package glossary

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type quietRecorder struct{}

func (quietRecorder) RecordSearchQuery(context.Context, string, string)               {}
func (quietRecorder) RecordAssetView(context.Context, string, string, string, string) {}
func (quietRecorder) RecordDBQuery(context.Context, string, time.Duration, bool)      {}
func (quietRecorder) WrapDBQuery(_ context.Context, _ string, fn func() error) error  { return fn() }
func (quietRecorder) RecordCustomMetrics(context.Context, []metrics.Metric) error     { return nil }

func TestSuggestionWordsDropsShortAndStopWordsAndPlurals(t *testing.T) {
	assert.Equal(t, []string{"gross", "merchandise", "value"}, suggestionWords("Gross merchandise value"))
	assert.Equal(t, []string{"refund", "rate"}, suggestionWords("Refunds rate"))
	assert.Equal(t, []string{"active", "customer", "month"}, suggestionWords("Active customers (of the month)"))
	assert.Empty(t, suggestionWords("ID"))
}

func TestRelatedTermsLinkBothWaysAndCountLinkedAssets(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	svc := NewService(NewPostgresRepository(db, quietRecorder{}))

	var userID string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('sam', 'Sam Jones') RETURNING id::text`).Scan(&userID))
	owners := []OwnerInput{{ID: userID, Type: "user"}}

	gmv, err := svc.Create(ctx, CreateTermInput{Name: "Gross merchandise value", Definition: "Orders by value.", Owners: owners, CreatedBy: &userID})
	require.NoError(t, err)
	net, err := svc.Create(ctx, CreateTermInput{Name: "Net revenue", Definition: "After refunds.", Owners: owners, RelatedTermIDs: []string{gmv.ID}})
	require.NoError(t, err)

	require.NotNil(t, gmv.CreatedByName)
	assert.Equal(t, "Sam Jones", *gmv.CreatedByName)
	assert.Nil(t, net.CreatedBy)

	got, err := svc.Get(ctx, gmv.ID)
	require.NoError(t, err)
	require.Len(t, got.RelatedTerms, 1)
	assert.Equal(t, net.ID, got.RelatedTerms[0].ID)

	_, err = svc.Update(ctx, net.ID, UpdateTermInput{RelatedTermIDs: []string{}})
	require.NoError(t, err)
	got, err = svc.Get(ctx, gmv.ID)
	require.NoError(t, err)
	assert.Empty(t, got.RelatedTerms)

	_, err = svc.AddRelatedTerm(ctx, gmv.ID, gmv.ID)
	assert.ErrorIs(t, err, ErrInvalidInput)

	related, err := svc.AddRelatedTerm(ctx, gmv.ID, net.ID)
	require.NoError(t, err)
	assert.Len(t, related, 1)
	related, err = svc.RemoveRelatedTerm(ctx, net.ID, gmv.ID)
	require.NoError(t, err)
	assert.Empty(t, related)

	_, err = db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by, tags, description)
		VALUES ('a1', 'orders_daily', 'mrn://table/postgresql/orders_daily', 'table', '{postgresql}', 'test', '{commerce}', 'Daily orders'),
		       ('a2', 'refunds', 'mrn://table/postgresql/refunds', 'table', '{postgresql}', 'test', '{}', 'Merchandise returned'),
		       ('a3', 'sessions', 'mrn://table/postgresql/sessions', 'table', '{postgresql}', 'test', '{}', 'Web visits')`)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO asset_terms (asset_id, glossary_term_id, source) VALUES ('a1', $1, 'user')`, gmv.ID)
	require.NoError(t, err)

	list, err := svc.List(ctx, 0, 10)
	require.NoError(t, err)
	counts := map[string]int{}
	for _, term := range list.Terms {
		counts[term.Name] = term.LinkedAssetCount
	}
	assert.Equal(t, 1, counts["Gross merchandise value"])
	assert.Equal(t, 0, counts["Net revenue"])

	suggested, err := svc.SuggestAssets(ctx, gmv.ID, 10)
	require.NoError(t, err)
	names := []string{}
	for _, a := range suggested {
		names = append(names, a.Name)
	}
	assert.Equal(t, []string{"refunds"}, names, "the linked asset and the unrelated one are left out")
}
