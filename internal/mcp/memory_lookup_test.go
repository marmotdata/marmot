package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/memory"
)

// listOnlyMemory serves List from fixed entries, newest first.
type listOnlyMemory struct {
	memory.Service
	entries []*memory.Memory
}

func (s listOnlyMemory) List(_ context.Context, _ memory.Entity, f memory.ListFilter) (*memory.ListResult, error) {
	return &memory.ListResult{Memories: s.entries[:min(f.Limit, len(s.entries))], Total: len(s.entries)}, nil
}

type readableMemory struct{ MemoryAccess }

func (readableMemory) CanRead(context.Context, memory.Entity) (bool, error) { return true, nil }

func entries(n int, size int) []*memory.Memory {
	out := make([]*memory.Memory, n)
	for i := range out {
		out[i] = &memory.Memory{
			ID:        fmt.Sprintf("m%d", i),
			Content:   strings.Repeat("x", size),
			CreatedBy: memory.Author{ID: "a", Name: "agent"}, UpdatedBy: memory.Author{ID: "a", Name: "agent"},
			UpdatedAt: time.Unix(0, 0),
		}
	}
	return out
}

func TestMemorySectionCarriesTheNewestMemories(t *testing.T) {
	e := memory.Entity{Type: memory.EntityAsset, ID: "a1"}

	tc := &ToolContext{memoryService: listOnlyMemory{entries: entries(defaultLookupLimit+3, 50)}, memoryAccess: readableMemory{}}
	got := tc.memorySection(context.Background(), e, `{"asset_id": "a1"}`)
	if n := strings.Count(got, "\n- "); n != defaultLookupLimit {
		t.Errorf("%d shown, want %d:\n%s", n, defaultLookupLimit, got)
	}
	if !strings.Contains(got, `3 older memories not shown. Use recall with {"asset_id": "a1"}`) {
		t.Errorf("should say how many were left out:\n%s", got)
	}

	tc.memoryService = listOnlyMemory{entries: entries(4, 50)}
	got = tc.memorySection(context.Background(), e, `{"asset_id": "a1"}`)
	if n := strings.Count(got, "\n- "); n != 4 || strings.Contains(got, "not shown") {
		t.Errorf("all memories fit:\n%s", got)
	}

	tc.memoryService = listOnlyMemory{}
	if got := tc.memorySection(context.Background(), e, ""); !strings.Contains(got, "Nothing remembered yet.") {
		t.Errorf("empty: %q", got)
	}
}
