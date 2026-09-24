package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/memory"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// MemoryAccess reports whether the caller may read or write an entity's
// memory.
type MemoryAccess interface {
	CanRead(ctx context.Context, e memory.Entity) (bool, error)
	CanWrite(ctx context.Context, e memory.Entity) (bool, error)
}

// EntityRef names the entity a memory call is about. Exactly one entity is
// identified.
type EntityRef struct {
	AssetID         string `json:"asset_id,omitempty" jsonschema:"ID of the asset"`
	AssetMRN        string `json:"asset_mrn,omitempty" jsonschema:"MRN of the asset, when the ID is not known"`
	DataProductID   string `json:"data_product_id,omitempty" jsonschema:"ID of the data product"`
	DataProductName string `json:"data_product_name,omitempty" jsonschema:"Exact name of the data product, when the ID is not known"`
}

func (r EntityRef) empty() bool {
	return r == EntityRef{}
}

// memoryTarget is a resolved, access-checked entity.
type memoryTarget struct {
	entity memory.Entity
	label  string
}

type RememberInput struct {
	EntityRef
	Content   string `json:"content" jsonschema:"One short fact about the entity, at most 280 characters"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Your run, conversation or task ID"`
}

type RecallInput struct {
	EntityRef
	Query     string `json:"query,omitempty" jsonschema:"What to look for. Omit to list the entity's most recent memories."`
	SessionID string `json:"session_id,omitempty" jsonschema:"Only memories written or edited in this session"`
	Limit     int    `json:"limit,omitempty"`
}

type UpdateMemoryInput struct {
	EntityRef
	MemoryID  string `json:"memory_id" jsonschema:"ID returned when the memory was written"`
	Content   string `json:"content" jsonschema:"The corrected fact, at most 280 characters"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Your run, conversation or task ID"`
}

type ForgetInput struct {
	EntityRef
	MemoryID string `json:"memory_id" jsonschema:"ID of the memory to remove"`
}

func (s *Server) registerMemoryTools(server *mcpsdk.Server, tc *ToolContext) {
	if s.memoryService == nil || s.memoryAccess == nil {
		return
	}

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "remember",
		Description: `<usecase>
Store durable knowledge about an asset or a data product so later runs (yours or other agents')
can recall it. Memory belongs to that entity and is visible to anyone who can see it.
</usecase>

<instructions>
- A memory is one short fact that enriches the entity, at most 280 characters.
- Each call adds a new memory. To change one that is already there, use update_memory.
- Store knowledge about the data, never the data itself: no values or rows copied from tables,
  no secrets, credentials or personal data. Memory is visible to everyone who can see the entity.
- Pass session_id consistently for everything written in one run.
- Attach it to the most specific entity it is about: a table's grain goes on the table.
- Identify an asset by asset_id or asset_mrn, a data product by data_product_id or data_product_name.
Returns the memory ID, needed for update_memory and forget.
</instructions>`,
	}, tc.remember)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "recall",
		Description: `<usecase>
Retrieve what has been remembered about an asset or a data product. Use before acting on it to
pick up what earlier runs and people recorded.
</usecase>

<instructions>
- With a query: the entity's most relevant memories, closest first.
- Without a query: the entity's most recently changed memories.
- With a query and no entity: the most relevant memories on every asset and data product,
  each labelled with its entity. Use it for "have we seen this before?".
- Looking up one asset or data product already returns its newest memories; use recall for
  anything that did not fit there.
- session_id limits the result to one run.
</instructions>`,
	}, tc.recall)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "update_memory",
		Description: `<usecase>
Change a memory in place, by the ID returned when it was written or shown by recall. Use it to
correct a memory or bring it up to date rather than adding a second one.
</usecase>`,
	}, tc.updateMemory)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "forget",
		Description: `<usecase>
Remove a memory from an asset or data product, by memory_id.
</usecase>`,
	}, tc.forget)
}

// resolveReadable finds the referenced entity and checks the caller can see
// it. On failure it returns a tool error result.
func (tc *ToolContext) resolveReadable(ctx context.Context, ref EntityRef) (*memoryTarget, *mcpsdk.CallToolResult) {
	return tc.resolveEntity(ctx, ref, false)
}

// resolveWritable also checks the caller may write the entity's memory. On
// failure it returns a tool error result.
func (tc *ToolContext) resolveWritable(ctx context.Context, ref EntityRef) (*memoryTarget, *mcpsdk.CallToolResult) {
	return tc.resolveEntity(ctx, ref, true)
}

// resolveEntity looks the entity up, then checks access. Callers use
// resolveReadable or resolveWritable.
func (tc *ToolContext) resolveEntity(ctx context.Context, ref EntityRef, write bool) (*memoryTarget, *mcpsdk.CallToolResult) {
	target, errResult := tc.lookupEntity(ctx, ref)
	if errResult != nil {
		return nil, errResult
	}

	visible, err := tc.memoryAccess.CanRead(ctx, target.entity)
	if err != nil {
		return nil, tc.accessCheckFailed(err)
	}
	if !visible {
		return nil, tc.errorWithGuidance(
			fmt.Sprintf("Not allowed to read memory on %s", target.label),
			"Reading memory needs the assets:view permission.",
			nil,
		)
	}
	if write {
		allowed, err := tc.memoryAccess.CanWrite(ctx, target.entity)
		if err != nil {
			return nil, tc.accessCheckFailed(err)
		}
		if !allowed {
			return nil, tc.errorWithGuidance(
				fmt.Sprintf("Not allowed to write memory on %s", target.label),
				"Writing memory needs the memory:write permission on it.",
				nil,
			)
		}
	}
	return target, nil
}

// lookupEntity finds the entity a reference names.
func (tc *ToolContext) lookupEntity(ctx context.Context, ref EntityRef) (*memoryTarget, *mcpsdk.CallToolResult) {
	assetNotFound := func(what string) *mcpsdk.CallToolResult {
		return tc.errorWithGuidance(
			fmt.Sprintf("Asset %s not found", what),
			"Find the asset first.",
			map[string]string{"Search assets": `Use discover_data with {"query": "..."}`},
		)
	}
	productNotFound := func(what string) *mcpsdk.CallToolResult {
		return tc.errorWithGuidance(
			fmt.Sprintf("Data product %s not found", what),
			"Find the product first.",
			map[string]string{"Search products": `Use explore_data_products with {"query": "..."}`},
		)
	}

	switch {
	case ref.AssetID != "":
		a, err := tc.assetService.Get(ctx, ref.AssetID)
		if errors.Is(err, asset.ErrAssetNotFound) {
			return nil, assetNotFound(fmt.Sprintf("'%s'", ref.AssetID))
		}
		if err != nil {
			return nil, tc.lookupFailed(err)
		}
		return assetTarget(a), nil
	case ref.AssetMRN != "":
		a, err := tc.assetService.GetByMRN(ctx, ref.AssetMRN)
		if errors.Is(err, asset.ErrAssetNotFound) {
			return nil, assetNotFound(fmt.Sprintf("'%s'", ref.AssetMRN))
		}
		if err != nil {
			return nil, tc.lookupFailed(err)
		}
		return assetTarget(a), nil
	case ref.DataProductID != "":
		if _, err := uuid.Parse(ref.DataProductID); err != nil {
			return nil, productNotFound(fmt.Sprintf("'%s'", ref.DataProductID))
		}
		p, err := tc.dataProductService.Get(ctx, ref.DataProductID)
		if errors.Is(err, dataproduct.ErrNotFound) {
			return nil, productNotFound(fmt.Sprintf("'%s'", ref.DataProductID))
		}
		if err != nil {
			return nil, tc.lookupFailed(err)
		}
		return productTarget(p), nil
	case ref.DataProductName != "":
		// Resolves a name the way explore_data_products does: an exact match,
		// or the only result.
		result, err := tc.dataProductService.Search(ctx, dataproduct.SearchFilter{Query: ref.DataProductName, Limit: 20})
		if err != nil {
			return nil, tc.lookupFailed(err)
		}
		for _, p := range result.DataProducts {
			if strings.EqualFold(p.Name, ref.DataProductName) {
				return productTarget(p), nil
			}
		}
		if len(result.DataProducts) == 1 {
			return productTarget(result.DataProducts[0]), nil
		}
		return nil, productNotFound(fmt.Sprintf("named '%s'", ref.DataProductName))
	}
	return nil, tc.errorWithGuidance("An entity is required",
		"Pass asset_id or asset_mrn for an asset, or data_product_id or data_product_name for a data product.", nil)
}

func assetTarget(a *asset.Asset) *memoryTarget {
	name := a.ID
	if a.Name != nil {
		name = *a.Name
	}
	return &memoryTarget{
		entity: memory.Entity{Type: memory.EntityAsset, ID: a.ID},
		label:  fmt.Sprintf("asset '%s'", oneLine(name)),
	}
}

func productTarget(p *dataproduct.DataProduct) *memoryTarget {
	return &memoryTarget{
		entity: memory.Entity{Type: memory.EntityDataProduct, ID: p.ID},
		label:  fmt.Sprintf("data product '%s'", oneLine(p.Name)),
	}
}

func (tc *ToolContext) memoryError(err error) *mcpsdk.CallToolResult {
	switch {
	case errors.Is(err, memory.ErrNotFound):
		return tc.errorWithGuidance("Memory not found", "Use recall to find the memory ID.", nil)
	case errors.Is(err, memory.ErrInvalid):
		return tc.errorWithGuidance(err.Error(), "", nil)
	}
	log.Error().Err(err).Msg("Memory operation failed")
	return tc.errorWithGuidance("Memory operation failed", "Try again later.", nil)
}

func (tc *ToolContext) accessCheckFailed(err error) *mcpsdk.CallToolResult {
	log.Error().Err(err).Msg("Failed to check memory access")
	return tc.errorWithGuidance("Failed to check access", "Try again later.", nil)
}

func (tc *ToolContext) lookupFailed(err error) *mcpsdk.CallToolResult {
	log.Error().Err(err).Msg("Failed to look up memory entity")
	return tc.errorWithGuidance("Failed to look up the entity", "Try again later.", nil)
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}}}
}

func (tc *ToolContext) remember(ctx context.Context, _ *mcpsdk.CallToolRequest, args RememberInput) (*mcpsdk.CallToolResult, any, error) {
	target, errResult := tc.resolveWritable(ctx, args.EntityRef)
	if errResult != nil {
		return errResult, nil, nil
	}

	m, err := tc.memoryService.Remember(ctx, target.entity, memory.RememberInput{
		Content:   args.Content,
		SessionID: args.SessionID,
		Author:    memory.AuthorFrom(tc.principal),
	})
	if err != nil {
		return tc.memoryError(err), nil, nil
	}
	return textResult(fmt.Sprintf("Remembered on %s.\nmemory_id: %s", target.label, m.ID)), nil, nil
}

func (tc *ToolContext) recall(ctx context.Context, _ *mcpsdk.CallToolRequest, args RecallInput) (*mcpsdk.CallToolResult, any, error) {
	filter := memory.Filter{SessionID: args.SessionID}

	if args.EntityRef.empty() {
		return tc.recallEverywhere(ctx, args, filter)
	}
	target, errResult := tc.resolveReadable(ctx, args.EntityRef)
	if errResult != nil {
		return errResult, nil, nil
	}

	if strings.TrimSpace(args.Query) != "" {
		result, err := tc.memoryService.Search(ctx, target.entity, memory.SearchQuery{
			Filter: filter, Query: args.Query, Limit: args.Limit,
		})
		if err != nil {
			return tc.memoryError(err), nil, nil
		}
		header := fmt.Sprintf("# Memory on %s matching %q", target.label, args.Query)
		return textResult(formatMemories(header, result.Memories)), nil, nil
	}

	result, err := tc.memoryService.List(ctx, target.entity, memory.ListFilter{Filter: filter, Limit: args.Limit})
	if err != nil {
		return tc.memoryError(err), nil, nil
	}
	header := fmt.Sprintf("# Memory on %s (%d of %d, most recent first)", target.label, len(result.Memories), result.Total)
	return textResult(formatMemories(header, result.Memories)), nil, nil
}

func (tc *ToolContext) updateMemory(ctx context.Context, _ *mcpsdk.CallToolRequest, args UpdateMemoryInput) (*mcpsdk.CallToolResult, any, error) {
	target, errResult := tc.resolveWritable(ctx, args.EntityRef)
	if errResult != nil {
		return errResult, nil, nil
	}
	m, err := tc.memoryService.Update(ctx, target.entity, args.MemoryID, memory.UpdateInput{
		Content:   args.Content,
		SessionID: args.SessionID,
		Author:    memory.AuthorFrom(tc.principal),
	})
	if err != nil {
		return tc.memoryError(err), nil, nil
	}
	return textResult(fmt.Sprintf("Updated memory %s on %s.", m.ID, target.label)), nil, nil
}

func (tc *ToolContext) forget(ctx context.Context, _ *mcpsdk.CallToolRequest, args ForgetInput) (*mcpsdk.CallToolResult, any, error) {
	target, errResult := tc.resolveWritable(ctx, args.EntityRef)
	if errResult != nil {
		return errResult, nil, nil
	}
	if args.MemoryID == "" {
		return tc.errorWithGuidance("memory_id is required", "Use recall to find the memory ID.", nil), nil, nil
	}
	if err := tc.memoryService.Forget(ctx, target.entity, args.MemoryID); err != nil {
		return tc.memoryError(err), nil, nil
	}
	return textResult(fmt.Sprintf("Forgot memory %s on %s.", args.MemoryID, target.label)), nil, nil
}

func formatMemories(header string, memories []*memory.Memory) string {
	if len(memories) == 0 {
		return header + "\n\nNothing found."
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for _, m := range memories {
		b.WriteString(formatMemory(m))
	}
	return b.String()
}

// formatMemory renders one entry as a list item, starting on a new line.
// Content is shown exactly as stored, so an agent can edit what it reads.
// Every field is forced onto one line, so no field can start a new entry.
func formatMemory(m *memory.Memory) string {
	var b strings.Builder
	b.Grow(len(m.Content) + 160)
	b.WriteString("\n- ")
	b.WriteString(oneLine(m.Content))
	fmt.Fprintf(&b, "\n  id: %s · by %s", m.ID, oneLine(m.CreatedBy.Name))
	if m.UpdatedBy.ID != m.CreatedBy.ID {
		fmt.Fprintf(&b, " · edited by %s", oneLine(m.UpdatedBy.Name))
	}
	fmt.Fprintf(&b, " · %s", m.UpdatedAt.UTC().Format(time.RFC3339))
	if m.SessionID != "" {
		fmt.Fprintf(&b, " · session %s", oneLine(m.SessionID))
	}
	if m.Score != nil {
		fmt.Fprintf(&b, " · score %.2f", *m.Score)
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
