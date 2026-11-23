package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// update_entity is the one tool that changes catalog metadata. Like create_knowledge it previews first and applies only with the preview_id that preview returned, which is signed over the caller, the exact change and the entity's version, so a change approved against one state can't land on another.

type UpdateEntityInput struct {
	ID            string   `json:"id" jsonschema:"The asset's MRN, ID or exact name, or a glossary term as term:<id> or its exact name"`
	Documentation *string  `json:"documentation,omitempty" jsonschema:"The full text to set as the asset's summary, in Markdown. An empty string clears it"`
	AddTags       []string `json:"add_tags,omitempty" jsonschema:"Tags to add to the asset"`
	RemoveTags    []string `json:"remove_tags,omitempty" jsonschema:"Tags to remove from the asset"`
	AddOwners     []string `json:"add_owners,omitempty" jsonschema:"Teams or people to add as owners, by team name or username"`
	RemoveOwners  []string `json:"remove_owners,omitempty" jsonschema:"Teams or people to remove as owners, by team name or username"`
	AddTerms      []string `json:"add_terms,omitempty" jsonschema:"Glossary terms to link to the asset, by name or ID"`
	RemoveTerms   []string `json:"remove_terms,omitempty" jsonschema:"Glossary terms to unlink from the asset, by name or ID"`
	Definition    *string  `json:"definition,omitempty" jsonschema:"For a glossary term: its new definition"`
	PreviewID     string   `json:"preview_id,omitempty" jsonschema:"Leave out to preview. After the user approves the preview, the preview_id it returned"`
}

type UpdateEntityOutput struct {
	Applied   bool     `json:"applied"`
	PreviewID string   `json:"preview_id,omitempty"`
	Changes   []string `json:"changes"`
	// Message is what to tell the user, the same text as the Markdown answer, for clients that show the model only this structured result.
	Message string `json:"message"`
}

type ownerRef struct{ kind, id, name string }

type termRef struct{ id, name string }

// entityPlan is the resolved change: what will be done to which entity, once every name is an ID.
type entityPlan struct {
	asset        *asset.Asset
	term         *glossary.GlossaryTerm
	version      string
	doc          *string
	addTags      []string
	removeTags   []string
	addOwners    []ownerRef
	removeOwners []ownerRef
	addTerms     []termRef
	removeTerms  []termRef
	definition   *string
	changes      []string
}

func (p *entityPlan) empty() bool { return len(p.changes) == 0 }

func (tc *ToolContext) updateEntity(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateEntityInput) (*mcpsdk.CallToolResult, UpdateEntityOutput, error) {
	if strings.TrimSpace(in.ID) == "" {
		return toolError(codeInvalidArgument, "Give id: the asset's MRN, ID or exact name, or term:<id> for a glossary term."), UpdateEntityOutput{}, nil
	}
	plan, bad := tc.planEntity(ctx, in)
	if bad != nil {
		return bad, UpdateEntityOutput{}, nil
	}
	if plan.empty() {
		return unchanged("it is already as asked, or the call asked for no change."), UpdateEntityOutput{Changes: []string{}, Message: "Nothing to change: it is already as asked, or the call asked for no change."}, nil
	}
	title := "an asset"
	if plan.asset != nil {
		title = fmt.Sprintf("%s (%s)", deref(plan.asset.Name), plan.asset.Type)
	} else if plan.term != nil {
		title = fmt.Sprintf("the glossary term %s", plan.term.Name)
	}

	if in.PreviewID == "" {
		id := tc.entityPreviewID(in, plan.version, time.Now().Add(previewTTL))
		var b strings.Builder
		fmt.Fprintf(&b, "Preview: nothing has changed yet.\n\nChanges to %s:", title)
		for _, c := range plan.changes {
			b.WriteString("\n- " + c)
		}
		fmt.Fprintf(&b, "\n\nShow this to the user. Only if they approve, call update_entity again with exactly the same arguments plus preview_id %q. It works for 30 minutes and only while %s is unchanged.", id, title)
		return textResult(b.String()), UpdateEntityOutput{PreviewID: id, Changes: plan.changes, Message: b.String()}, nil
	}
	if !tc.validEntityPreview(in, plan.version, in.PreviewID) {
		return toolError(codeInvalidArgument, "That preview_id doesn't match: the arguments differ from the preview, it has expired, or the entity changed since. Preview again without preview_id and show the user the new preview."), UpdateEntityOutput{}, nil
	}

	var done []string
	fail := func(what string, err error) (*mcpsdk.CallToolResult, UpdateEntityOutput, error) {
		log.Warn().Err(err).Msg("MCP update_entity " + what)
		return toolError(codeUnavailable, partialFailureDetail(what, done)), UpdateEntityOutput{Changes: done}, nil
	}
	by := ""
	if tc.principal != nil {
		by = tc.principal.ID()
	}
	if a := plan.asset; a != nil {
		if plan.doc != nil {
			if _, err := tc.assetService.Update(ctx, a.ID, asset.UpdateInput{UserDescription: plan.doc}); err != nil {
				return fail("saving the summary", err)
			}
			done = append(done, "summary saved")
		}
		for _, t := range plan.addTags {
			if _, err := tc.assetService.AddTag(ctx, a.ID, t); err != nil {
				return fail(fmt.Sprintf("adding tag %q", t), err)
			}
			done = append(done, fmt.Sprintf("added tag %q", t))
		}
		for _, t := range plan.removeTags {
			if _, err := tc.assetService.RemoveTag(ctx, a.ID, t); err != nil {
				return fail(fmt.Sprintf("removing tag %q", t), err)
			}
			done = append(done, fmt.Sprintf("removed tag %q", t))
		}
		for _, o := range plan.addOwners {
			if err := tc.teamService.AddAssetOwner(ctx, a.ID, o.kind, o.id); err != nil {
				return fail("adding owner "+o.name, err)
			}
			done = append(done, "added owner "+o.name)
		}
		for _, o := range plan.removeOwners {
			if err := tc.teamService.RemoveAssetOwner(ctx, a.ID, o.kind, o.id); err != nil {
				return fail("removing owner "+o.name, err)
			}
			done = append(done, "removed owner "+o.name)
		}
		if len(plan.addTerms) > 0 {
			ids := make([]string, len(plan.addTerms))
			for i, t := range plan.addTerms {
				ids[i] = t.id
			}
			if err := tc.assetService.AddTerms(ctx, a.ID, ids, "user", by); err != nil {
				return fail("linking terms", err)
			}
			done = append(done, fmt.Sprintf("linked %d terms", len(ids)))
		}
		for _, t := range plan.removeTerms {
			if err := tc.assetService.RemoveTerm(ctx, a.ID, t.id); err != nil {
				return fail("unlinking term "+t.name, err)
			}
			done = append(done, "unlinked term "+t.name)
		}
	}
	if t := plan.term; t != nil && plan.definition != nil {
		if _, err := tc.glossaryService.Update(ctx, t.ID, glossary.UpdateTermInput{Definition: plan.definition}); err != nil {
			return fail("saving the definition", err)
		}
		done = append(done, "definition saved")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Applied to %s:", title)
	for _, c := range plan.changes {
		b.WriteString("\n- " + c)
	}
	return textResult(b.String()), UpdateEntityOutput{Applied: true, Changes: plan.changes, Message: b.String()}, nil
}

// planEntity resolves the target and every name in the request, refusing anything unknown or not permitted before a preview is shown.
func (tc *ToolContext) planEntity(ctx context.Context, in UpdateEntityInput) (*entityPlan, *mcpsdk.CallToolResult) {
	plan := &entityPlan{}
	ref := strings.TrimSpace(in.ID)
	assetChange := in.Documentation != nil || len(in.AddTags)+len(in.RemoveTags)+len(in.AddOwners)+len(in.RemoveOwners)+len(in.AddTerms)+len(in.RemoveTerms) > 0

	if strings.HasPrefix(ref, kindTerm+":") || (in.Definition != nil && !assetChange) {
		if assetChange {
			return nil, toolError(codeInvalidArgument, "A glossary term only takes definition here. To link a term to an asset, give the asset's id and add_terms.")
		}
		if denied := tc.requireManage("glossary"); denied != nil {
			return nil, denied
		}
		t, err := tc.findTerm(ctx, strings.TrimPrefix(ref, kindTerm+":"))
		if err != nil {
			log.Warn().Err(err).Msg("MCP update_entity term lookup failed")
			return nil, unavailable("look that term up")
		}
		if t == nil {
			return nil, toolError(codeNotFound, fmt.Sprintf("No glossary term %q.", ref), step{"search terms", "search", map[string]any{"query": strings.TrimPrefix(ref, kindTerm+":"), "kinds": []string{kindTerm}}})
		}
		plan.term, plan.version = t, strconv.FormatInt(t.UpdatedAt.UnixNano(), 10)
		if in.Definition != nil {
			def := strings.TrimSpace(*in.Definition)
			if def == "" {
				return nil, toolError(codeInvalidArgument, "A term's definition can't be empty.")
			}
			if def != t.Definition {
				plan.definition = &def
				plan.changes = append(plan.changes, fmt.Sprintf("definition: %q -> %q", truncate(t.Definition, 300), truncate(def, 300)))
			}
		}
		return plan, nil
	}

	if in.Definition != nil {
		return nil, toolError(codeInvalidArgument, "definition is for glossary terms. Give id as term:<id> or the term's name, with no asset changes.")
	}
	if denied := tc.requireManage("assets"); denied != nil {
		return nil, denied
	}
	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Msg("MCP update_entity lookup failed")
		return nil, unavailable("look that asset up")
	}
	if a == nil {
		return nil, assetNotFound("update_entity", "id", ref, candidates)
	}
	plan.asset, plan.version = a, strconv.FormatInt(a.UpdatedAt.UnixNano(), 10)

	if in.Documentation != nil {
		doc := strings.TrimSpace(*in.Documentation)
		if secret := findSecret(doc); secret != "" {
			return nil, toolError(codeInvalidArgument, fmt.Sprintf("The summary looks like it contains %s. Remove it; asset summaries are visible to the whole company.", secret))
		}
		if doc != deref(a.UserDescription) {
			plan.doc = &doc
			if doc == "" {
				plan.changes = append(plan.changes, "clear the summary")
			} else {
				plan.changes = append(plan.changes, fmt.Sprintf("summary: %q -> %q", truncate(deref(a.UserDescription), 300), truncate(doc, 600)))
			}
		}
	}
	has := map[string]bool{}
	for _, t := range a.Tags {
		has[t] = true
	}
	for _, t := range in.AddTags {
		if t = strings.TrimSpace(t); t != "" && !has[t] {
			plan.addTags = append(plan.addTags, t)
			has[t] = true
			plan.changes = append(plan.changes, fmt.Sprintf("add tag %q", t))
		}
	}
	for _, t := range in.RemoveTags {
		if t = strings.TrimSpace(t); t != "" && has[t] {
			plan.removeTags = append(plan.removeTags, t)
			delete(has, t)
			plan.changes = append(plan.changes, fmt.Sprintf("remove tag %q", t))
		}
	}

	if len(in.AddOwners)+len(in.RemoveOwners) > 0 {
		current, err := tc.teamService.ListAssetOwners(ctx, a.ID)
		if err != nil {
			log.Warn().Err(err).Msg("MCP update_entity owners failed")
			return nil, unavailable("read the owners")
		}
		owns := map[string]bool{}
		for _, o := range current {
			owns[o.Type+":"+o.ID] = true
		}
		for _, name := range in.AddOwners {
			o, bad := tc.ownerByName(ctx, name)
			if bad != nil {
				return nil, bad
			}
			if !owns[o.kind+":"+o.id] {
				owns[o.kind+":"+o.id] = true
				plan.addOwners = append(plan.addOwners, o)
				plan.changes = append(plan.changes, fmt.Sprintf("add owner %s (%s)", o.name, o.kind))
			}
		}
		for _, name := range in.RemoveOwners {
			o, bad := tc.ownerByName(ctx, name)
			if bad != nil {
				return nil, bad
			}
			if owns[o.kind+":"+o.id] {
				delete(owns, o.kind+":"+o.id)
				plan.removeOwners = append(plan.removeOwners, o)
				plan.changes = append(plan.changes, fmt.Sprintf("remove owner %s (%s)", o.name, o.kind))
			}
		}
	}

	if len(in.AddTerms)+len(in.RemoveTerms) > 0 {
		linked := map[string]bool{}
		if terms, err := tc.assetService.GetTerms(ctx, a.ID); err == nil {
			for _, t := range terms {
				linked[t.TermID] = true
			}
		}
		for _, name := range in.AddTerms {
			t, bad := tc.termByName(ctx, name)
			if bad != nil {
				return nil, bad
			}
			if !linked[t.id] {
				linked[t.id] = true
				plan.addTerms = append(plan.addTerms, t)
				plan.changes = append(plan.changes, "link term "+t.name)
			}
		}
		for _, name := range in.RemoveTerms {
			t, bad := tc.termByName(ctx, name)
			if bad != nil {
				return nil, bad
			}
			if linked[t.id] {
				delete(linked, t.id)
				plan.removeTerms = append(plan.removeTerms, t)
				plan.changes = append(plan.changes, "unlink term "+t.name)
			}
		}
	}
	return plan, nil
}

// ownerByName finds a team or person by name, team first, taking "team:" or "user:" to say which.
func (tc *ToolContext) ownerByName(ctx context.Context, ref string) (ownerRef, *mcpsdk.CallToolResult) {
	ref = strings.TrimSpace(ref)
	kinds := []string{"team", "user"}
	if k, rest, ok := strings.Cut(ref, ":"); ok && (k == "team" || k == "user") {
		kinds, ref = []string{k}, rest
	}
	for _, k := range kinds {
		if id, name, ok := tc.resolveOwner(ctx, k, ref); ok {
			return ownerRef{kind: k, id: id, name: name}, nil
		}
	}
	var next []step
	if suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, ref, 3); len(suggestions) > 0 {
		for _, s := range suggestions {
			next = append(next, step{"team " + s, "search", map[string]any{"query": s, "kinds": []string{kindTeam}}})
		}
	} else {
		next = append(next, step{"list teams", "search", map[string]any{"kinds": []string{kindTeam}}})
	}
	return ownerRef{}, toolError(codeNotFound, fmt.Sprintf("No team or person %q. Use a team's name or a person's username.", ref), next...)
}

func (tc *ToolContext) termByName(ctx context.Context, ref string) (termRef, *mcpsdk.CallToolResult) {
	t, err := tc.findTerm(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Msg("MCP update_entity term lookup failed")
		return termRef{}, unavailable("look that term up")
	}
	if t == nil {
		return termRef{}, toolError(codeNotFound, fmt.Sprintf("No glossary term %q.", ref), step{"search terms", "search", map[string]any{"query": ref, "kinds": []string{kindTerm}}})
	}
	return termRef{id: t.ID, name: t.Name}, nil
}

// findTerm reads a term by ID or exact name.
func (tc *ToolContext) findTerm(ctx context.Context, ref string) (*glossary.GlossaryTerm, error) {
	ref = strings.TrimSpace(ref)
	if _, err := uuid.Parse(ref); err == nil {
		t, err := tc.glossaryService.Get(ctx, ref)
		if errors.Is(err, glossary.ErrTermNotFound) {
			return nil, nil
		}
		return t, err
	}
	res, err := tc.glossaryService.Search(ctx, glossary.SearchFilter{Query: ref, Limit: 10})
	if err != nil {
		return nil, err
	}
	for _, t := range res.Terms {
		if strings.EqualFold(t.Name, ref) {
			return t, nil
		}
	}
	return nil, nil
}

func (tc *ToolContext) entityPreviewID(in UpdateEntityInput, version string, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + tc.signEntity(in, version, exp)
}

func (tc *ToolContext) validEntityPreview(in UpdateEntityInput, version, id string) bool {
	exp, sig, ok := strings.Cut(id, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().After(time.Unix(unix, 0)) {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(tc.signEntity(in, version, exp)))
}

func (tc *ToolContext) signEntity(in UpdateEntityInput, version, exp string) string {
	in.PreviewID = ""
	payload, _ := json.Marshal(in)
	mac := hmac.New(sha256.New, tc.previewKey)
	who := ""
	if tc.principal != nil {
		who = tc.principal.ID()
	}
	mac.Write([]byte("update_entity\n" + who + "\n" + exp + "\n" + version + "\n"))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
