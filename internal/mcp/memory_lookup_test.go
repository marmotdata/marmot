package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/memory"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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
	if !strings.Contains(got, `3 less used memories not shown. Use recall with {"asset_id": "a1"}`) {
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

func TestClientsReceiveMemoryInstructions(t *testing.T) {
	ctx := context.Background()
	s := &Server{}
	s.SetMemory(listOnlyMemory{}, readableMemory{})
	server := s.CreateMCPServer(ctx, auth.NewServiceAccountPrincipal("sa-1", "agent", nil, nil))

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()

	if got := session.InitializeResult().Instructions; !strings.Contains(got, "change it with update_memory") {
		t.Errorf("instructions: %q", got)
	}
}
