package mcp

import (
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/knowledge"
)

func treeOf(content string) *docs.PageTree {
	return &docs.PageTree{Pages: []*docs.Page{{Title: "Entry", Content: &content}}}
}

func TestWriteWriteupShortIsWhole(t *testing.T) {
	var b strings.Builder
	writeWriteup(&b, "id-1", treeOf("## Rules\n\nSentence case.\n"), "", 0)
	out := b.String()
	if !strings.Contains(out, "## Write-up\n") || !strings.Contains(out, "Sentence case.") {
		t.Fatalf("short write-up should come back whole, got:\n%s", out)
	}
	if strings.Contains(out, "There is more") {
		t.Fatalf("short write-up should not offer a continuation:\n%s", out)
	}
}

func TestWriteWriteupPagesLongText(t *testing.T) {
	section := strings.Repeat("A sentence about the rule. ", 400) // about 10,800 characters
	text := "## One\n\n" + section + "\n\n## Two\n\n" + section + "\n\n## Three\n\n" + section + "\n"

	var first strings.Builder
	writeWriteup(&first, "id-1", treeOf(text), "", 0)
	out := first.String()
	if !strings.Contains(out, "Sections:\n- One\n- Two\n- Three\n") {
		t.Fatalf("first part should list the sections, got:\n%s", out[:min(len(out), 400)])
	}
	if !strings.Contains(out, `"offset": `) {
		t.Fatal("first part of a long write-up should say how to continue")
	}
	if strings.Contains(out, "## Three") {
		t.Fatal("first part should stop at a heading inside the budget")
	}

	var two strings.Builder
	writeWriteup(&two, "id-1", treeOf(text), "two", 0)
	got := two.String()
	if !strings.Contains(got, "## Two") || strings.Contains(got, "## Three") || strings.Contains(got, "## One") {
		t.Fatalf("section read should return only that section, got:\n%s", got[:min(len(got), 300)])
	}
}

func TestWriteWriteupUnknownSection(t *testing.T) {
	var b strings.Builder
	writeWriteup(&b, "id-1", treeOf("## Rules\n\nx\n\n## Examples\n\ny\n"), "Voice", 0)
	if !strings.Contains(b.String(), `no section "Voice"`) || !strings.Contains(b.String(), "- Examples") {
		t.Fatalf("unknown section should list the real ones, got:\n%s", b.String())
	}
}

func TestNormaliseKinds(t *testing.T) {
	got, bad := normaliseKinds([]string{"Writing Style", "policies", "design_system", "runbooks", "memo"})
	want := []string{"writing-style", "policy", "design-system", "runbook"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	if len(bad) != 1 || bad[0] != "memo" {
		t.Fatalf("bad = %v, want [memo]", bad)
	}
}

func TestWhyApplies(t *testing.T) {
	inherited := knowledge.AboutItem{Source: knowledge.SourceInherited, Relation: knowledge.RelationAppliesTo, Via: &knowledge.AssetRef{Name: "marmotdata/marmot", Type: "Repository"}}
	if got := whyApplies(inherited); got != "applies here through marmotdata/marmot (Repository), which contains it" {
		t.Fatalf("inherited = %q", got)
	}
	rule := knowledge.AboutItem{Source: knowledge.SourceRule, Relation: knowledge.RelationAppliesTo, Rule: &knowledge.Rule{SelectorType: "tag", SelectorValue: "pii"}}
	if got := whyApplies(rule); got != `applies here by rule: every asset with tag "pii"` {
		t.Fatalf("rule = %q", got)
	}
}
