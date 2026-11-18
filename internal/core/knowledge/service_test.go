package knowledge

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func seed(t *testing.T) (Service, context.Context, *pgxpool.Pool) {
	t.Helper()
	db := pgtest.TempDB(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, tags, created_by, user_description)
		VALUES ('p1', 'Retention policy', 'mrn://asset/policy/marmot/retention-policy', 'Policy', '{Marmot}', '{}', 'test', 'Keep orders for seven years'),
		       ('p2', 'PII handling', 'mrn://asset/policy/marmot/pii-handling', 'Policy', '{Marmot}', '{}', 'test', NULL),
		       ('d1', 'Use Postgres', 'mrn://asset/decision/marmot/use-postgres', 'Decision', '{Marmot}', '{}', 'test', NULL),
		       ('d2', 'Use Postgres 17', 'mrn://asset/decision/marmot/use-postgres-17', 'decision', '{Marmot}', '{}', 'test', NULL),
		       ('d3', 'Use MySQL', 'mrn://asset/decision/marmot/use-mysql', 'Decision', '{Marmot}', '{}', 'test', NULL),
		       ('db1', 'shop', 'mrn://asset/database/postgresql/shop', 'database', '{PostgreSQL}', '{}', 'test', NULL),
		       ('t1', 'orders', 'mrn://asset/table/postgresql/shop.orders', 'table', '{PostgreSQL}', '{pii}', 'test', NULL)`)
	require.NoError(t, err)
	// shop contains orders, the way database plugins describe containment.
	_, err = db.Exec(ctx, `WITH ev AS (INSERT INTO lineage_events (event_time, event_type, event_data) VALUES (NOW(), 'test', '{}') RETURNING event_id)
		INSERT INTO lineage_edges (source_mrn, target_mrn, event_id, type) SELECT 'mrn://asset/database/postgresql/shop', 'mrn://asset/table/postgresql/shop.orders', event_id, 'CONTAINS' FROM ev`)
	require.NoError(t, err)
	return NewService(NewPostgresRepository(db)), ctx, db
}

func links(ids ...string) *[]LinkInput {
	out := make([]LinkInput, len(ids))
	for i, id := range ids {
		out[i] = LinkInput{TargetID: id}
	}
	return &out
}

func names(items []AboutItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func TestAnAssetOfAKnowledgeTypeIsAnEntryBeforeAnythingIsSaved(t *testing.T) {
	svc, ctx, _ := seed(t)

	e, err := svc.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Equal(t, "decision", e.Kind)
	assert.Empty(t, e.Status)
	assert.Empty(t, e.Fields)
	assert.Empty(t, e.Links)

	_, err = svc.Get(ctx, "t1")
	assert.ErrorIs(t, err, ErrNotFound, "a table is not a knowledge entry")
}

func TestSavingAnEntryCleansItsFieldsAndLinksIt(t *testing.T) {
	svc, ctx, _ := seed(t)
	due := "2000-01-01"

	e, err := svc.Save(ctx, "p1", SaveInput{
		Status:   "active",
		Fields:   map[string]any{"audience": "  All staff ", "approver": ""},
		ReviewBy: &due,
		Links:    &[]LinkInput{{TargetID: "t1"}, {TargetID: "t1"}, {TargetID: "p1"}, {Relation: "documents", TargetID: "t1"}},
	}, "Alice")
	require.NoError(t, err)

	assert.Equal(t, StateCurrent, e.State)
	assert.Equal(t, map[string]any{"audience": "All staff"}, e.Fields)
	assert.True(t, e.ReviewDue)
	assert.Equal(t, 2, e.LinkCount, "duplicates and the entry itself are dropped, and each relation is its own link")
	require.Len(t, e.Links, 2)
	assert.Equal(t, "orders", e.Links[0].Target.Name)
	assert.Equal(t, "Alice", e.UpdatedBy)

	counts, err := svc.Counts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, counts.Total)
	assert.Equal(t, 1, counts.ReviewDue)
	assert.Equal(t, 3, counts.ByKind["decision"], "type matching ignores case")
	assert.Equal(t, 1, counts.ByState["current"])
	assert.Equal(t, 4, counts.Unset)
}

func TestSavingWithoutLinksOrRulesKeepsThem(t *testing.T) {
	svc, ctx, _ := seed(t)
	_, err := svc.Save(ctx, "p1", SaveInput{Status: "active", Links: links("t1"),
		Rules: &[]RuleInput{{SelectorType: SelectorTag, SelectorValue: "pii"}}}, "Alice")
	require.NoError(t, err)

	e, err := svc.Save(ctx, "p1", SaveInput{Status: "retired"}, "Bob")
	require.NoError(t, err)
	assert.Equal(t, 1, e.LinkCount)
	assert.Len(t, e.Rules, 1)

	e, err = svc.Save(ctx, "p1", SaveInput{Status: "retired", Links: &[]LinkInput{}, Rules: &[]RuleInput{}}, "Bob")
	require.NoError(t, err)
	assert.Equal(t, 0, e.LinkCount, "an empty list clears them")
	assert.Empty(t, e.Rules)
}

func TestSavingRejectsWhatTheKindDoesNotAllow(t *testing.T) {
	svc, ctx, _ := seed(t)
	self := "d1"
	policy := "p1"
	bad := "next week"

	cases := map[string]SaveInput{
		"unknown status":        {Status: "active"},
		"date once accepted":    {Status: "accepted", Fields: map[string]any{}},
		"unknown field":         {Status: "proposed", Fields: map[string]any{"colour": "red"}},
		"bad date":              {Status: "proposed", Fields: map[string]any{"decided_at": "yesterday"}},
		"bad list":              {Status: "proposed", Fields: map[string]any{"deciders": "Alice"}},
		"replaces itself":       {Status: "proposed", Supersedes: &self},
		"replaces another kind": {Status: "proposed", Supersedes: &policy},
		"unknown asset":         {Status: "proposed", Links: links("nope")},
		"unknown relation":      {Status: "proposed", Links: &[]LinkInput{{Relation: "owns", TargetID: "t1"}}},
		"unknown selector":      {Status: "proposed", Rules: &[]RuleInput{{SelectorType: "domain", SelectorValue: "x"}}},
		"empty selector":        {Status: "proposed", Rules: &[]RuleInput{{SelectorType: SelectorTag, SelectorValue: " "}}},
		"bad effective date":    {Status: "proposed", EffectiveFrom: &bad},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Save(ctx, "d1", in, "Alice")
			assert.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func TestKnowledgeAboutAnAssetComesFromLinksRulesAndWhatContainsIt(t *testing.T) {
	svc, ctx, _ := seed(t)
	accepted := map[string]any{"decided_at": "2026-01-01"}

	_, err := svc.Save(ctx, "d1", SaveInput{Status: "accepted", Fields: accepted, Links: links("t1")}, "Alice")
	require.NoError(t, err)
	_, err = svc.Save(ctx, "p2", SaveInput{Status: "active", Rules: &[]RuleInput{{SelectorType: SelectorTag, SelectorValue: "pii"}}}, "Alice")
	require.NoError(t, err)
	_, err = svc.Save(ctx, "p1", SaveInput{Status: "active", Links: links("db1")}, "Alice")
	require.NoError(t, err)
	_, err = svc.Save(ctx, "d3", SaveInput{Status: "rejected", Fields: accepted, Rules: &[]RuleInput{{SelectorType: SelectorType, SelectorValue: "Table"}}}, "Alice")
	require.NoError(t, err)

	page, err := svc.About(ctx, "t1", AboutFilter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"PII handling", "Retention policy", "Use Postgres", "Use MySQL"}, names(page.Entries), "in use first by name, then no longer in use")
	assert.Equal(t, 4, page.Total)
	assert.Equal(t, map[string]int{SourceDirect: 1, SourceRule: 2, SourceInherited: 1}, page.BySource)
	assert.Equal(t, map[string]int{"policy": 2, "decision": 2}, page.ByKind)

	byName := map[string]AboutItem{}
	for _, it := range page.Entries {
		byName[it.Name] = it
	}
	assert.Equal(t, SourceInherited, byName["Retention policy"].Source)
	require.NotNil(t, byName["Retention policy"].Via)
	assert.Equal(t, "shop", byName["Retention policy"].Via.Name, "an inherited entry says where it came from")
	assert.Equal(t, SourceRule, byName["PII handling"].Source)
	require.NotNil(t, byName["PII handling"].Rule)
	assert.Equal(t, "pii", byName["PII handling"].Rule.SelectorValue)

	current, err := svc.About(ctx, "t1", AboutFilter{States: []State{StateCurrent}, Kinds: []string{"policy"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"PII handling", "Retention policy"}, names(current.Entries))

	inherited, err := svc.About(ctx, "t1", AboutFilter{Sources: []string{SourceInherited}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Retention policy"}, names(inherited.Entries))
	assert.Equal(t, 1, inherited.Total)

	p2, err := svc.Get(ctx, "p2")
	require.NoError(t, err)
	require.Len(t, p2.Rules, 1)
	assert.Equal(t, 1, p2.Rules[0].Matches, "a rule says how many assets it covers")
}

func TestKnowledgeAboutAnAssetPagesInOrderWithoutRepeats(t *testing.T) {
	svc, ctx, db := seed(t)
	for i := range 23 {
		id := fmt.Sprintf("x%02d", i)
		_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES ($1, $2, $3, 'Decision', '{Marmot}', 'test')`,
			id, fmt.Sprintf("Decision %02d", i), "mrn://asset/decision/marmot/"+id)
		require.NoError(t, err)
		status := "accepted"
		if i%4 == 0 {
			status = "superseded"
		}
		_, err = svc.Save(ctx, id, SaveInput{Status: status, Fields: map[string]any{"decided_at": "2026-01-01"}, Links: links("t1")}, "Alice")
		require.NoError(t, err)
	}

	var seen []AboutItem
	cursor := ""
	for range 10 {
		page, err := svc.About(ctx, "t1", AboutFilter{Limit: 10, Cursor: cursor})
		require.NoError(t, err)
		assert.Equal(t, 23, page.Total)
		seen = append(seen, page.Entries...)
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	require.Len(t, seen, 23)
	unique := map[string]bool{}
	for i, it := range seen {
		unique[it.AssetID] = true
		if i > 0 && seen[i-1].State == StateHistorical {
			assert.Equal(t, StateHistorical, it.State, "nothing in use comes after something no longer in use")
		}
	}
	assert.Len(t, unique, 23)

	_, err := svc.About(ctx, "t1", AboutFilter{Cursor: "not a cursor"})
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestAnEntrysLinksPageByName(t *testing.T) {
	svc, ctx, db := seed(t)
	var ids []string
	for i := range 7 {
		id := fmt.Sprintf("tbl%d", i)
		_, err := db.Exec(ctx, `INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES ($1, $2, $3, 'table', '{PostgreSQL}', 'test')`,
			id, fmt.Sprintf("table_%d", i), "mrn://asset/table/postgresql/"+id)
		require.NoError(t, err)
		ids = append(ids, id)
	}
	_, err := svc.Save(ctx, "p1", SaveInput{Status: "active", Links: links(ids...)}, "Alice")
	require.NoError(t, err)

	first, err := svc.Links(ctx, "p1", "", "", 5)
	require.NoError(t, err)
	assert.Equal(t, 7, first.Total)
	require.Len(t, first.Links, 5)
	assert.Equal(t, "table_0", first.Links[0].Target.Name)
	require.NotEmpty(t, first.Next)

	second, err := svc.Links(ctx, "p1", "", first.Next, 5)
	require.NoError(t, err)
	require.Len(t, second.Links, 2)
	assert.Equal(t, "table_6", second.Links[1].Target.Name)
	assert.Empty(t, second.Next)
}

func TestStatusChangesAndRenamesReorderTheEntriesAboutAnAsset(t *testing.T) {
	svc, ctx, db := seed(t)
	accepted := map[string]any{"decided_at": "2026-01-01"}
	_, err := svc.Save(ctx, "d1", SaveInput{Status: "accepted", Fields: accepted, Links: links("t1")}, "Alice")
	require.NoError(t, err)
	_, err = svc.Save(ctx, "d3", SaveInput{Status: "accepted", Fields: accepted, Links: links("t1")}, "Alice")
	require.NoError(t, err)

	page, err := svc.About(ctx, "t1", AboutFilter{Kinds: []string{"decision"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Use MySQL", "Use Postgres"}, names(page.Entries))

	_, err = db.Exec(ctx, `UPDATE assets SET name = 'Adopt Postgres' WHERE id = 'd1'`)
	require.NoError(t, err)
	page, err = svc.About(ctx, "t1", AboutFilter{Kinds: []string{"decision"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Adopt Postgres", "Use MySQL"}, names(page.Entries))

	_, err = svc.Save(ctx, "d1", SaveInput{Status: "deprecated", Fields: accepted}, "Alice")
	require.NoError(t, err)
	page, err = svc.About(ctx, "t1", AboutFilter{Kinds: []string{"decision"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Use MySQL", "Adopt Postgres"}, names(page.Entries))
}

func TestACurrentReplacementMarksTheOldEntrySuperseded(t *testing.T) {
	svc, ctx, _ := seed(t)
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
	svc, ctx, _ := seed(t)
	later := "2999-01-01"

	e, err := svc.Save(ctx, "p1", SaveInput{Status: "active", EffectiveFrom: &later}, "Alice")
	require.NoError(t, err)
	assert.Equal(t, StatePending, e.State)
}

func TestMarkingReviewedRecordsWhoAndMovesTheNextReview(t *testing.T) {
	svc, ctx, _ := seed(t)
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

func TestListingFindsEntriesByKindStatusTeamAndText(t *testing.T) {
	svc, ctx, db := seed(t)
	_, err := svc.Save(ctx, "p1", SaveInput{Status: "active", Fields: map[string]any{"audience": "Finance team"}}, "Alice")
	require.NoError(t, err)

	all, total, err := svc.List(ctx, ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, all, 5)

	for _, q := range []string{"finance", "fin", "reten pol", "seven years"} {
		found, _, err := svc.List(ctx, ListFilter{Query: q})
		require.NoError(t, err)
		require.Len(t, found, 1, "%q matches typed fields, the summary and word prefixes", q)
		assert.Equal(t, "p1", found[0].AssetID)
	}

	_, err = db.Exec(ctx, `INSERT INTO doc_pages (entity_type, entity_id, title, content) VALUES ('asset', 'mrn://asset/policy/marmot/retention-policy', 'Exceptions', 'Ask the archivist')`)
	require.NoError(t, err)
	byPage, _, err := svc.List(ctx, ListFilter{Query: "archivist"})
	require.NoError(t, err)
	require.Len(t, byPage, 1, "write-up pages are searched")

	unsaved, _, err := svc.List(ctx, ListFilter{Query: "mysql"})
	require.NoError(t, err)
	require.Len(t, unsaved, 1, "an entry never saved is still found by its title")

	byKind, _, err := svc.List(ctx, ListFilter{Kinds: []string{"decision"}})
	require.NoError(t, err)
	assert.Len(t, byKind, 3)

	byStatus, _, err := svc.List(ctx, ListFilter{Statuses: []string{"active"}})
	require.NoError(t, err)
	assert.Len(t, byStatus, 1)

	var team string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO teams (name) VALUES ('Finance') RETURNING id::text`).Scan(&team))
	_, err = db.Exec(ctx, `INSERT INTO asset_owners (asset_id, team_id) VALUES ('p1', $1::uuid)`, team)
	require.NoError(t, err)
	byTeam, _, err := svc.List(ctx, ListFilter{TeamIDs: []string{team}})
	require.NoError(t, err)
	require.Len(t, byTeam, 1)
	counts, err := svc.Counts(ctx)
	require.NoError(t, err)
	assert.Equal(t, []TeamCount{{ID: team, Name: "Finance", Count: 1}}, counts.ByTeam)
}

func TestStateRanksAgreeWithTheDatabase(t *testing.T) {
	_, ctx, db := seed(t)
	for _, k := range Kinds() {
		for _, st := range k.Statuses {
			var rank int16
			require.NoError(t, db.QueryRow(ctx, `SELECT knowledge_state_rank($1)`, st.Value).Scan(&rank))
			assert.Equal(t, stateRanks[st.State], rank, "%s status %s", k.ID, st.Value)
			assert.Equal(t, rank, rankOf(st.Value))
		}
	}
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
