package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// The write tools follow a two-phase propose-then-apply shape. Called with confirm false, a tool resolves the asset and returns the exact before and after of the change without touching anything; called with confirm true it performs the write. This keeps a human, or the Slack approval button that re-invokes with confirm true, in the loop for every mutation without needing any pending-change storage. Permission is enforced here at the tool layer as well as by listing, so a description carrying injected instructions can never exceed the caller's real grants.

type UpdateDocumentationInput struct {
	ID            string `json:"id,omitempty" jsonschema:"The asset's MRN, ID or exact name"`
	MRN           string `json:"mrn,omitempty" jsonschema:"Same as id, kept for older callers"`
	Documentation string `json:"documentation" jsonschema:"The full text to set as the asset's summary, in Markdown. An empty string clears it"`
	Confirm       bool   `json:"confirm,omitempty" jsonschema:"Leave out to preview. Send true only after the user approved the preview"`
}

type ManageTagsInput struct {
	ID         string   `json:"id,omitempty" jsonschema:"The asset's MRN, ID or exact name"`
	MRN        string   `json:"mrn,omitempty" jsonschema:"Same as id, kept for older callers"`
	AddTags    []string `json:"add_tags,omitempty" jsonschema:"Tags to add"`
	RemoveTags []string `json:"remove_tags,omitempty" jsonschema:"Tags to remove"`
	Confirm    bool     `json:"confirm,omitempty" jsonschema:"Leave out to preview. Send true only after the user approved the preview"`
}

type ManageOwnersInput struct {
	ID        string `json:"id,omitempty" jsonschema:"The asset's MRN, ID or exact name"`
	MRN       string `json:"mrn,omitempty" jsonschema:"Same as id, kept for older callers"`
	Action    string `json:"action" jsonschema:"add or remove"`
	OwnerType string `json:"owner_type" jsonschema:"user or team"`
	Owner     string `json:"owner,omitempty" jsonschema:"The team's name or the user's username, or their ID"`
	OwnerID   string `json:"owner_id,omitempty" jsonschema:"Same as owner, kept for older callers"`
	Confirm   bool   `json:"confirm,omitempty" jsonschema:"Leave out to preview. Send true only after the user approved the preview"`
}

// requireManage returns a refusal result when the caller lacks the given manage permission, or nil when the write may proceed. Admins bypass the fine-grained check the same way they do on the REST endpoints.
func (tc *ToolContext) requireManage(resource string) *mcpsdk.CallToolResult {
	if tc.principal != nil && (tc.principal.IsAdmin() || tc.principal.HasPermission(resource, "manage")) {
		return nil
	}
	return toolError(codePermission, fmt.Sprintf("Changing the catalog needs the %s:manage permission, which this identity doesn't hold. Tell the user; don't look for another way to make the change.", resource))
}

// resolveWritableAsset loads the asset a write tool targets, returning a result to hand straight back when it can't.
func (tc *ToolContext) resolveWritableAsset(ctx context.Context, tool, id, mrnRef string) (*asset.Asset, *mcpsdk.CallToolResult) {
	ref := strings.TrimSpace(id)
	if ref == "" {
		ref = strings.TrimSpace(mrnRef)
	}
	if ref == "" {
		return nil, toolError(codeInvalidArgument, "Give id: the asset's MRN, ID or exact name.")
	}
	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Str("tool", tool).Msg("MCP write lookup failed")
		return nil, unavailable("look that asset up")
	}
	if a == nil {
		return nil, assetNotFound(tool, "id", ref, candidates)
	}
	return a, nil
}

func escapeMarkdown(s string) string {
	return strings.NewReplacer("[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "_", "\\_", "*", "\\*", "`", "\\`").Replace(s)
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
		return fmt.Sprintf("Failed %s. No changes were applied.", cause)
	}
	return fmt.Sprintf("Failed %s. Applied before the failure: %s. The asset is partially updated.", cause, strings.Join(done, ", "))
}

// proposal renders the confirm-false preview: what would change, and the exact call that applies it.
func proposal(title, diff, tool string, applyArgs map[string]any) *mcpsdk.CallToolResult {
	text := fmt.Sprintf("# Proposed change\n\n%s", title)
	if diff != "" {
		text += "\n\n" + diff
	}
	text += fmt.Sprintf("\n\nThis is a preview. Nothing has changed yet. Show it to the user, and only if they approve, apply it with:\n%s %s", tool, compactJSON(applyArgs))
	return textResult(text)
}

// applied renders the confirm-true success result.
func applied(title, detail string) *mcpsdk.CallToolResult {
	text := fmt.Sprintf("# Change applied\n\n%s", title)
	if detail != "" {
		text += "\n\n" + detail
	}
	return textResult(text)
}

// unchanged answers a write that would change nothing; it succeeds, since the asset is already as asked.
func unchanged(detail string) *mcpsdk.CallToolResult {
	return textResult("Nothing to change: " + detail)
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "_(none)_"
	}
	return s
}

func (tc *ToolContext) updateDocumentation(ctx context.Context, _ *mcpsdk.CallToolRequest, args UpdateDocumentationInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, "update_documentation", args.ID, args.MRN)
	if bad != nil {
		return bad, nil, nil
	}
	current := deref(a.UserDescription)
	proposed := strings.TrimSpace(args.Documentation)
	if proposed == current {
		return unchanged("the documentation already says exactly that."), nil, nil
	}
	verb := "Set"
	if proposed == "" {
		verb = "Clear"
	}
	title := fmt.Sprintf("%s the summary people wrote on **%s** (%s)", verb, escapeMarkdown(deref(a.Name)), a.Type)
	diff := fmt.Sprintf("**Current:**\n%s\n\n**Proposed:**\n%s", orNone(current), orNone(proposed))
	if !args.Confirm {
		return proposal(title, diff, "update_documentation", map[string]any{"id": idFor(a), "documentation": proposed, "confirm": true}), nil, nil
	}
	if _, err := tc.assetService.Update(ctx, a.ID, asset.UpdateInput{UserDescription: &proposed}); err != nil {
		log.Warn().Err(err).Msg("MCP update_documentation failed")
		return unavailable("save the documentation"), nil, nil
	}
	return applied(title, diff), nil, nil
}

func (tc *ToolContext) manageTags(ctx context.Context, _ *mcpsdk.CallToolRequest, args ManageTagsInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, "manage_tags", args.ID, args.MRN)
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
		return unchanged("the asset's tags are already as asked."), nil, nil
	}
	title := fmt.Sprintf("Update tags on **%s** (%s)", escapeMarkdown(deref(a.Name)), a.Type)
	var diffParts []string
	if len(toAdd) > 0 {
		diffParts = append(diffParts, "**Add:** "+strings.Join(escapeMarkdownAll(toAdd), ", "))
	}
	if len(toRemove) > 0 {
		diffParts = append(diffParts, "**Remove:** "+strings.Join(escapeMarkdownAll(toRemove), ", "))
	}
	diff := strings.Join(diffParts, "\n\n")
	if !args.Confirm {
		call := map[string]any{"id": idFor(a), "confirm": true}
		if len(toAdd) > 0 {
			call["add_tags"] = toAdd
		}
		if len(toRemove) > 0 {
			call["remove_tags"] = toRemove
		}
		return proposal(title, diff, "manage_tags", call), nil, nil
	}
	var done []string
	for _, t := range toAdd {
		if _, err := tc.assetService.AddTag(ctx, a.ID, t); err != nil {
			log.Warn().Err(err).Msg("MCP manage_tags add failed")
			return toolError(codeUnavailable, partialFailureDetail(fmt.Sprintf("adding %q", t), done)), nil, nil
		}
		done = append(done, fmt.Sprintf("added %q", t))
	}
	for _, t := range toRemove {
		if _, err := tc.assetService.RemoveTag(ctx, a.ID, t); err != nil {
			log.Warn().Err(err).Msg("MCP manage_tags remove failed")
			return toolError(codeUnavailable, partialFailureDetail(fmt.Sprintf("removing %q", t), done)), nil, nil
		}
		done = append(done, fmt.Sprintf("removed %q", t))
	}
	return applied(title, diff), nil, nil
}

// resolveOwner finds the team or user an owner change names, by ID or by name, so a preview shows who it is and an unknown owner is refused before anything is proposed.
func (tc *ToolContext) resolveOwner(ctx context.Context, ownerType, ref string) (id, name string, found bool) {
	_, isID := uuid.Parse(ref)
	if ownerType == "team" {
		if isID == nil {
			if t, err := tc.teamService.GetTeam(ctx, ref); err == nil && t != nil {
				return t.ID, t.Name, true
			}
		}
		if t, err := tc.teamService.GetTeamByName(ctx, ref); err == nil && t != nil {
			return t.ID, t.Name, true
		}
		return "", "", false
	}
	if tc.userService == nil {
		return "", "", false
	}
	if isID == nil {
		if u, err := tc.userService.Get(ctx, ref); err == nil && u != nil {
			return u.ID, u.Name, true
		}
	}
	if u, err := tc.userService.GetUserByUsername(ctx, ref); err == nil && u != nil {
		return u.ID, u.Name, true
	}
	return "", "", false
}

func (tc *ToolContext) manageOwners(ctx context.Context, _ *mcpsdk.CallToolRequest, args ManageOwnersInput) (*mcpsdk.CallToolResult, any, error) {
	if denied := tc.requireManage("assets"); denied != nil {
		return denied, nil, nil
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action != "add" && action != "remove" {
		return toolError(codeInvalidArgument, `action must be "add" or "remove".`), nil, nil
	}
	ownerType := strings.ToLower(strings.TrimSpace(args.OwnerType))
	if ownerType != "user" && ownerType != "team" {
		return toolError(codeInvalidArgument, `owner_type must be "user" or "team".`), nil, nil
	}
	ref := strings.TrimSpace(args.Owner)
	if ref == "" {
		ref = strings.TrimSpace(args.OwnerID)
	}
	if ref == "" {
		return toolError(codeInvalidArgument, "Give owner: the team's name or the user's username.", step{"list teams", "explore_teams", nil}), nil, nil
	}
	a, bad := tc.resolveWritableAsset(ctx, "manage_owners", args.ID, args.MRN)
	if bad != nil {
		return bad, nil, nil
	}
	ownerID, ownerName, ok := tc.resolveOwner(ctx, ownerType, ref)
	if !ok {
		next := []step{{"list teams", "explore_teams", nil}}
		if ownerType == "team" {
			if suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, ref, 5); len(suggestions) > 0 {
				next = nil
				for _, s := range suggestions {
					next = append(next, step{"team " + s, "manage_owners", map[string]any{"id": idFor(a), "action": action, "owner_type": ownerType, "owner": s}})
				}
			}
		}
		return toolError(codeNotFound, fmt.Sprintf("No %s %q.", ownerType, ref), next...), nil, nil
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
			return unchanged(fmt.Sprintf("%s already owns this asset.", ownerName)), nil, nil
		}
		if action == "remove" && !assigned {
			return unchanged(fmt.Sprintf("%s doesn't own this asset.", ownerName)), nil, nil
		}
	}

	title := fmt.Sprintf("%s %s **%s** as an owner %s **%s** (%s)",
		capitalise(action), ownerType, escapeMarkdown(ownerName), preposition(action), escapeMarkdown(deref(a.Name)), a.Type)
	if !args.Confirm {
		return proposal(title, "", "manage_owners", map[string]any{"id": idFor(a), "action": action, "owner_type": ownerType, "owner": ownerID, "confirm": true}), nil, nil
	}
	var err error
	if action == "add" {
		err = tc.teamService.AddAssetOwner(ctx, a.ID, ownerType, ownerID)
	} else {
		err = tc.teamService.RemoveAssetOwner(ctx, a.ID, ownerType, ownerID)
	}
	if err != nil {
		log.Warn().Err(err).Msg("MCP manage_owners failed")
		return unavailable("change the owners"), nil, nil
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
