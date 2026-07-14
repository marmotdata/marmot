package lookups

import (
	"context"
	"sync"
	"testing"
)

func TestRecorderConcurrent(t *testing.T) {
	r := NewRecorder()

	const goroutines = 200
	const perGoroutine = 500

	sources := []string{SourceHTTP, SourceMCP, SourceWeb}
	cats := []Category{CategoryAssetDetail, CategoryLineage}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ctx := WithSource(context.Background(), sources[g%len(sources)])
			cat := cats[g%len(cats)]
			for i := 0; i < perGoroutine; i++ {
				r.Record(ctx, cat)
			}
		}(g)
	}
	wg.Wait()

	snap := r.Snapshot()
	var total int64
	for _, cats := range snap {
		for _, n := range cats {
			total += n
		}
	}
	if want := int64(goroutines * perGoroutine); total != want {
		t.Fatalf("total = %d, want %d (snapshot=%v)", total, want, snap)
	}

	// Second snapshot should be empty — Snapshot drains.
	if snap2 := r.Snapshot(); len(snap2) != 0 {
		t.Fatalf("expected empty snapshot after drain, got %v", snap2)
	}
}

func TestSourceFromDefault(t *testing.T) {
	if got := SourceFrom(context.Background()); got != SourceOther {
		t.Fatalf("SourceFrom(empty) = %q, want %q", got, SourceOther)
	}
	ctx := WithSource(context.Background(), SourceMCP)
	if got := SourceFrom(ctx); got != SourceMCP {
		t.Fatalf("SourceFrom(mcp) = %q, want %q", got, SourceMCP)
	}
}
