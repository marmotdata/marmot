package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/plugin-sdk/mrn"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Saving knowledge is the one write an assistant may suggest on its own, so it is built so a person always decides: the first call only previews, the save needs the handle that preview returned for exactly the same content, and what is saved starts as a draft that someone sets live in Marmot. Secrets are refused outright, because an entry is readable by the whole company.

const createKnowledgeDescription = `Save something to Marmot's company knowledge as a draft entry: a runbook, decision, policy, incident write-up, guide, reference to an outside document, fact, writing style, brand voice or design system that people in the company should be able to find and follow.

Use it only when the user asks to save something, or says yes after you offer. Offering is fine when the conversation produced something reusable, such as the steps that fixed an incident or a decision with its reasons, at most once per task and in one short line. Never save secrets, credentials, personal data about individuals or one-off details.

Call it first without preview_id: nothing is saved, and it returns a preview and a preview_id. Show the user the preview and ask whether to save it. Only after they say yes, call again with the same arguments plus that preview_id. Changing anything needs a new preview. The entry is saved as a draft for a person to review and set live in Marmot.`

// CreateKnowledgeInput is the input of create_knowledge.
type CreateKnowledgeInput struct {
	Kind      string         `json:"kind" jsonschema:"One of: runbook, decision, policy, incident, guide, reference, fact, writing-style, brand-voice, design-system"`
	Title     string         `json:"title" jsonschema:"A short title people would search for, such as 'Restart a stuck Flux sync'"`
	Summary   string         `json:"summary" jsonschema:"One sentence saying what the entry is for"`
	Writeup   string         `json:"writeup" jsonschema:"The body in Markdown: steps, context, rules or examples"`
	Fields    map[string]any `json:"fields,omitempty" jsonschema:"Typed fields for the kind, such as a runbook's triggers (list of strings) or a decision's decided_at (YYYY-MM-DD). Optional"`
	AppliesTo []string       `json:"applies_to,omitempty" jsonschema:"MRNs, IDs or exact names of the assets it applies to, such as a repository, service or cluster"`
	PreviewID string         `json:"preview_id,omitempty" jsonschema:"Leave out to preview. After the user approves the preview, the preview_id it returned"`
}

// previewTTL bounds how long a person has to approve a preview, so an old approval can't be replayed much later.
const previewTTL = 30 * time.Minute

func (s *Server) registerKnowledgeWriteTools(server *mcpsdk.Server, tc *ToolContext) {
	// Only callers who could save are offered the tool, so assistants for read-only users never suggest saving.
	if !s.canSaveKnowledge(tc.principal) {
		return
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "create_knowledge",
		Title:       "Save company knowledge (draft)",
		Description: createKnowledgeDescription,
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPtr(false), IdempotentHint: false, OpenWorldHint: boolPtr(false)},
	}, tc.createKnowledge)
}

func (tc *ToolContext) createKnowledge(ctx context.Context, _ *mcpsdk.CallToolRequest, args CreateKnowledgeInput) (*mcpsdk.CallToolResult, any, error) {
	if refusal := tc.requireManage("assets"); refusal != nil {
		return refusal, nil, nil
	}
	plan, problems := tc.planKnowledge(ctx, args)
	if len(problems) > 0 {
		return textResult("Nothing was saved. Fix these and preview again:\n- " + strings.Join(problems, "\n- ")), nil, nil
	}

	if args.PreviewID == "" {
		return textResult(tc.renderPreview(args, plan)), nil, nil
	}
	if !tc.validPreview(args, args.PreviewID) {
		return textResult("Nothing was saved. That preview_id doesn't match these arguments or has expired. Call create_knowledge without preview_id, show the user the new preview and ask again."), nil, nil
	}
	return tc.saveKnowledge(ctx, args, plan)
}

// knowledgePlan is everything a save will write, worked out once for both the preview and the save.
type knowledgePlan struct {
	kind    knowledge.Kind
	status  string
	fields  map[string]any
	targets []*asset.Asset
	mrn     string
	notes   []string
}

func (tc *ToolContext) planKnowledge(ctx context.Context, args CreateKnowledgeInput) (knowledgePlan, []string) {
	var plan knowledgePlan
	var problems []string

	ids, bad := normaliseKinds([]string{args.Kind})
	if len(bad) > 0 || len(ids) == 0 {
		return plan, []string{fmt.Sprintf("kind %q is not one of %s", args.Kind, strings.Join(kindIDs(), ", "))}
	}
	plan.kind, _ = knowledge.KindByID(ids[0])
	plan.status = plan.kind.DraftStatus()

	title := strings.TrimSpace(args.Title)
	if title == "" {
		problems = append(problems, "title is empty")
	}
	if strings.TrimSpace(args.Summary) == "" {
		problems = append(problems, "summary is empty; give one sentence saying what the entry is for")
	}
	if strings.TrimSpace(args.Writeup) == "" {
		problems = append(problems, "writeup is empty")
	}
	for _, f := range []struct{ name, text string }{{"title", args.Title}, {"summary", args.Summary}, {"writeup", args.Writeup}, {"fields", fieldsText(args.Fields)}} {
		if what := findSecret(f.text); what != "" {
			problems = append(problems, fmt.Sprintf("the %s looks like it holds %s. Entries are readable by the whole company, so leave secrets out and say where to get them instead (for example, \"from the secret manager\")", f.name, what))
		}
	}

	fields, err := knowledge.CheckFields(plan.kind, plan.status, args.Fields)
	if err != nil {
		problems = append(problems, strings.TrimPrefix(err.Error(), knowledge.ErrInvalid.Error()+": "))
	}
	plan.fields = fields

	for _, ref := range args.AppliesTo {
		a, candidates, err := tc.resolveAsset(ctx, ref)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("couldn't look up %q just now; retry", ref))
		// An unknown asset shouldn't block saving what the user approved, so it is left out and the preview says so.
		case a == nil && len(candidates) > 0:
			plan.notes = append(plan.notes, fmt.Sprintf("Not linked to %q, which matches no asset exactly. To link it, preview again with one of these MRNs in applies_to:\n%s", ref, strings.TrimRight(formatCandidates(candidates), "\n")))
		case a == nil:
			plan.notes = append(plan.notes, fmt.Sprintf("Not linked to %q: Marmot has no asset by that name.", ref))
		default:
			plan.targets = append(plan.targets, a)
		}
	}

	if title != "" {
		plan.mrn = mrn.Asset(plan.kind.Type, knowledge.Provider, title).String()
		if existing, err := tc.assetService.GetByMRN(ctx, plan.mrn); err == nil && existing != nil {
			problems = append(problems, fmt.Sprintf("an entry called %q already exists (id %s). Read it with read_knowledge and suggest the change to the user, who can edit it in Marmot, rather than saving a second copy", deref(existing.Name), existing.ID))
		}
		if items, _, err := tc.knowledgeService.List(ctx, knowledge.ListFilter{Kinds: []string{plan.kind.ID}, Query: title, AnyWord: true, Limit: 3}); err == nil {
			for _, it := range items {
				if !strings.EqualFold(it.Name, title) && similarTitles(it.Name, title) {
					plan.notes = append(plan.notes, fmt.Sprintf("A similar entry exists: %s (id %s). Check it isn't the same thing.", it.Name, it.AssetID))
				}
			}
		}
	}
	if emailRe.MatchString(args.Writeup + " " + args.Summary + " " + fieldsText(args.Fields)) {
		plan.notes = append(plan.notes, "It contains an email address. Check the user is happy for the whole company to see it.")
	}
	return plan, problems
}

func (tc *ToolContext) renderPreview(args CreateKnowledgeInput, plan knowledgePlan) string {
	var b strings.Builder
	b.WriteString("Preview only. Nothing has been saved.\n\n")
	fmt.Fprintf(&b, "# %s\n\n%s · saved as %s, for someone to review and set live in Marmot\n\n%s\n", strings.TrimSpace(args.Title), plan.kind.Label, statusLabel(plan.kind, plan.status), strings.TrimSpace(args.Summary))
	writeFields(&b, plan.kind, plan.fields)
	if len(plan.targets) > 0 {
		b.WriteString("\n## Applies to\n\n")
		for _, a := range plan.targets {
			fmt.Fprintf(&b, "- %s (%s)\n", deref(a.Name), a.Type)
		}
	}
	fmt.Fprintf(&b, "\n## Write-up\n\n%s\n", strings.TrimSpace(args.Writeup))
	if tc.principal.Type() == "user" {
		fmt.Fprintf(&b, "\nOwner: %s. Everyone in the company can read it.\n", tc.principal.DisplayName())
	} else {
		b.WriteString("\nNo owner is set, because this identity isn't a person; set one in Marmot. Everyone in the company can read it.\n")
	}
	for _, n := range plan.notes {
		fmt.Fprintf(&b, "\nNote: %s\n", n)
	}
	id := tc.previewID(args, time.Now().Add(previewTTL))
	fmt.Fprintf(&b, "\n---\nShow this preview to the user and ask whether to save it. Only if they say yes, call create_knowledge again with the same arguments and \"preview_id\": %q. It expires in 30 minutes.\n", id)
	return b.String()
}

func (tc *ToolContext) saveKnowledge(ctx context.Context, args CreateKnowledgeInput, plan knowledgePlan) (*mcpsdk.CallToolResult, any, error) {
	title := strings.TrimSpace(args.Title)
	summary := strings.TrimSpace(args.Summary)
	created, err := tc.assetService.Create(ctx, asset.CreateInput{
		Name:      &title,
		MRN:       &plan.mrn,
		Type:      plan.kind.Type,
		Providers: []string{knowledge.Provider},
		CreatedBy: tc.principal.DisplayName(),
		Writer:    "mcp:" + string(tc.principal.Type()) + ":" + tc.principal.ID(),
		Role:      asset.RoleManual,
	})
	if errors.Is(err, asset.ErrAlreadyExists) {
		return textResult("Nothing was saved: an entry with this title was created in the meantime. Read it with read_knowledge instead."), nil, nil
	}
	if err != nil {
		return textResult("Nothing was saved: Marmot couldn't create the entry just now. Retry the same call."), nil, nil
	}
	// A half-made entry would show up in search without its body, so undo the asset when a later step fails.
	undo := func(step string) (*mcpsdk.CallToolResult, any, error) {
		_ = tc.assetService.Delete(ctx, created.ID)
		return textResult(fmt.Sprintf("Nothing was saved: Marmot couldn't %s. Retry the same call.", step)), nil, nil
	}
	if _, err := tc.assetService.Update(ctx, created.ID, asset.UpdateInput{UserDescription: &summary, SkipNotification: true}); err != nil {
		return undo("set the summary")
	}
	links := make([]knowledge.LinkInput, 0, len(plan.targets))
	for _, a := range plan.targets {
		links = append(links, knowledge.LinkInput{Relation: knowledge.RelationAppliesTo, TargetID: a.ID})
	}
	if _, err := tc.knowledgeService.Save(ctx, created.ID, knowledge.SaveInput{Status: plan.status, Fields: plan.fields, Links: &links}, tc.principal.DisplayName()); err != nil {
		return undo("save the entry's fields and links")
	}
	body := strings.TrimSpace(args.Writeup) + "\n"
	var by *string
	if tc.principal.Type() == "user" {
		id := tc.principal.ID()
		by = &id
	}
	if _, err := tc.docsService.CreatePage(ctx, docs.EntityTypeAsset, plan.mrn, docs.CreatePageInput{Title: title, Content: &body}, by); err != nil {
		return undo("save the write-up")
	}
	if tc.principal.Type() == "user" && tc.teamService != nil {
		_ = tc.teamService.AddAssetOwner(ctx, created.ID, "user", tc.principal.ID())
	}

	msg := fmt.Sprintf("Saved %q as a %s %s (id %s). It isn't live until someone reviews it and sets it to %s in Marmot.", title, statusLabel(plan.kind, plan.status), strings.ToLower(plan.kind.Label), created.ID, statusLabel(plan.kind, plan.kind.DefaultStatus))
	if link := entryURL(tc.config.Server.RootURL, created.ID); link != "" {
		msg += " Open it at " + link
	}
	return textResult(msg), nil, nil
}

// previewID signs the arguments, the caller and an expiry, so a save goes through only for content a preview showed, for the same caller, within the time allowed.
func (tc *ToolContext) previewID(args CreateKnowledgeInput, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + tc.signPreview(args, exp)
}

func (tc *ToolContext) validPreview(args CreateKnowledgeInput, id string) bool {
	exp, sig, ok := strings.Cut(id, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().After(time.Unix(unix, 0)) {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(tc.signPreview(args, exp)))
}

func (tc *ToolContext) signPreview(args CreateKnowledgeInput, exp string) string {
	args.PreviewID = ""
	payload, _ := json.Marshal(args)
	mac := hmac.New(sha256.New, tc.previewKey)
	mac.Write([]byte(tc.principal.ID() + "\n" + exp + "\n"))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var (
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	secretRes = []struct {
		what string
		re   *regexp.Regexp
	}{
		{"a private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
		{"an AWS access key", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
		{"a GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}`)},
		{"a Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
		{"a Marmot API key", regexp.MustCompile(`\bmrmt_[A-Za-z0-9_+/=-]{20,}`)},
		{"an API key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
		{"a Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
		{"a token", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
		{"a password or key value", regexp.MustCompile(`(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token|client[_-]?secret)\s*[:=]\s*["']?[^\s"'<>${}]{8,}`)},
		{"a connection string with a password", regexp.MustCompile(`[a-z][a-z0-9+.-]*://[^\s:/@]+:[^\s@<>${}]{3,}@`)},
	}
)

// findSecret names the kind of secret text appears to contain, or returns "" when it looks clean. Placeholders such as <password> and ${PG_PASSWORD} are allowed.
func findSecret(text string) string {
	for _, s := range secretRes {
		if s.re.MatchString(text) {
			return s.what
		}
	}
	return ""
}

func fieldsText(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// canSaveKnowledge reports whether create_knowledge is listed for this caller.
func (s *Server) canSaveKnowledge(p auth.Principal) bool {
	return s.knowledgeService != nil && s.docsService != nil && p != nil && (p.IsAdmin() || p.HasPermission("assets", "manage"))
}

// similarTitles reports whether two titles share at least half of the longer one's meaningful words, so the duplicate warning fires for near copies rather than for any shared word.
func similarTitles(a, b string) bool {
	words := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			if len(w) > 3 {
				out[w] = true
			}
		}
		return out
	}
	wa, wb := words(a), words(b)
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	most := max(len(wa), len(wb))
	return most > 0 && shared*2 >= most
}
