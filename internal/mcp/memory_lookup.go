package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/memory"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// defaultLookupLimit is how many memories a single-entity lookup carries when
// memory.lookup_limit is not set.
const defaultLookupLimit = 25

func (tc *ToolContext) lookupLimit() int {
	if tc.config != nil && tc.config.Memory.LookupLimit > 0 {
		return tc.config.Memory.LookupLimit
	}
	return defaultLookupLimit
}

// memorySection renders the memory a lookup of one entity carries. ref is the
// recall argument naming the entity, for the hint when entries are left out.
// It is empty when memory is off or the caller cannot read the entity.
func (tc *ToolContext) memorySection(ctx context.Context, e memory.Entity, ref string) string {
	if tc.memoryService == nil || tc.memoryAccess == nil {
		return ""
	}
	if ok, err := tc.memoryAccess.CanRead(ctx, e); err != nil || !ok {
		return ""
	}

	recent, err := tc.memoryService.List(ctx, e, memory.ListFilter{Limit: tc.lookupLimit()})
	if err != nil {
		log.Warn().Err(err).Str("entity_type", string(e.Type)).Str("entity_id", e.ID).Msg("Failed to list memory for a lookup")
		return ""
	}

	var b strings.Builder
	b.WriteString("## Memory\n")
	if recent.Total == 0 {
		b.WriteString("\nNothing remembered yet.")
		return b.String()
	}

	for _, m := range recent.Memories {
		b.WriteString(formatMemory(m))
	}
	left := recent.Total - len(recent.Memories)
	if left > 0 {
		fmt.Fprintf(&b, "\n\n%d older %s not shown. Use recall with %s and a query to search them.",
			left, plural(left, "memory", "memories"), ref)
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// recallEverywhere searches the memory of every asset and data product.
func (tc *ToolContext) recallEverywhere(ctx context.Context, args RecallInput, filter memory.Filter) (*mcpsdk.CallToolResult, any, error) {
	if strings.TrimSpace(args.Query) == "" {
		return tc.errorWithGuidance("A query or an entity is required",
			"Pass a query to search everything you can see, or name an asset or data product.", nil), nil, nil
	}
	if ok, err := tc.memoryAccess.CanRead(ctx, memory.Entity{}); err != nil || !ok {
		return tc.errorWithGuidance("Not allowed to read memory", "Reading memory needs assets:view.", nil), nil, nil
	}
	result, err := tc.memoryService.SearchAll(ctx, memory.SearchQuery{
		Filter: filter, Query: args.Query, Limit: args.Limit,
	})
	if err != nil {
		return tc.memoryError(err), nil, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Memory matching %q\n", args.Query)
	if len(result.Memories) == 0 {
		b.WriteString("\nNothing found.")
	}
	labels := map[memory.Entity]string{}
	for _, m := range result.Memories {
		e := memory.Entity{Type: m.EntityType, ID: m.EntityID}
		if _, ok := labels[e]; !ok {
			labels[e] = tc.entityLabel(ctx, e)
		}
		b.WriteString(formatMemory(m) + " · on " + labels[e])
	}
	return textResult(b.String()), nil, nil
}

// entityLabel names an entity and gives the argument that addresses it.
func (tc *ToolContext) entityLabel(ctx context.Context, e memory.Entity) string {
	switch e.Type {
	case memory.EntityAsset:
		name := e.ID
		if a, err := tc.assetService.Get(ctx, e.ID); err == nil && a.Name != nil {
			name = *a.Name
		}
		return fmt.Sprintf("asset '%s' (asset_id %s)", escapeMarkdown(name), e.ID)
	case memory.EntityDataProduct:
		name := e.ID
		if p, err := tc.dataProductService.Get(ctx, e.ID); err == nil {
			name = p.Name
		}
		return fmt.Sprintf("data product '%s' (data_product_id %s)", escapeMarkdown(name), e.ID)
	}
	return string(e.Type) + " " + e.ID
}
