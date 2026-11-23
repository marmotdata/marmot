package knowledge

import "strings"

// FieldType says how a kind's field is entered, checked and shown.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldLongText FieldType = "long_text"
	FieldDate     FieldType = "date"
	FieldSelect   FieldType = "select"
	FieldURL      FieldType = "url"
	FieldList     FieldType = "list"
	FieldTable    FieldType = "table"
)

// State is what a status means to someone relying on an entry: a pending entry is not yet binding (a draft, under review, or not yet in force), a current one is, and a historical one no longer is.
type State string

const (
	StatePending    State = "pending"
	StateCurrent    State = "current"
	StateHistorical State = "historical"
)

// Option is one choice of a select field.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
} // @name KnowledgeOption

// Field is one typed field of a kind. A table field holds rows whose cells are described by Columns, which may only be text, long text, select or URL fields.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Help        string    `json:"help,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	// RequiredWhen lists statuses in which the field becomes required, such as a decision's date once it is accepted.
	RequiredWhen []string `json:"required_when,omitempty"`
	// MaxRows caps a table field below the general limit.
	MaxRows int      `json:"max_rows,omitempty"`
	Options []Option `json:"options,omitempty"`
	Columns []Field  `json:"columns,omitempty"`
} // @name KnowledgeField

// Status is one status a kind's entries can have and the state it stands for.
type Status struct {
	Value string `json:"value"`
	Label string `json:"label"`
	State State  `json:"state"`
} // @name KnowledgeStatus

// Kind is a kind of knowledge entry. An entry is an asset whose type is the kind's Type, with its typed fields kept beside the asset and its free text in documentation pages, so the template seeds the first page.
type Kind struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Label         string   `json:"label"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Statuses      []Status `json:"statuses"`
	DefaultStatus string   `json:"default_status"`
	Fields        []Field  `json:"fields"`
	Template      string   `json:"template,omitempty"`
} // @name KnowledgeKind

// Provider is the provider every entry written in Marmot carries, since an asset needs one and the entry lives nowhere else.
const Provider = "Marmot"

// lifecycle is the status vocabulary of every kind but decisions.
var lifecycle = []Status{
	{Value: "draft", Label: "Draft", State: StatePending},
	{Value: "in_review", Label: "In review", State: StatePending},
	{Value: "active", Label: "Active", State: StateCurrent},
	{Value: "superseded", Label: "Superseded", State: StateHistorical},
	{Value: "retired", Label: "Retired", State: StateHistorical},
}

var kinds = []Kind{
	{
		ID:            "policy",
		Type:          "Policy",
		Label:         "Policy",
		Description:   "A rule people are expected to follow, who it applies to and who approves exceptions.",
		Icon:          "policy",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "audience", Label: "Applies to", Type: FieldText, Placeholder: "Everyone", Help: "Who has to follow it, such as a team, a role or a region. Leave it empty if it applies to everyone."},
			{Key: "approver", Label: "Approved by", Type: FieldText, Placeholder: "Head of Finance", Help: "The person or role that signed it off."},
			{Key: "approved_at", Label: "Approved on", Type: FieldDate},
			{Key: "exceptions_contact", Label: "Exceptions", Type: FieldText, Placeholder: "finance@example.com", Help: "Who to ask for an exception."},
		},
		Template: "## What the policy says\n\nState the rule in one or two sentences.\n\n## What to do\n\n- \n\n## Why it exists\n",
	},
	{
		ID:          "decision",
		Type:        "Decision",
		Label:       "Decision",
		Description: "What was decided, when and by whom, and what it replaced. Rejected decisions stay findable so nobody proposes them again.",
		Icon:        "flag",
		Statuses: []Status{
			{Value: "proposed", Label: "Proposed", State: StatePending},
			{Value: "accepted", Label: "Accepted", State: StateCurrent},
			{Value: "rejected", Label: "Rejected", State: StateHistorical},
			{Value: "superseded", Label: "Superseded", State: StateHistorical},
			{Value: "deprecated", Label: "Deprecated", State: StateHistorical},
		},
		DefaultStatus: "accepted",
		Fields: []Field{
			{Key: "decided_at", Label: "Decided on", Type: FieldDate, RequiredWhen: []string{"accepted", "rejected"}},
			{Key: "deciders", Label: "Decided by", Type: FieldList, Help: "The people or roles who made the call."},
		},
		Template: "## Context\n\nThe problem and the constraints at the time.\n\n## Options considered\n\n- \n\n## Consequences\n\nWhat changes as a result, good and bad.\n",
	},
	{
		ID:            "runbook",
		Type:          "Runbook",
		Label:         "Runbook",
		Description:   "The steps for a task people do again and again, and the alerts or symptoms that call for it.",
		Icon:          "checklist",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "triggers", Label: "Use it when", Type: FieldList, Help: "Alert names or symptoms that call for this runbook, one per line."},
			{Key: "escalation_contact", Label: "Escalate to", Type: FieldText, Placeholder: "#data-platform on-call", Help: "Who to contact when the steps do not work."},
		},
		Template: "## Before you start\n\n- \n\n## Steps\n\n1. \n\n## How to tell it worked\n\n## If it goes wrong\n",
	},
	{
		ID:          "incident",
		Type:        "Incident",
		Label:       "Incident",
		Description: "The record of something that went wrong: what happened, why and what changed after. Live response stays in your incident tool, and Marmot keeps what was learned.",
		Icon:        "report",
		// The values are the shared lifecycle so they rank the same everywhere; only the label for an agreed write-up differs.
		Statuses: []Status{
			{Value: "draft", Label: "Draft", State: StatePending},
			{Value: "in_review", Label: "In review", State: StatePending},
			{Value: "active", Label: "Published", State: StateCurrent},
			{Value: "superseded", Label: "Superseded", State: StateHistorical},
			{Value: "retired", Label: "Retired", State: StateHistorical},
		},
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "started_at", Label: "Started on", Type: FieldDate, RequiredWhen: []string{"active"}},
			{Key: "resolved_at", Label: "Resolved on", Type: FieldDate},
			{Key: "severity", Label: "Severity", Type: FieldSelect, Options: []Option{
				{Value: "sev1", Label: "SEV1: critical"},
				{Value: "sev2", Label: "SEV2: major"},
				{Value: "sev3", Label: "SEV3: minor"},
				{Value: "sev4", Label: "SEV4: low"},
			}},
			{Key: "incident_url", Label: "Incident record", Type: FieldURL, Help: "The incident in your paging or incident tool, such as PagerDuty, incident.io or Jira."},
			{Key: "follow_ups", Label: "Follow-ups", Type: FieldTable, MaxRows: 20, Help: "What was agreed to stop it happening again.", Columns: []Field{
				{Key: "action", Label: "Action", Type: FieldText, Placeholder: "Alert on consumer lag"},
				{Key: "owner", Label: "Owner", Type: FieldText, Placeholder: "Data platform"},
				{Key: "status", Label: "Status", Type: FieldSelect, Options: []Option{
					{Value: "open", Label: "Open"},
					{Value: "done", Label: "Done"},
					{Value: "wont_do", Label: "Won't do"},
				}},
				{Key: "url", Label: "Ticket", Type: FieldURL},
			}},
		},
		Template: "## Summary\n\nWhat happened and who it affected, in two or three sentences.\n\n## Timeline\n\n- \n\n## Cause\n\n## What went well\n\n## What we'll change\n",
	},
	{
		ID:            "guide",
		Type:          "Guide",
		Label:         "Guide",
		Description:   "An explanation of how something works or how to get something done, for anything that isn't a rule, a decision or an operational runbook.",
		Icon:          "menu-book",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "audience", Label: "Written for", Type: FieldText, Placeholder: "New engineers", Help: "Who should read it. Leave it empty if it's for everyone."},
		},
		Template: "## Overview\n\nWhat this explains and when you'd need it.\n\n## How it works\n\n## Common questions\n",
	},
	{
		ID:            "reference",
		Type:          "Reference",
		Label:         "Reference",
		Description:   "A document that lives somewhere else, such as a wiki page, a shared doc or a vendor's documentation, with what it covers and when to read it.",
		Icon:          "link",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "url", Label: "Link", Type: FieldURL, Required: true, Help: "Where the document lives. It stays the source of truth, and assistants are sent to it."},
			{Key: "location", Label: "Lives in", Type: FieldSelect, Options: []Option{
				{Value: "confluence", Label: "Confluence"},
				{Value: "google-drive", Label: "Google Drive"},
				{Value: "notion", Label: "Notion"},
				{Value: "sharepoint", Label: "SharePoint"},
				{Value: "github", Label: "GitHub"},
				{Value: "vendor", Label: "Vendor documentation"},
				{Value: "website", Label: "Website"},
				{Value: "other", Label: "Other"},
			}},
			{Key: "access", Label: "Who can open it", Type: FieldText, Placeholder: "Anyone at the company", Help: "So people know whether to ask for access before they follow the link."},
		},
		Template: "## What it covers\n\n## When to read it\n",
	},
	{
		ID:            "fact",
		Type:          "Fact",
		Label:         "Fact",
		Description:   "One short thing that is true about the company, such as how a system behaves or who handles a request, with where it came from.",
		Icon:          "push-pin",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "question", Label: "Answers", Type: FieldText, Placeholder: "When is the staging database reset?", Help: "The question someone would ask to need this fact, written the way they'd ask it."},
			{Key: "as_of", Label: "True as of", Type: FieldDate, Help: "When it was last known to be true."},
			{Key: "sources", Label: "Sources", Type: FieldTable, MaxRows: 10, Help: "Where it came from, such as a Slack thread, a document or a commit.", Columns: []Field{
				{Key: "url", Label: "Link", Type: FieldURL},
				{Key: "label", Label: "Label", Type: FieldText, Placeholder: "#platform thread"},
			}},
		},
		Template: "## Details\n\nAnything that qualifies it, such as where it doesn't hold.\n",
	},
	{
		ID:            "brand-voice",
		Type:          "Brand Voice",
		Label:         "Brand voice",
		Description:   "How the company sounds: a few traits, and real samples for people and assistants to imitate.",
		Icon:          "forum",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "traits", Label: "Traits", Type: FieldTable, MaxRows: 6, Help: "Three or four traits, each with what it is not.", Columns: []Field{
				{Key: "trait", Label: "We are", Type: FieldText, Placeholder: "Direct"},
				{Key: "not", Label: "But not", Type: FieldText, Placeholder: "Blunt"},
			}},
			{Key: "samples", Label: "Samples", Type: FieldTable, MaxRows: 10, Help: "Three to five real passages written the way you want. Assistants imitate samples better than descriptions.", Columns: []Field{
				{Key: "audience", Label: "Audience or channel", Type: FieldText, Placeholder: "Support replies"},
				{Key: "text", Label: "Sample", Type: FieldLongText},
			}},
		},
		Template: "## How we sound\n\nWhat each trait means in practice, and how the tone shifts by audience.\n",
	},
	{
		ID:            "writing-style",
		Type:          "Writing Style",
		Label:         "Writing style",
		Description:   "The rules writing can be checked against: which English, which guide, and the terms to use and avoid.",
		Icon:          "match-case",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "locale", Label: "Language", Type: FieldSelect, Options: []Option{
				{Value: "en-GB", Label: "British English"},
				{Value: "en-US", Label: "American English"},
				{Value: "en-AU", Label: "Australian English"},
				{Value: "en-CA", Label: "Canadian English"},
				{Value: "other", Label: "Other"},
			}},
			{Key: "base_guide", Label: "Based on", Type: FieldText, Placeholder: "Microsoft Writing Style Guide", Help: "A published guide you follow where this one is silent."},
			{Key: "terms", Label: "Terms", Type: FieldTable, Help: "Words and names to prefer, avoid or use with care, with what to write instead.", Columns: []Field{
				{Key: "term", Label: "Term", Type: FieldText, Placeholder: "e-mail"},
				{Key: "guidance", Label: "Guidance", Type: FieldSelect, Options: []Option{
					{Value: "preferred", Label: "Preferred"},
					{Value: "avoid", Label: "Avoid"},
					{Value: "caution", Label: "Use with care"},
				}},
				{Key: "replacement", Label: "Write instead", Type: FieldText, Placeholder: "email"},
				{Key: "note", Label: "Note", Type: FieldText},
			}},
		},
		Template: "## Rules\n\n- Sentence case for headings\n\n## Examples\n\nBefore and after, for the rules that need showing.\n",
	},
	{
		ID:            "design-system",
		Type:          "Design System",
		Label:         "Design system",
		Description:   "Where the design system lives and which version is current. Tokens and components stay in their own tools, and Marmot points to them.",
		Icon:          "widgets",
		Statuses:      lifecycle,
		DefaultStatus: "active",
		Fields: []Field{
			{Key: "sources", Label: "Sources", Type: FieldTable, Required: true, Help: "Where each part lives. These stay the source of truth, and assistants are sent to them.", Columns: []Field{
				{Key: "kind", Label: "Kind", Type: FieldSelect, Options: []Option{
					{Value: "storybook", Label: "Storybook"},
					{Value: "figma", Label: "Figma"},
					{Value: "tokens", Label: "Design tokens"},
					{Value: "repository", Label: "Repository"},
					{Value: "package", Label: "Package"},
					{Value: "docs", Label: "Documentation site"},
					{Value: "mcp", Label: "MCP server"},
					{Value: "other", Label: "Other"},
				}},
				{Key: "url", Label: "Link", Type: FieldURL},
				{Key: "label", Label: "Label", Type: FieldText},
			}},
			{Key: "version", Label: "Current version", Type: FieldText, Placeholder: "4.2.0"},
			{Key: "platforms", Label: "Platforms", Type: FieldList, Help: "Such as Web, iOS, Android, React or SwiftUI."},
			{Key: "themes", Label: "Themes and modes", Type: FieldList, Help: "Such as Light, Dark or High contrast."},
		},
		Template: "## Principles\n\n- \n\n## Which components to use\n\nThe choices people get wrong, and which components are on their way out.\n",
	},
}

// Kinds returns every kind Marmot offers, in the order they are shown.
func Kinds() []Kind {
	return kinds
}

// KindByID returns the kind with the given key.
func KindByID(id string) (Kind, bool) {
	for _, k := range kinds {
		if k.ID == id {
			return k, true
		}
	}
	return Kind{}, false
}

// KindOfType returns the kind whose asset type matches, ignoring case, so an entry made through the API with type "policy" is still a policy.
func KindOfType(assetType string) (Kind, bool) {
	for _, k := range kinds {
		if strings.EqualFold(k.Type, assetType) {
			return k, true
		}
	}
	return Kind{}, false
}

// Types returns the lowercase asset type of every kind, for queries that match types case-insensitively.
func Types() []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = strings.ToLower(k.Type)
	}
	return out
}

func (k Kind) status(value string) (Status, bool) {
	for _, s := range k.Statuses {
		if s.Value == value {
			return s, true
		}
	}
	return Status{}, false
}

// StateOf returns the state of one of the kind's statuses, or an empty state for a status the kind does not have.
func (k Kind) StateOf(status string) State {
	s, _ := k.status(status)
	return s.State
}
