package knowledge

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func seed(t *testing.T) (Service, context.Context) {
	t.Helper()
	db := pgtest.TempDB(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by, user_description)
		VALUES ('p1', 'Retention policy', 'mrn://asset/policy/marmot/retention-policy', 'Policy', '{Marmot}', 'test', 'Keep orders for seven years'),
		       ('d1', 'Use Postgres', 'mrn://asset/decision/marmot/use-postgres', 'Decision', '{Marmot}', 'test', NULL),
		       ('d2', 'Use Postgres 17', 'mrn://asset/decision/marmot/use-postgres-17', 'decision', '{Marmot}', 'test', NULL),
		       ('d3', 'Use MySQL', 'mrn://asset/decision/marmot/use-mysql', 'Decision', '{Marmot}', 'test', NULL),
		       ('t1', 'orders', 'mrn://asset/table/postgresql/orders', 'table', '{postgresql}', 'test', NULL)`)
	require.NoError(t, err)
	return NewService(NewPostgresRepository(db)), ctx
}

func TestAnAssetOfAKnowledgeTypeIsAnEntryBeforeAnythingIsSaved(t *testing.T) {
	svc, ctx := seed(t)

	e, err := svc.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, "decision", e.Kind)
	assert.Empty(t, e.Status)
	assert.Empty(t, e.Fields)

	_, err = svc.Get(ctx, "t1")
	assert.ErrorIs(t, err, ErrNotFound, "a table is not a knowledge entry")
}

func TestSavingAnEntryCleansItsFieldsAndLinksTheAssetsItAppliesTo(t *testing.T) {
	svc, ctx := seed(t)
	due := "2000-01-01"

	e, err := svc.Save(ctx, "p1", SaveInput{
		Status: "active",
		Fields: map[string]any{
			"audience": "  All staff ",
			"approver": "",
		},
		ReviewBy:  &due,
		AppliesTo: []string{"t1", "t1", "p1"},
	}, "Alice")
	require.NoError(t, err)

	assert.Equal(t, StateCurrent, e.State)
	assert.Equal(t, map[string]any{"audience": "All staff"}, e.Fields)
	assert.True(t, e.ReviewDue)
	require.Len(t, e.AppliesTo, 1, "duplicates and the entry itself are dropped")
	assert.Equal(t, "orders", e.AppliesTo[0].Name)
	assert.Equal(t, "Alice", e.UpdatedBy)

	applying, err := svc.ApplyingTo(ctx, "t1")
	require.NoError(t, err)
	require.Len(t, applying, 1)
	assert.Equal(t, "Retention policy", applying[0].Name)
	assert.Equal(t, "policy", applying[0].Kind)

	counts, err := svc.Counts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, counts.Total)
	assert.Equal(t, 1, counts.ReviewDue)
	assert.Equal(t, 3, counts.ByKind["decision"], "type matching ignores case")
	assert.Equal(t, 1, counts.ByState["current"])
	assert.Equal(t, 3, counts.Unset)
}

func TestSavingRejectsWhatTheKindDoesNotAllow(t *testing.T) {
	svc, ctx := seed(t)
	self := "d1"
	bad := "next week"

	policy := "p1"
	cases := map[string]SaveInput{
		"unknown status":        {Status: "active"},
		"date once accepted":    {Status: "accepted", Fields: map[string]any{}},
		"unknown field":         {Status: "proposed", Fields: map[string]any{"colour": "red"}},
		"bad date":              {Status: "proposed", Fields: map[string]any{"decided_at": "yesterday"}},
		"bad list":              {Status: "proposed", Fields: map[string]any{"deciders": "Alice"}},
		"replaces itself":       {Status: "proposed", Supersedes: &self},
		"replaces another kind": {Status: "proposed", Supersedes: &policy},
		"unknown asset":         {Status: "proposed", AppliesTo: []string{"nope"}},
		"bad effective date":    {Status: "proposed", EffectiveFrom: &bad},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Save(ctx, "d1", in, "Alice")
			assert.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func TestACurrentReplacementMarksTheOldEntrySuperseded(t *testing.T) {
	svc, ctx := seed(t)
	old := "d1"
	accepted := map[string]any{"decided_at": "2026-01-01"}

	_, err := svc.Save(ctx, "d2", SaveInput{Status: "proposed", Supersedes: &old}, "Alice")
	require.NoError(t, err)
	still, err := svc.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Empty(t, still.Status, "a proposal does not replace anything yet")
	require.Len(t, still.SupersededBy, 1)

	_, err = svc.Save(ctx, "d2", SaveInput{Status: "accepted", Fields: accepted, Supersedes: &old}, "Alice")
	require.NoError(t, err)
	replaced, err := svc.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, "superseded", replaced.Status)
	assert.Equal(t, StateHistorical, replaced.State)
	require.Len(t, replaced.SupersededBy, 1)
	assert.Equal(t, "Use Postgres 17", replaced.SupersededBy[0].Name)

	newer, err := svc.Get(ctx, "d2")
	require.NoError(t, err)
	require.NotNil(t, newer.Supersedes)
	assert.Equal(t, "superseded", newer.Supersedes.Status)
}

func TestAnEntryNotYetInForceIsPending(t *testing.T) {
	svc, ctx := seed(t)
	later := "2999-01-01"

	e, err := svc.Save(ctx, "p1", SaveInput{Status: "active", EffectiveFrom: &later}, "Alice")
	require.NoError(t, err)
	assert.Equal(t, StatePending, e.State)
}

func TestMarkingReviewedRecordsWhoAndMovesTheNextReview(t *testing.T) {
	svc, ctx := seed(t)
	past, next := "2000-01-01", "2999-06-01"

	_, err := svc.MarkReviewed(ctx, "p1", nil, "Alice")
	assert.ErrorIs(t, err, ErrInvalid, "an entry never saved has nothing to vouch for")

	_, err = svc.Save(ctx, "p1", SaveInput{Status: "active", ReviewBy: &past}, "Alice")
	require.NoError(t, err)
	_, err = svc.MarkReviewed(ctx, "p1", &past, "Bob")
	assert.ErrorIs(t, err, ErrInvalid, "the next review is in the future")

	e, err := svc.MarkReviewed(ctx, "p1", &next, "Bob")
	require.NoError(t, err)
	assert.Equal(t, "Bob", e.LastReviewedBy)
	assert.NotNil(t, e.LastReviewedAt)
	assert.Equal(t, next, *e.ReviewBy)
	assert.False(t, e.ReviewDue)
}

func TestListingFindsEntriesByKindStatusAndText(t *testing.T) {
	svc, ctx := seed(t)
	_, err := svc.Save(ctx, "p1", SaveInput{Status: "active", Fields: map[string]any{"audience": "Finance team"}}, "Alice")
	require.NoError(t, err)

	all, total, err := svc.List(ctx, ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Len(t, all, 4)

	byText, _, err := svc.List(ctx, ListFilter{Query: "finance"})
	require.NoError(t, err)
	require.Len(t, byText, 1, "typed fields are searched")
	assert.Equal(t, "p1", byText[0].AssetID)
	assert.Equal(t, "Keep orders for seven years", byText[0].Summary)

	byKind, _, err := svc.List(ctx, ListFilter{Kinds: []string{"decision"}})
	require.NoError(t, err)
	assert.Len(t, byKind, 3)

	byStatus, _, err := svc.List(ctx, ListFilter{Statuses: []string{"active"}})
	require.NoError(t, err)
	assert.Len(t, byStatus, 1)
}

func TestEveryKindIsWellFormed(t *testing.T) {
	ids := map[string]bool{}
	for _, k := range Kinds() {
		assert.False(t, ids[k.ID], "kind ids are unique")
		ids[k.ID] = true
		_, ok := k.status(k.DefaultStatus)
		assert.True(t, ok, "%s's default status is one of its statuses", k.ID)
		states := map[State]bool{}
		for _, s := range k.Statuses {
			states[s.State] = true
		}
		assert.True(t, states[StatePending] && states[StateCurrent] && states[StateHistorical], "%s has a pending, a current and a historical status", k.ID)
		for _, f := range k.Fields {
			if f.Type == FieldTable {
				assert.NotEmpty(t, f.Columns, "%s.%s has columns", k.ID, f.Key)
				for _, c := range f.Columns {
					assert.Contains(t, []FieldType{FieldText, FieldLongText, FieldSelect, FieldURL}, c.Type, "%s.%s.%s is a cell type", k.ID, f.Key, c.Key)
				}
			}
			if f.Type == FieldSelect {
				assert.NotEmpty(t, f.Options, "%s.%s has options", k.ID, f.Key)
			}
		}
	}
}

func TestKindStatusesTheListSinksAreAllHistorical(t *testing.T) {
	for _, k := range Kinds() {
		for _, st := range k.Statuses {
			sunk := strings.Contains(historicalLast, "'"+st.Value+"'")
			assert.Equal(t, st.State == StateHistorical, sunk, "%s status %s", k.ID, st.Value)
		}
	}
}
