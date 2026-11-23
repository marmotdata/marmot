package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/cmd/output"
	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:     "knowledge",
	Aliases: []string{"kb"},
	Short:   "Write down and read the company's policies, decisions, runbooks and other knowledge",
	Long: `Knowledge entries hold what the company has written down about how it works: policies, decisions, runbooks, incidents, guides, references, facts, brand voice, writing styles and design systems.

An entry is identified by its ID, its MRN or its exact title. List the kinds and their fields with "marmot knowledge kinds".`,
}

var knowledgeKindsCmd = &cobra.Command{
	Use:   "kinds [kind]",
	Short: "List the kinds of entry, or show one kind's fields and statuses",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, err := c.Knowledge.Kinds(cmd.Context())
		if err != nil {
			return err
		}
		if len(args) == 1 {
			k := findKind(resp.Kinds, args[0])
			if k == nil {
				return fmt.Errorf("no kind %q; run marmot knowledge kinds to list them", args[0])
			}
			if p.IsRaw() {
				return printRaw(p, k)
			}
			t := output.NewTable("FIELD", "TYPE", "REQUIRED", "HELP")
			for _, f := range k.Fields {
				req := ""
				switch {
				case f.Required:
					req = "yes"
				case len(f.RequiredWhen) > 0:
					req = "when " + strings.Join(f.RequiredWhen, ", ")
				}
				typ := string(f.Type)
				if len(f.Options) > 0 {
					var opts []string
					for _, o := range f.Options {
						opts = append(opts, o.Value)
					}
					typ += " (" + strings.Join(opts, "|") + ")"
				}
				if len(f.Columns) > 0 {
					var cols []string
					for _, col := range f.Columns {
						cols = append(cols, col.Key)
					}
					typ += " of {" + strings.Join(cols, ", ") + "}"
				}
				t.AddRow(f.Key, typ, req, f.Help)
			}
			var statuses []string
			for _, s := range k.Statuses {
				statuses = append(statuses, s.Value)
			}
			t.SetFooter("%s (%s). Statuses: %s; new entries are %s", k.Label, k.Type, strings.Join(statuses, ", "), k.DefaultStatus)
			p.PrintTable(t)
			return nil
		}
		if p.IsRaw() {
			return printRaw(p, resp)
		}
		t := output.NewTable("KIND", "TYPE", "DESCRIPTION")
		for _, k := range resp.Kinds {
			t.AddRow(k.ID, k.Type, k.Description)
		}
		p.PrintTable(t)
		return nil
	},
}

var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List knowledge entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}
		opts := marmot.KnowledgeListOptions{}
		opts.Kind, _ = cmd.Flags().GetString("kind")
		opts.Status, _ = cmd.Flags().GetString("status")
		opts.Query, _ = cmd.Flags().GetString("query")
		opts.Team, _ = cmd.Flags().GetString("team")
		opts.ReviewDue, _ = cmd.Flags().GetBool("review-due")
		opts.Mine, _ = cmd.Flags().GetBool("mine")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		opts.Limit, opts.Offset = int64(limit), int64(offset)
		resp, err := c.Knowledge.List(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if p.IsRaw() {
			return printRaw(p, resp)
		}
		t := output.NewTable("ID", "KIND", "STATUS", "TITLE", "OWNERS")
		for _, e := range resp.Entries {
			status := e.Status
			if e.ReviewDue {
				status += " (review due)"
			}
			t.AddRow(e.AssetID, e.Kind, status, e.Name, ownerNames(e.Owners))
		}
		t.SetFooter("Showing %d of %d entries", len(resp.Entries), resp.Total)
		p.PrintTable(t)
		return nil
	},
}

var knowledgeGetCmd = &cobra.Command{
	Use:   "get <id|mrn|title>",
	Short: "Show a knowledge entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := resolveEntry(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		entry, err := c.Knowledge.Get(cmd.Context(), a.ID)
		if err != nil {
			return err
		}
		writeupOnly, _ := cmd.Flags().GetBool("writeup")
		pages, err := c.Knowledge.Writeup(cmd.Context(), a.Mrn)
		if err != nil {
			return err
		}
		if writeupOnly {
			for i, pg := range pages {
				if i > 0 {
					fmt.Println()
				}
				fmt.Println(strings.TrimRight(pg.Content, "\n"))
			}
			return nil
		}
		if p.IsRaw() {
			return printRaw(p, map[string]any{"asset": a, "entry": entry, "writeup": pages})
		}
		t := output.NewTable("FIELD", "VALUE")
		t.AddRow("ID", a.ID)
		t.AddRow("Title", a.Name)
		t.AddRow("Kind", entry.Kind)
		t.AddRow("Status", entry.Status)
		t.AddRow("MRN", a.Mrn)
		t.AddRow("Summary", a.UserDescription)
		if entry.EffectiveFrom != "" {
			t.AddRow("In force from", entry.EffectiveFrom)
		}
		if entry.ReviewBy != "" {
			due := entry.ReviewBy
			if entry.ReviewDue {
				due += " (due)"
			}
			t.AddRow("Review by", due)
		}
		for _, k := range sortedKeys(entry.Fields) {
			t.AddRow(k, fieldText(entry.Fields[k]))
		}
		for _, l := range entry.Links {
			if l.Target != nil {
				t.AddRow(strings.ReplaceAll(l.Relation, "_", " "), fmt.Sprintf("%s (%s) %s", l.Target.Name, l.Target.Type, l.Target.Mrn))
			}
		}
		for _, r := range entry.SupersededBy {
			t.AddRow("Replaced by", fmt.Sprintf("%s (%s)", r.Name, r.ID))
		}
		t.AddRow("Updated", entry.UpdatedAt)
		t.SetFooter("Write-up: %d %s; print it with --writeup", len(pages), plural(len(pages), "page", "pages"))
		p.PrintTable(t)
		return nil
	},
}

var knowledgeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Write down a new knowledge entry",
	Example: `  marmot knowledge create --kind runbook --title "Force Flux to reconcile" \
    --summary "Make Flux apply your push now." --owner "team:Platform" \
    --field triggers="HelmRelease not ready,Kustomization stale" --writeup-file steps.md \
    --applies-to mrn://asset/repository/github/marmotdata-infra`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := newClient()
		if err != nil {
			return err
		}
		kinds, err := c.Knowledge.Kinds(ctx)
		if err != nil {
			return err
		}
		kindRef, _ := cmd.Flags().GetString("kind")
		kind := findKind(kinds.Kinds, kindRef)
		if kind == nil {
			return fmt.Errorf("no kind %q; run marmot knowledge kinds to list them", kindRef)
		}
		title, _ := cmd.Flags().GetString("title")
		summary, _ := cmd.Flags().GetString("summary")
		in := marmot.CreateKnowledgeInput{Type: kind.Type, Title: title, Summary: summary}
		in.Tags, _ = cmd.Flags().GetStringSlice("tag")
		if in.Owners, err = resolveOwners(ctx, c, cmd); err != nil {
			return err
		}
		if in.Entry, err = entryInput(ctx, c, cmd, kind); err != nil {
			return err
		}
		if in.Entry.Status == "" {
			in.Entry.Status = kind.DefaultStatus
		}
		if in.Writeup, err = readWriteup(cmd); err != nil {
			return err
		}
		if in.Writeup == "" {
			in.Writeup = kind.Template
		}
		a, _, err := c.Knowledge.Create(ctx, in)
		if err != nil {
			return err
		}
		fmt.Printf("%s created: %s (ID: %s)\n", kind.Label, a.Name, a.ID)
		return nil
	},
}

var knowledgeUpdateCmd = &cobra.Command{
	Use:   "update <id|mrn|title>",
	Short: "Change a knowledge entry's status, fields, links or write-up",
	Long:  `Only what you pass changes. --field sets one field and leaves the others; --fields-file replaces them all. --applies-to replaces the entry's links; --clear-links removes them.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := resolveEntry(ctx, c, args[0])
		if err != nil {
			return err
		}
		current, err := c.Knowledge.Get(ctx, a.ID)
		if err != nil {
			return err
		}
		kinds, err := c.Knowledge.Kinds(ctx)
		if err != nil {
			return err
		}
		kind := findKind(kinds.Kinds, current.Kind)
		if kind == nil {
			return fmt.Errorf("entry %s has kind %q, which this server doesn't list", a.ID, current.Kind)
		}
		in, err := entryInput(ctx, c, cmd, kind)
		if err != nil {
			return err
		}
		// The server replaces fields as a whole, so single --field changes are merged into what is there.
		if !cmd.Flags().Changed("fields-file") {
			merged := map[string]any{}
			for k, v := range current.Fields {
				merged[k] = v
			}
			for k, v := range in.Fields {
				merged[k] = v
			}
			in.Fields = merged
		}
		if in.Status == "" {
			in.Status = current.Status
		}
		if in.EffectiveFrom == "" {
			in.EffectiveFrom = current.EffectiveFrom
		}
		if in.ReviewBy == "" {
			in.ReviewBy = current.ReviewBy
		}
		if clear, _ := cmd.Flags().GetBool("clear-links"); clear {
			in.Links = []*marmot.KnowledgeLinkInput{}
		}
		if _, err := c.Knowledge.Save(ctx, a.ID, in); err != nil {
			return err
		}
		if cmd.Flags().Changed("summary") {
			summary, _ := cmd.Flags().GetString("summary")
			if _, err := c.Assets.Update(ctx, a.ID, marmot.UpdateAssetInput{UserDescription: summary}); err != nil {
				return fmt.Errorf("entry saved, but the summary wasn't: %w", err)
			}
		}
		writeup, err := readWriteup(cmd)
		if err != nil {
			return err
		}
		if writeup != "" {
			if _, err := c.Knowledge.SetWriteup(ctx, a.Mrn, a.Name, writeup); err != nil {
				return fmt.Errorf("entry saved, but the write-up wasn't: %w", err)
			}
		}
		fmt.Printf("Updated %s (ID: %s)\n", a.Name, a.ID)
		return nil
	},
}

var knowledgeReviewedCmd = &cobra.Command{
	Use:   "reviewed <id|mrn|title>",
	Short: "Record that an entry is still true, and when to check it next",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := resolveEntry(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		next, _ := cmd.Flags().GetString("next")
		e, err := c.Knowledge.MarkReviewed(cmd.Context(), a.ID, next)
		if err != nil {
			return err
		}
		if e.ReviewBy != "" {
			fmt.Printf("Marked %s reviewed; next review by %s\n", a.Name, e.ReviewBy)
		} else {
			fmt.Printf("Marked %s reviewed\n", a.Name)
		}
		return nil
	},
}

var knowledgeAboutCmd = &cobra.Command{
	Use:   "about <asset id|mrn>",
	Short: "List the knowledge that applies to an asset",
	Long:  `Entries linked to the asset, matched by a rule on its tag, type or provider, or applying to something that contains it. Superseded and retired entries are left out unless --include-old.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := resolveAsset(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		opts := marmot.KnowledgeAboutOptions{Limit: 100}
		opts.Kind, _ = cmd.Flags().GetString("kind")
		if old, _ := cmd.Flags().GetBool("include-old"); !old {
			opts.State = "current,pending"
		}
		resp, err := c.Knowledge.About(cmd.Context(), a.ID, opts)
		if err != nil {
			return err
		}
		if p.IsRaw() {
			return printRaw(p, resp)
		}
		t := output.NewTable("ID", "KIND", "STATUS", "TITLE", "WHY")
		for _, e := range resp.Entries {
			why := strings.ReplaceAll(e.Relation, "_", " ")
			switch {
			case e.Via != nil:
				why = "through " + e.Via.Name
			case e.Rule != nil:
				why = fmt.Sprintf("rule: %s %s", e.Rule.SelectorType, e.Rule.SelectorValue)
			}
			t.AddRow(e.AssetID, e.Kind, e.Status, e.Name, why)
		}
		t.SetFooter("%d entries apply to %s", resp.Total, a.Name)
		p.PrintTable(t)
		return nil
	},
}

var knowledgeDeleteCmd = &cobra.Command{
	Use:   "delete <id|mrn|title>",
	Short: "Delete a knowledge entry and its write-up",
	Long:  `Deleting removes the entry for everyone. To keep it findable as history, set its status to superseded or retired with update instead.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := resolveEntry(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		if err := confirm(cmd, fmt.Sprintf("Delete %q (%s)? This can't be undone.", a.Name, a.ID)); err != nil {
			return err
		}
		if err := c.Assets.Delete(cmd.Context(), a.ID); err != nil {
			return err
		}
		fmt.Printf("Deleted %s (ID: %s)\n", a.Name, a.ID)
		return nil
	},
}

// findKind matches a kind by its ID, its type or its label, ignoring case and allowing a plural.
func findKind(kinds []*marmot.KnowledgeKind, ref string) *marmot.KnowledgeKind {
	r := strings.ToLower(strings.TrimSpace(ref))
	r = strings.ReplaceAll(strings.ReplaceAll(r, "_", "-"), " ", "-")
	for _, k := range kinds {
		for _, name := range []string{k.ID, k.Type, k.Label} {
			n := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			if r == n || r == n+"s" {
				return k
			}
		}
	}
	return nil
}

// resolveAsset finds an asset by ID or MRN.
func resolveAsset(ctx context.Context, c *marmot.Client, ref string) (*marmot.Asset, error) {
	ref = strings.TrimSpace(ref)
	if _, err := uuid.Parse(ref); err == nil {
		return c.Assets.Get(ctx, ref)
	}
	if strings.Contains(ref, "://") {
		m, err := mrn.ParseMRN(ref)
		if err != nil || m.Kind != mrn.KindAsset {
			return nil, fmt.Errorf("%q isn't an asset MRN", ref)
		}
		a, err := c.Assets.Find(ctx, marmot.LookupInput{Type: m.Type, Service: m.ProviderSegment(), Name: m.Name})
		if err != nil {
			return nil, err
		}
		if a == nil {
			return nil, fmt.Errorf("no asset %s", ref)
		}
		return a, nil
	}
	return nil, fmt.Errorf("%q isn't an asset ID or MRN; find it with marmot assets search", ref)
}

// resolveEntry finds an entry by ID, MRN or exact title.
func resolveEntry(ctx context.Context, c *marmot.Client, ref string) (*marmot.Asset, error) {
	if _, err := uuid.Parse(strings.TrimSpace(ref)); err == nil || strings.Contains(ref, "://") {
		return resolveAsset(ctx, c, ref)
	}
	resp, err := c.Knowledge.List(ctx, marmot.KnowledgeListOptions{Query: ref, Limit: 50})
	if err != nil {
		return nil, err
	}
	var matches []*marmot.KnowledgeSummary
	for _, e := range resp.Entries {
		if strings.EqualFold(e.Name, strings.TrimSpace(ref)) {
			matches = append(matches, e)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no entry titled %q; find it with marmot knowledge list --query", ref)
	case 1:
		return c.Assets.Get(ctx, matches[0].AssetID)
	}
	var ids []string
	for _, m := range matches {
		ids = append(ids, fmt.Sprintf("%s (%s)", m.AssetID, m.Kind))
	}
	return nil, fmt.Errorf("%d entries are titled %q; use an ID: %s", len(matches), ref, strings.Join(ids, ", "))
}

// resolveOwners turns --owner team:<name>, user:<username> or a bare team name into owner references.
func resolveOwners(ctx context.Context, c *marmot.Client, cmd *cobra.Command) ([]marmot.EntryOwner, error) {
	refs, _ := cmd.Flags().GetStringSlice("owner")
	var out []marmot.EntryOwner
	for _, ref := range refs {
		kind, name, ok := strings.Cut(ref, ":")
		if !ok {
			kind, name = "team", ref
		}
		switch kind {
		case "team":
			id, err := teamID(ctx, c, name)
			if err != nil {
				return nil, err
			}
			out = append(out, marmot.EntryOwner{ID: id, Type: "team"})
		case "user":
			id, err := userID(ctx, c, name)
			if err != nil {
				return nil, err
			}
			out = append(out, marmot.EntryOwner{ID: id, Type: "user"})
		default:
			return nil, fmt.Errorf("owner %q: use team:<name> or user:<username>", ref)
		}
	}
	return out, nil
}

func teamID(ctx context.Context, c *marmot.Client, ref string) (string, error) {
	if _, err := uuid.Parse(ref); err == nil {
		return ref, nil
	}
	teams, err := c.Teams.List(ctx, marmot.TeamsListOptions{Limit: 500})
	if err != nil {
		return "", err
	}
	for _, t := range teams.Teams {
		if strings.EqualFold(t.Name, ref) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no team %q; list them with marmot teams list", ref)
}

func userID(ctx context.Context, c *marmot.Client, ref string) (string, error) {
	if _, err := uuid.Parse(ref); err == nil {
		return ref, nil
	}
	users, err := c.Users.List(ctx, marmot.UsersListOptions{Query: ref, Limit: 50})
	if err != nil {
		return "", err
	}
	for _, u := range users.Users {
		if strings.EqualFold(u.Username, ref) {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("no user %q", ref)
}

// entryInput reads the flags shared by create and update into the entry's typed record.
func entryInput(ctx context.Context, c *marmot.Client, cmd *cobra.Command, kind *marmot.KnowledgeKind) (marmot.KnowledgeSaveInput, error) {
	var in marmot.KnowledgeSaveInput
	in.Status, _ = cmd.Flags().GetString("status")
	in.EffectiveFrom, _ = cmd.Flags().GetString("effective-from")
	in.ReviewBy, _ = cmd.Flags().GetString("review-by")
	fields := map[string]any{}
	if path, _ := cmd.Flags().GetString("fields-file"); path != "" {
		data, err := readFileOrStdin(path)
		if err != nil {
			return in, err
		}
		if err := json.Unmarshal(data, &fields); err != nil {
			return in, fmt.Errorf("%s isn't a JSON object of fields: %w", path, err)
		}
	}
	pairs, _ := cmd.Flags().GetStringArray("field")
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return in, fmt.Errorf("--field %q: write it as key=value", pair)
		}
		v, err := fieldValue(kind, key, value)
		if err != nil {
			return in, err
		}
		fields[key] = v
	}
	if len(fields) > 0 {
		in.Fields = fields
	}
	if ref, _ := cmd.Flags().GetString("supersedes"); ref != "" {
		old, err := resolveEntry(ctx, c, ref)
		if err != nil {
			return in, err
		}
		in.Supersedes = old.ID
	}
	links, _ := cmd.Flags().GetStringArray("applies-to")
	docs, _ := cmd.Flags().GetStringArray("documents")
	related, _ := cmd.Flags().GetStringArray("related-to")
	if len(links)+len(docs)+len(related) > 0 {
		in.Links = []*marmot.KnowledgeLinkInput{}
		for relation, refs := range map[string][]string{"applies_to": links, "documents": docs, "related_to": related} {
			for _, ref := range refs {
				a, err := resolveAsset(ctx, c, ref)
				if err != nil {
					return in, err
				}
				in.Links = append(in.Links, &marmot.KnowledgeLinkInput{Relation: relation, TargetID: a.ID})
			}
		}
	}
	return in, nil
}

// fieldValue converts a --field value to the shape its kind expects: a list is comma separated, a table is JSON, and anything else is text.
func fieldValue(kind *marmot.KnowledgeKind, key, value string) (any, error) {
	for _, f := range kind.Fields {
		if f.Key != key {
			continue
		}
		switch string(f.Type) {
		case "list":
			var items []string
			for _, s := range strings.Split(value, ",") {
				if s = strings.TrimSpace(s); s != "" {
					items = append(items, s)
				}
			}
			return items, nil
		case "table":
			var rows []map[string]any
			if err := json.Unmarshal([]byte(value), &rows); err != nil {
				return nil, fmt.Errorf("--field %s is a table: give a JSON array of rows, or use --fields-file", key)
			}
			return rows, nil
		}
		return value, nil
	}
	var keys []string
	for _, f := range kind.Fields {
		keys = append(keys, f.Key)
	}
	return nil, fmt.Errorf("a %s has no field %q; its fields are %s", strings.ToLower(kind.Label), key, strings.Join(keys, ", "))
}

func readWriteup(cmd *cobra.Command) (string, error) {
	path, _ := cmd.Flags().GetString("writeup-file")
	if path == "" {
		return "", nil
	}
	data, err := readFileOrStdin(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no file %s", path)
	}
	return data, err
}

func ownerNames(owners []*marmot.KnowledgeOwner) string {
	var names []string
	for _, o := range owners {
		names = append(names, o.Name)
	}
	return strings.Join(names, ", ")
}

// fieldText shows a field value on one line: lists joined, tables as row counts.
func fieldText(v any) string {
	switch x := v.(type) {
	case []any:
		if len(x) > 0 {
			if _, isRow := x[0].(map[string]any); isRow {
				return fmt.Sprintf("%d rows (see -o json)", len(x))
			}
		}
		var parts []string
		for _, item := range x {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", ")
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func printRaw(p *output.Printer, v any) error {
	data, err := marshalPayload(v)
	if err != nil {
		return err
	}
	return p.PrintRaw(data)
}

func addEntryFlags(c *cobra.Command) {
	c.Flags().String("status", "", "Status, such as draft, active or accepted (see marmot knowledge kinds <kind>)")
	c.Flags().StringArray("field", nil, "A field as key=value; lists are comma separated, tables are JSON. Repeatable")
	c.Flags().String("fields-file", "", "A JSON object of every field, or - for stdin")
	c.Flags().String("writeup-file", "", "The write-up in Markdown, or - for stdin")
	c.Flags().String("effective-from", "", "Date it comes into force, YYYY-MM-DD")
	c.Flags().String("review-by", "", "Date an owner should check it is still true, YYYY-MM-DD")
	c.Flags().String("supersedes", "", "ID, MRN or title of the entry this one replaces; that entry becomes superseded")
	c.Flags().StringArray("applies-to", nil, "ID or MRN of an asset this entry governs. Repeatable")
	c.Flags().StringArray("documents", nil, "ID or MRN of an asset this entry explains. Repeatable")
	c.Flags().StringArray("related-to", nil, "ID or MRN of an asset worth reading alongside. Repeatable")
}

func init() {
	knowledgeListCmd.Flags().String("kind", "", "Only this kind, such as policy or runbook")
	knowledgeListCmd.Flags().String("status", "", "Only this status")
	knowledgeListCmd.Flags().StringP("query", "q", "", "Words to match in titles, summaries, fields and write-ups")
	knowledgeListCmd.Flags().String("team", "", "Only entries owned by this team ID")
	knowledgeListCmd.Flags().Bool("review-due", false, "Only entries past their review date")
	knowledgeListCmd.Flags().Bool("mine", false, "Only entries you or your teams own")
	knowledgeListCmd.Flags().Int("limit", 50, "Maximum number of results")
	knowledgeListCmd.Flags().Int("offset", 0, "Offset for pagination")

	knowledgeGetCmd.Flags().Bool("writeup", false, "Print only the write-up, as Markdown")

	knowledgeCreateCmd.Flags().String("kind", "", "Kind of entry, such as policy, runbook or fact (required)")
	knowledgeCreateCmd.Flags().String("title", "", "What people would search for to find it (required)")
	knowledgeCreateCmd.Flags().String("summary", "", "One or two sentences saying what it decides or is for")
	knowledgeCreateCmd.Flags().StringSlice("owner", nil, "Owner as team:<name> or user:<username>. Repeatable")
	knowledgeCreateCmd.Flags().StringSlice("tag", nil, "Tag. Repeatable")
	addEntryFlags(knowledgeCreateCmd)
	_ = knowledgeCreateCmd.MarkFlagRequired("kind")
	_ = knowledgeCreateCmd.MarkFlagRequired("title")

	knowledgeUpdateCmd.Flags().String("summary", "", "New summary")
	knowledgeUpdateCmd.Flags().Bool("clear-links", false, "Remove every link")
	addEntryFlags(knowledgeUpdateCmd)

	knowledgeReviewedCmd.Flags().String("next", "", "Next review date, YYYY-MM-DD")

	knowledgeAboutCmd.Flags().String("kind", "", "Only this kind")
	knowledgeAboutCmd.Flags().Bool("include-old", false, "Include superseded and retired entries")

	knowledgeDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	knowledgeCmd.AddCommand(knowledgeKindsCmd, knowledgeListCmd, knowledgeGetCmd, knowledgeCreateCmd, knowledgeUpdateCmd, knowledgeReviewedCmd, knowledgeAboutCmd, knowledgeDeleteCmd)
	rootCmd.AddCommand(knowledgeCmd)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
