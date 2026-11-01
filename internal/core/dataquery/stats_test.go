package dataquery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFillSeriesHourly(t *testing.T) {
	from := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Hour)
	found := map[int64]StatsPoint{
		from.Add(time.Hour).Unix(): {Bucket: from.Add(time.Hour), Succeeded: 3, Denied: 1},
	}

	points := fillSeries(found, from, to, IntervalHour)
	require.Len(t, points, 4, "one point per hour in [from, to)")
	assert.Equal(t, from, points[0].Bucket)
	assert.Zero(t, points[0].Succeeded)
	assert.Equal(t, int64(3), points[1].Succeeded)
	assert.Equal(t, int64(1), points[1].Denied)
	assert.Equal(t, from.Add(3*time.Hour), points[3].Bucket)
}

func TestFillSeriesDailyStartsAtMidnight(t *testing.T) {
	from := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)

	points := fillSeries(map[int64]StatsPoint{}, from, to, IntervalDay)
	require.Len(t, points, 8, "the partial first and last days are included")
	assert.Equal(t, time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), points[0].Bucket)
}

func TestStatsDefaultsAndValidation(t *testing.T) {
	f := newFixture(t)
	repo := f.repo

	_, err := f.svc.Stats(context.Background(), StatsFilter{})
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, repo.lastStats.To.Sub(repo.lastStats.From), "defaults to the last day")
	assert.Equal(t, IntervalHour, repo.lastStats.Interval)
	assert.Equal(t, 6, repo.lastStats.Limit)

	to := time.Now().UTC()
	_, err = f.svc.Stats(context.Background(), StatsFilter{From: to.Add(-7 * 24 * time.Hour), To: to})
	require.NoError(t, err)
	assert.Equal(t, IntervalDay, repo.lastStats.Interval, "a week defaults to daily points")

	_, err = f.svc.Stats(context.Background(), StatsFilter{From: to.Add(-200 * 24 * time.Hour), To: to})
	require.ErrorIs(t, err, ErrInvalidInput)

	_, err = f.svc.Stats(context.Background(), StatsFilter{From: to, To: to.Add(-time.Hour)})
	require.ErrorIs(t, err, ErrInvalidInput)

	_, err = f.svc.Stats(context.Background(), StatsFilter{Interval: "minute"})
	require.ErrorIs(t, err, ErrInvalidInput)
}
