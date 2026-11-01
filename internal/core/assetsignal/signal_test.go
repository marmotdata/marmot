package assetsignal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestPopularityLevel(t *testing.T) {
	assert.Equal(t, 0, PopularityLevel(0, 1000))
	assert.Equal(t, 1, PopularityLevel(1, 0), "a single view in a quiet catalog is not full bars")
	assert.Equal(t, 4, PopularityLevel(1000, 1000))
	assert.Equal(t, 4, PopularityLevel(5000, 1000), "views above the busiest clamp to the top level")
	prev := 0
	for _, v := range []int64{1, 5, 20, 100, 400, 1000} {
		level := PopularityLevel(v, 1000)
		assert.GreaterOrEqual(t, level, prev, "levels never fall as views grow")
		assert.GreaterOrEqual(t, level, 1)
		assert.LessOrEqual(t, level, MaxPopularity)
		prev = level
	}
}

func TestScheduleInterval(t *testing.T) {
	from := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) // a Wednesday
	hourly, err := ScheduleInterval("0 * * * *", from)
	require.NoError(t, err)
	assert.Equal(t, time.Hour, hourly)

	// Weekdays only: the weekend gap is the longest pause.
	weekdays, err := ScheduleInterval("0 9 * * 1-5", from)
	require.NoError(t, err)
	assert.Equal(t, 72*time.Hour, weekdays)

	_, err = ScheduleInterval("not a cron", from)
	assert.Error(t, err)
}

func TestSizeOf(t *testing.T) {
	size := SizeOf(map[string]interface{}{"row_count": float64(1200)}, map[string]string{"columns": `[{"name":"id"},{"name":"total"}]`})
	require.NotNil(t, size.Rows)
	require.NotNil(t, size.Columns)
	assert.Equal(t, int64(1200), *size.Rows)
	assert.Equal(t, 2, *size.Columns)

	size = SizeOf(nil, map[string]string{"schema": "syntax = \"proto3\";"})
	assert.Nil(t, size.Rows)
	assert.Nil(t, size.Columns)
}

func TestStaleAndPopularity(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()
	now := time.Now()

	_, err := db.Exec(ctx, `
		INSERT INTO assets (id, name, mrn, type, providers, created_by) VALUES
			('fresh', 'fresh', 'mrn://table/pg/fresh', 'table', '{postgresql}', 'test'),
			('old', 'old', 'mrn://table/pg/old', 'table', '{postgresql}', 'test'),
			('paused', 'paused', 'mrn://table/pg/paused', 'table', '{postgresql}', 'test'),
			('manual', 'manual', 'mrn://table/pg/manual', 'table', '{postgresql}', 'test')`)
	require.NoError(t, err)

	var hourly, disabled string
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO ingestion_schedules (name, plugin_id, cron_expression) VALUES ('shop', 'postgresql', '0 * * * *') RETURNING id::text`).Scan(&hourly))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO ingestion_schedules (name, plugin_id, cron_expression, enabled) VALUES ('off', 'postgresql', '0 * * * *', false) RETURNING id::text`).Scan(&disabled))
	_, err = db.Exec(ctx, `
		INSERT INTO asset_schedules (asset_id, schedule_id, updated_at) VALUES
			('fresh', $1, $3), ('old', $1, $4), ('paused', $2, $4)`,
		hourly, disabled, now.Add(-30*time.Minute), now.Add(-5*time.Hour))
	require.NoError(t, err)

	svc := New(db)
	stale, err := svc.Stale(ctx, []string{"fresh", "old", "paused", "manual"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"fresh": false, "old": true, "paused": false, "manual": false}, stale)

	list, err := svc.ListStale(ctx, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	assert.Equal(t, "old", list.Assets[0].ID)
	assert.Equal(t, "shop", list.Assets[0].PipelineName)
	assert.Equal(t, []string{"postgresql"}, list.Assets[0].Providers)

	_, err = db.Exec(ctx, `
		INSERT INTO metrics_timeseries (metric_name, metric_type, labels, hour, day, total_sum)
		VALUES ('asset_views_total', 'counter', '{"asset_id":"fresh"}', date_trunc('hour', now()), CURRENT_DATE, 80),
		       ('asset_views_total', 'counter', '{"asset_id":"old"}', date_trunc('hour', now()), CURRENT_DATE, 2)`)
	require.NoError(t, err)
	pop, err := svc.Popularity(ctx, []string{"fresh", "old", "manual"})
	require.NoError(t, err)
	assert.Equal(t, 4, pop["fresh"])
	assert.Equal(t, 0, pop["manual"])
	assert.Greater(t, pop["old"], 0)
	assert.Less(t, pop["old"], pop["fresh"])
}
