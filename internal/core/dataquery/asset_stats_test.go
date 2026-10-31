package dataquery

import (
	"testing"
	"time"
)

func TestFillDays(t *testing.T) {
	from := time.Date(2026, 10, 17, 13, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	found := map[int64]int64{
		time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC).Unix(): 3,
		time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC).Unix(): 7,
	}
	days := fillDays(found, from, to)
	if len(days) != 15 {
		t.Fatalf("got %d days, want 15 (both partial ends included)", len(days))
	}
	if days[0] != 3 || days[14] != 7 || days[7] != 0 {
		t.Fatalf("unexpected series %v", days)
	}
}
