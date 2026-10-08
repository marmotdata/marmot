package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The write tools follow a two-phase propose-then-apply shape. Called with confirm false, a tool resolves the asset and returns the exact before and after of the change without touching anything; called with confirm true it performs the write. This keeps a human, or the Slack approval button that re-invokes with confirm true, in the loop for every mutation without needing any pending-change storage. Permission is enforced here at the tool layer rather than in the prompt, so a description carrying injected instructions can never exceed the caller's real grants.

type UpdateDocumentationInput struct {
	ID            string `json:"id,omitempty"`
	MRN           string `json:"mrn,omitempty"`
	Documentation string `json:"documentation"`
	Confirm       bool   `json:"confirm,omitempty"`
}

type ManageTagsInput struct {
	ID         string   `json:"id,omitempty"`
	MRN        string   `json:"mrn,omitempty"`
	AddTags    []string `json:"add_tags,omitempty"`
	RemoveTags []string `json:"remove_tags,omitempty"`
	Confirm    bool     `json:"confirm,omitempty"`
}

type ManageOwnersInput struct {
	ID        string `json:"id,omitempty"`
	MRN       string `json:"mrn,omitempty"`
	Action    string `json:"action"`
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
	Confirm   bool   `json:"confirm,omitempty"`
}

// requireManage returns a refusal result when the caller lacks the given manage permission, or nil when the write may proceed. Admins bypass the fine-grained check the same way they do on the REST endpoints.
func (tc *ToolContext) requireManage(resource string) *mcpsdk.CallToolResult {
	if tc.principal != nil && (tc.principal.IsAdmin() || tc.principal.HasPermission(resource, "manage")) {
		return nil
	}
	return tc.errorWithGuidance(
		"Permission denied",
		fmt.Sprintf("Changing the catalog needs the %s:manage permission, which the current identity does not hold. Continue as a read-only assistant and tell the user they do not have permission to make this change.", resource),
		nil,
	)
}

// resolveWritableAsset loads the asset a write tool targets by id or mrn, returning a refusal result the caller should hand straight back when neither is usable.
func (tc *ToolContext) resolveWritableAsset(ctx context.Context, id, mrn string) (*asset.Asset, *mcpsdk.CallToolResult) {
	var (
		a   *asset.Asset
		err error
	)
	switch {
	case id != "":
		a, err = tc.assetService.Get(ctx, id)
	case mrn != "":
		a, err = tc.assetService.GetByMRN(ctx, mrn)
	default:
		return nil, tc.errorWithGuidance(
			"Missing asset",
			"Provide the id or mrn of the asset to change.",
			map[string]string{"Find the asset": `Use discover_data with {"query": "asset name"}`},
		)
	}
	if errors.Is(err, asset.ErrAssetNotFound) || errors.Is(err, asset.ErrNotFound) {
		return nil, tc.errorWithGuidance(
			"Asset not found",
			"The id or mrn may be wrong.",
			map[string]string{"Find the asset": `Use discover_data with {"query": "asset name"}`},
		)
	}
	if err != nil {
		return nil, tc.errorWithGuidance("Asset lookup failed", fmt.Sprintf("Error: %v", err), nil)
	}
	return a, nil
}

func escapeMarkdownAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = escapeMarkdown(v)
	}
	return out
}

// partialFailureDetail reports a mid-sequence failure together with the writes that already landed, so a half-applied change is never mistaken for an untouched one.
func partialFailureDetail(cause string, done []string) string {
	if len(done) == 0 {
		return fmt.Sprintf("Error %s. No changes were applied.", cause)
	}
	return fmt.Sprintf("Error %s. Applied before the failure: %s. The asset is partially updated.", cause, strings.Join(done, ", "))
}

// proposal renders the confirm-false preview: what would change, and the exact call that applies it.
func proposal(title, diff, applyCall string) *mcpsdk.CallToolResult {
	text := fmt.Sprintf("# Proposed change\n\n%s\n\n%s\n\n---\nThis is a preview. Nothing has changed yet. To apply, call this tool again with the same arguments plus `\"confirm\": true`:\n\n%s", title, diff, applyCall)
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// applied renders the confirm-true success result.
func applied(title, detail string) *mcpsdk.CallToolResult {
	text := fmt.Sprintf("# Change applied\n\n%s", title)
	if detail != "" {
		text += "\n\n" + detail
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "_(none)_"
	}
	return s
}

func (tc *ToolContext) updateDocumentation(ctx context.Context, req *mcpsdk.CallToolRequest, args UpdateDocumentationInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, args.ID, args.MRN)
	if bad != nil {
		return bad, nil, nil
	}

	current := ""
	if a.UserDescription != nil {
		current = *a.UserDescription
	}
	proposed := strings.TrimSpace(args.Documentation)
	if proposed == current {
		return tc.errorWithGuidance("Nothing to change", "The documentation already matches the requested value.", nil), nil, nil
	}

	verb := "Set"
	if proposed == "" {
		verb = "Clear"
	}
	title := fmt.Sprintf("%s the human-authored documentation on **%s** (%s)", verb, escapeMarkdown(*a.Name), a.Type)
	diff := fmt.Sprintf("**Current:**\n%s\n\n**Proposed:**\n%s", orNone(current), orNone(proposed))

	if !args.Confirm {
		return proposal(title, diff, formatJSON(map[string]any{"id": a.ID, "documentation": proposed, "confirm": true})), nil, nil
	}

	if _, err := tc.assetService.Update(ctx, a.ID, asset.UpdateInput{UserDescription: &proposed}); err != nil {
		return tc.errorWithGuidance("Update failed", fmt.Sprintf("Error: %v", err), nil), nil, nil
	}
	return applied(title, diff), nil, nil
}

func (tc *ToolContext) manageTags(ctx context.Context, req *mcpsdk.CallToolRequest, args ManageTagsInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, args.ID, args.MRN)
	if bad != nil {
		return bad, nil, nil
	}

	existing := map[string]bool{}
	for _, t := range a.Tags {
		existing[t] = true
	}
	var toAdd, toRemove []string
	for _, t := range args.AddTags {
		if t = strings.TrimSpace(t); t != "" && !existing[t] {
			toAdd = append(toAdd, t)
		}
	}
	for _, t := range args.RemoveTags {
		if t = strings.TrimSpace(t); t != "" && existing[t] {
			toRemove = append(toRemove, t)
		}
	}
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return tc.errorWithGuidance("Nothing to change", "The requested tags are already in the desired state on this asset.", nil), nil, nil
	}

	title := fmt.Sprintf("Update tags on **%s** (%s)", escapeMarkdown(*a.Name), a.Type)
	var diffParts []string
	if len(toAdd) > 0 {
		diffParts = append(diffParts, "**Add:** "+strings.Join(escapeMarkdownAll(toAdd), ", "))
	}
	if len(toRemove) > 0 {
		diffParts = append(diffParts, "**Remove:** "+strings.Join(escapeMarkdownAll(toRemove), ", "))
	}
	diff := strings.Join(diffParts, "\n\n")

	if !args.Confirm {
		call := map[string]any{"id": a.ID, "confirm": true}
		if len(toAdd) > 0 {
			call["add_tags"] = toAdd
		}
		if len(toRemove) > 0 {
			call["remove_tags"] = toRemove
		}
		return proposal(title, diff, formatJSON(call)), nil, nil
	}

	var done []string
	for _, t := range toAdd {
		if _, err := tc.assetService.AddTag(ctx, a.ID, t); err != nil {
			return tc.errorWithGuidance("Tag update failed", partialFailureDetail(fmt.Sprintf("adding %q: %v", t, err), done), nil), nil, nil
		}
		done = append(done, fmt.Sprintf("added %q", t))
	}
	for _, t := range toRemove {
		if _, err := tc.assetService.RemoveTag(ctx, a.ID, t); err != nil {
			return tc.errorWithGuidance("Tag update failed", partialFailureDetail(fmt.Sprintf("removing %q: %v", t, err), done), nil), nil, nil
		}
		done = append(done, fmt.Sprintf("removed %q", t))
	}
	return applied(title, diff), nil, nil
}

func (tc *ToolContext) manageOwners(ctx context.Context, req *mcpsdk.CallToolRequest, args ManageOwnersInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action != "add" && action != "remove" {
		return tc.errorWithGuidance("Invalid action", `action must be "add" or "remove".`, nil), nil, nil
	}
	ownerType := strings.ToLower(strings.TrimSpace(args.OwnerType))
	if ownerType != "user" && ownerType != "team" {
		return tc.errorWithGuidance("Invalid owner_type", `owner_type must be "user" or "team".`, nil), nil, nil
	}
	ownerID := strings.TrimSpace(args.OwnerID)
	if ownerID == "" {
		return tc.errorWithGuidance(
			"Missing owner_id",
			"Provide the id of the user or team to assign.",
			map[string]string{
				"Find a team's id": `Use explore_teams with {"team_name": "data-engineering"}`,
				"Find a user's id": `Use find_ownership or explore_teams to resolve a username to an id`,
			},
		), nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, args.ID, args.MRN)
	if bad != nil {
		return bad, nil, nil
	}

	// A failed owner listing falls through to the write itself, which surfaces any real error.
	if owners, err := tc.teamService.ListAssetOwners(ctx, a.ID); err == nil {
		assigned := false
		for _, o := range owners {
			if strings.EqualFold(o.Type, ownerType) && o.ID == ownerID {
				assigned = true
				break
			}
		}
		if action == "add" && assigned {
			return tc.errorWithGuidance("Nothing to change", "That owner is already assigned to this asset.", nil), nil, nil
		}
		if action == "remove" && !assigned {
			return tc.errorWithGuidance("Nothing to change", "That owner is not assigned to this asset.", nil), nil, nil
		}
	}

	title := fmt.Sprintf("%s %s owner **%s** %s **%s** (%s)",
		capitalise(action), ownerType, escapeMarkdown(ownerID), preposition(action), escapeMarkdown(*a.Name), a.Type)

	if !args.Confirm {
		return proposal(title, "", formatJSON(map[string]any{"id": a.ID, "action": action, "owner_type": ownerType, "owner_id": ownerID, "confirm": true})), nil, nil
	}

	var err error
	if action == "add" {
		err = tc.teamService.AddAssetOwner(ctx, a.ID, ownerType, ownerID)
	} else {
		err = tc.teamService.RemoveAssetOwner(ctx, a.ID, ownerType, ownerID)
	}
	if err != nil {
		return tc.errorWithGuidance("Owner change failed", fmt.Sprintf("Error: %v", err), nil), nil, nil
	}
	return applied(title, ""), nil, nil
}

func preposition(action string) string {
	if action == "remove" {
		return "from"
	}
	return "to"
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
