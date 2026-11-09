package runs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// processAs is process for a run that belongs to owner, the way the API and the scheduler start runs.
func processAs(t *testing.T, service Service, pipeline, owner string, assets []CreateAssetInput, edges []LineageInput) *ProcessAssetsResponse {
	t.Helper()
	ctx := context.Background()
	run, err := service.StartRunAs(ctx, pipeline, "postgresql", "tester", owner, plugin.RawPluginConfig{})
	require.NoError(t, err)
	response, err := service.ProcessEntities(ctx, run.RunID, assets, edges, nil, nil, nil, pipeline, "postgresql")
	require.NoError(t, err)
	require.NoError(t, service.CompleteRun(ctx, run.RunID, plugin.StatusCompleted, &plugin.RunSummary{}, ""))
	return response
}

func qualified(name string, path []string, metadata map[string]interface{}) CreateAssetInput {
	m := mrn.Asset("Table", "PostgreSQL", path...).String()
	return CreateAssetInput{Name: name, MRN: &m, Type: "Table", Providers: []string{"PostgreSQL"}, Metadata: metadata}
}

func statusesOf(r *ProcessAssetsResponse) map[string]string {
	out := map[string]string{}
	for _, a := range r.Assets {
		out[a.MRN] = a.Status
	}
	return out
}

func TestSameNameInTwoSchemasIsTwoAssets(t *testing.T) {
	pool := pgtest.TempDB(t)
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	r := processAs(t, service, "shop", "user:alice", []CreateAssetInput{
		qualified("orders", []string{"shop", "public", "orders"}, nil),
		qualified("orders", []string{"shop", "sales", "orders"}, nil),
	}, nil)
	for _, a := range r.Assets {
		assert.Equal(t, StatusCreated, a.Status, a.Error)
	}
	found, err := assetSvc.GetByMRNs(context.Background(), []string{"mrn://asset/table/postgresql/shop.public.orders", "mrn://asset/table/postgresql/shop.sales.orders"})
	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestTwoAssetsWithOneMRNInARunFailInsteadOfMerging(t *testing.T) {
	pool := pgtest.TempDB(t)
	service, _ := newTestService(t, pool)

	r := processAs(t, service, "shop", "user:alice", []CreateAssetInput{tableInput("orders"), tableInput("orders")}, nil)
	require.Len(t, r.Assets, 2)
	assert.Equal(t, StatusCreated, r.Assets[0].Status)
	assert.Equal(t, StatusFailed, r.Assets[1].Status)
	assert.Contains(t, r.Assets[1].Error, "same MRN")
}

func TestALegacyAssetMovesToItsQualifiedMRNKeepingItsID(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	// What an older plugin produced: a bare name.
	first := processAs(t, service, "shop", "user:alice", []CreateAssetInput{tableInput("orders")}, nil)
	oldMRN := first.Assets[0].MRN
	before, err := assetSvc.GetByMRN(ctx, oldMRN)
	require.NoError(t, err)

	second := processAs(t, service, "shop", "user:alice", []CreateAssetInput{qualified("orders", []string{"shop", "public", "orders"}, nil)}, nil)
	require.Len(t, second.Assets, 1)
	assert.Contains(t, []string{StatusUpdated, StatusUnchanged}, second.Assets[0].Status, second.Assets[0].Error)
	assert.Equal(t, oldMRN, second.Assets[0].MovedFrom)
	assert.Empty(t, second.StaleEntitiesRemoved, "the move must not look like the old asset going stale")

	after, err := assetSvc.GetByMRN(ctx, "mrn://asset/table/postgresql/shop.public.orders")
	require.NoError(t, err)
	assert.Equal(t, before.ID, after.ID)

	viaLegacy, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/orders")
	require.NoError(t, err, "the old MRN keeps working as an alias")
	assert.Equal(t, before.ID, viaLegacy.ID)
}

func TestAnAssetSeveralNewOnesCouldBeGoesToTheBestMatch(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	merged := tableInput("orders")
	merged.Metadata = map[string]interface{}{"database": "shop2", "schema": "public"}
	processAs(t, service, "shop", "user:alice", []CreateAssetInput{merged}, nil)
	old, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/orders")
	require.NoError(t, err)

	r := processAs(t, service, "shop", "user:alice", []CreateAssetInput{
		qualified("orders", []string{"shop", "public", "orders"}, map[string]interface{}{"database": "shop", "schema": "public"}),
		qualified("orders", []string{"shop2", "public", "orders"}, map[string]interface{}{"database": "shop2", "schema": "public"}),
	}, nil)
	s := statusesOf(r)
	assert.Equal(t, StatusCreated, s["mrn://asset/table/postgresql/shop.public.orders"])
	assert.Contains(t, []string{StatusUpdated, StatusUnchanged}, s["mrn://asset/table/postgresql/shop2.public.orders"])

	heir, err := assetSvc.GetByMRN(ctx, "mrn://asset/table/postgresql/shop2.public.orders")
	require.NoError(t, err)
	assert.Equal(t, old.ID, heir.ID)
}

func TestATieLeavesTheOldAssetAloneAndWarns(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	processAs(t, service, "shop", "user:alice", []CreateAssetInput{tableInput("orders")}, nil)
	r := processAs(t, service, "shop", "user:alice", []CreateAssetInput{
		qualified("orders", []string{"a", "public", "orders"}, nil),
		qualified("orders", []string{"b", "public", "orders"}, nil),
	}, nil)
	require.Len(t, r.Warnings, 1)
	_, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/orders")
	assert.NoError(t, err, "nobody inherited the old asset, so it is still where it was until someone merges it")

	// And it stays protected on later runs until someone merges it.
	processAs(t, service, "shop", "user:alice", []CreateAssetInput{
		qualified("orders", []string{"a", "public", "orders"}, nil),
		qualified("orders", []string{"b", "public", "orders"}, nil),
	}, nil)
	_, err = assetSvc.GetByMRN(ctx, "mrn://table/postgresql/orders")
	assert.NoError(t, err)
}

func TestTwoOwnersOfOnePipelineNeverDeleteEachOthersAssets(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	processAs(t, service, "local-duckdb", "user:alice", []CreateAssetInput{tableInput("alice_only"), tableInput("shared")}, nil)
	bob := processAs(t, service, "local-duckdb", "user:bob", []CreateAssetInput{tableInput("bob_only"), tableInput("shared")}, nil)
	assert.Empty(t, bob.StaleEntitiesRemoved)
	_, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/alice_only")
	assert.NoError(t, err)

	// Alice dropping a table only takes her own word back; the shared one survives because Bob still sees it.
	alice := processAs(t, service, "local-duckdb", "user:alice", nil, nil)
	assert.ElementsMatch(t, []string{"mrn://asset/table/postgresql/alice_only"}, alice.StaleEntitiesRemoved)
	assert.ElementsMatch(t, []string{"mrn://asset/table/postgresql/shared"}, alice.StaleContributionsWithdrawn)
	_, err = assetSvc.GetByMRN(ctx, "mrn://table/postgresql/shared")
	assert.NoError(t, err)
}

func TestAnAssetAnotherWriterDescribesOutlivesThePipeline(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	processAs(t, service, "pg", "schedule:1", []CreateAssetInput{tableInput("orders")}, nil)
	processAs(t, service, "trino", "schedule:2", []CreateAssetInput{tableInput("orders")}, nil)

	r := processAs(t, service, "trino", "schedule:2", nil, nil)
	assert.Empty(t, r.StaleEntitiesRemoved)
	_, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/orders")
	require.NoError(t, err)

	r = processAs(t, service, "pg", "schedule:1", nil, nil)
	assert.Equal(t, []string{"mrn://asset/table/postgresql/orders"}, r.StaleEntitiesRemoved)
}

func TestTheFirstOwnedRunAdoptsThePreOwnerBaselineWithoutDeleting(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	// A run from before owners existed saw both tables; one of them was really someone else's.
	process(t, service, "local-duckdb", []CreateAssetInput{tableInput("alice_only"), tableInput("bob_only")}, nil)

	r := processAs(t, service, "local-duckdb", "user:alice", []CreateAssetInput{tableInput("alice_only")}, nil)
	assert.Empty(t, r.StaleEntitiesRemoved)
	_, err := assetSvc.GetByMRN(ctx, "mrn://table/postgresql/bob_only")
	assert.NoError(t, err)

	contribs, err := assetSvc.Contributions(ctx, mustID(t, assetSvc, "mrn://table/postgresql/alice_only"))
	require.NoError(t, err)
	writers := []string{}
	for _, c := range contribs {
		writers = append(writers, c.Writer)
	}
	assert.Equal(t, []string{"run:local-duckdb/postgresql#user:alice"}, writers, "the pre-owner contribution is handed over")
}

func TestGivingAPipelineAZoneMovesItsAssets(t *testing.T) {
	pool := pgtest.TempDB(t)
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	processAs(t, service, "pg", "schedule:1", []CreateAssetInput{qualified("orders", []string{"shop", "public", "orders"}, nil)}, nil)
	before := mustID(t, assetSvc, "mrn://asset/table/postgresql/shop.public.orders")

	m := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").WithZone("prod-eu").String()
	in := CreateAssetInput{Name: "orders", MRN: &m, Type: "Table", Providers: []string{"PostgreSQL"}}
	r := processAs(t, service, "pg", "schedule:1", []CreateAssetInput{in}, nil)
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders", r.Assets[0].MovedFrom)
	assert.Equal(t, before, mustID(t, assetSvc, m))
}

func TestHostHintsResolveThroughBindings(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, lineageService := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	eu := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").WithZone("prod-eu").String()
	us := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").WithZone("prod-us").String()
	topic := mrn.Asset("Topic", "Kafka", "orders").String()
	processAs(t, service, "pg", "schedule:1", []CreateAssetInput{
		{Name: "orders", MRN: &eu, Type: "Table", Providers: []string{"PostgreSQL"}},
		{Name: "orders", MRN: &us, Type: "Table", Providers: []string{"PostgreSQL"}},
		{Name: "orders", MRN: &topic, Type: "Topic", Providers: []string{"Kafka"}},
	}, nil)
	require.NoError(t, service.RecordZoneBindings(ctx, []asset.ZoneBinding{{Provider: "PostgreSQL", Host: "PG-US.internal:5432", Zone: "prod-us"}}))

	hinted := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").WithHost("pg-us.internal:5432").String()
	r := processAs(t, service, "connect", "schedule:3", nil, []LineageInput{{Source: topic, Target: hinted, Type: "DIRECT"}})
	require.Len(t, r.Lineage, 1)
	assert.Equal(t, us, r.Lineage[0].Target)
	assert.NotEqual(t, StatusFailed, r.Lineage[0].Status)

	exists, err := lineageService.EdgeExists(ctx, topic, us)
	require.NoError(t, err)
	assert.True(t, exists)
	_ = assetSvc
}

func TestAPersonsDescriptionBeatsThePipelines(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	fromPipeline := "from the database comment"
	in := tableInput("orders")
	in.Description = &fromPipeline
	in.Metadata = map[string]interface{}{"owner": "db", "rows": 10}
	processAs(t, service, "pg", "schedule:1", []CreateAssetInput{in}, nil)
	id := mustID(t, assetSvc, "mrn://table/postgresql/orders")

	byHand := "written by a person"
	_, err := assetSvc.Contribute(ctx, id, "api:user:1", asset.RoleManual, asset.ContributionFields{Description: &byHand, Metadata: map[string]interface{}{"owner": "data team"}}, true)
	require.NoError(t, err)

	in.Metadata = map[string]interface{}{"owner": "db", "rows": 11}
	processAs(t, service, "pg", "schedule:1", []CreateAssetInput{in}, nil)
	got, err := assetSvc.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, byHand, *got.Description)
	assert.Equal(t, "data team", got.Metadata["owner"])
	assert.EqualValues(t, 11, got.Metadata["rows"])
}

func mustID(t *testing.T, svc asset.Service, m string) string {
	t.Helper()
	a, err := svc.GetByMRN(context.Background(), m)
	require.NoError(t, err, m)
	return a.ID
}

// Alice's run moves the old shared asset to her zone; Bob's run, with a table of the same name on his own zone, must get a new asset rather than take Alice's over through the alias the move left behind.
func TestASecondZoneNeverTakesOverAnAssetAlreadyMoved(t *testing.T) {
	pool := pgtest.TempDB(t)
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	processAs(t, service, "local", "user:x", []CreateAssetInput{tableInput("orders")}, nil)
	old := mustID(t, assetSvc, "mrn://table/postgresql/orders")

	mine := mrn.Asset("Table", "PostgreSQL", "dev", "public", "orders").WithZone("alice").String()
	processAs(t, service, "local", "user:x/alice", []CreateAssetInput{{Name: "orders", MRN: &mine, Type: "Table", Providers: []string{"PostgreSQL"}}}, nil)
	assert.Equal(t, old, mustID(t, assetSvc, mine))

	theirs := mrn.Asset("Table", "PostgreSQL", "dev", "public", "orders").WithZone("bob").String()
	r := processAs(t, service, "local", "user:x/bob", []CreateAssetInput{{Name: "orders", MRN: &theirs, Type: "Table", Providers: []string{"PostgreSQL"}}}, nil)
	assert.Equal(t, StatusCreated, r.Assets[0].Status)
	assert.Equal(t, old, mustID(t, assetSvc, mine), "Alice keeps her asset")
	assert.NotEqual(t, old, mustID(t, assetSvc, theirs))
}

// Setting a zone changes who owns the pipeline's runs; the first zoned run takes over the unzoned owner's contribution, so the old owner doesn't keep the assets alive forever.
func TestSettingAZoneHandsOverThePreviousOwnersContribution(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	service, _ := newTestService(t, pool)
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))

	plain := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").String()
	processAs(t, service, "pg", "user:alice", []CreateAssetInput{{Name: "orders", MRN: &plain, Type: "Table", Providers: []string{"PostgreSQL"}}}, nil)

	zoned := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").WithZone("prod-eu").String()
	r := processAs(t, service, "pg", "user:alice/prod-eu", []CreateAssetInput{{Name: "orders", MRN: &zoned, Type: "Table", Providers: []string{"PostgreSQL"}}}, nil)
	assert.Empty(t, r.StaleEntitiesRemoved)

	contribs, err := assetSvc.Contributions(ctx, mustID(t, assetSvc, zoned))
	require.NoError(t, err)
	writers := []string{}
	for _, c := range contribs {
		writers = append(writers, c.Writer)
	}
	assert.Equal(t, []string{"run:pg/postgresql#user:alice/prod-eu"}, writers)

	// Dropping the table now removes it, because nobody else describes it.
	r = processAs(t, service, "pg", "user:alice/prod-eu", nil, nil)
	assert.Equal(t, []string{zoned}, r.StaleEntitiesRemoved)
}
