package importer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/xuri/excelize/v2"
)

const profile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: area
    type: enum
    values: [finance, legal]
    required: true
    core: true
    storage: metadata.governance.area
    appliesTo:
      kinds: [glossary_term]
    presentation:
      labelKey: example.area.label
  - id: regulated
    type: boolean
    core: true
    storage: metadata.regulated
    appliesTo:
      kinds: [glossary_term]
    presentation:
      labelKey: example.regulated.label
  - id: asset_only
    type: string
    core: true
    storage: metadata.asset_only
    presentation:
      labelKey: example.asset_only.label
messages:
  en:
    example.area.label: Business area
  es:
    example.area.label: Área de negocio
`

type fakeTerms map[string][]*glossary.GlossaryTerm

func (f fakeTerms) ByNames(_ context.Context, names []string) (map[string][]*glossary.GlossaryTerm, error) {
	out := map[string][]*glossary.GlossaryTerm{}
	for _, n := range names {
		if t, ok := f[n]; ok {
			out[n] = t
		}
	}
	return out, nil
}

type fakeOwners struct{}

func (fakeOwners) ResolveOwner(_ context.Context, ref string) (glossary.OwnerInput, error) {
	switch ref {
	case "ana":
		return glossary.OwnerInput{ID: "u-ana", Type: "user"}, nil
	case "team:finance":
		return glossary.OwnerInput{ID: "t-finance", Type: "team"}, nil
	}
	return glossary.OwnerInput{}, ErrOwnerNotFound
}

func registry(t *testing.T) *metamodel.Registry {
	t.Helper()
	r, err := metamodel.Load(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func csvSheet(t *testing.T, content string) *Sheet {
	t.Helper()
	sheet, err := Read(strings.NewReader(content), FormatCSV)
	if err != nil {
		t.Fatal(err)
	}
	return sheet
}

func TestColumnsFollowTheProfile(t *testing.T) {
	ids := []string{}
	for _, c := range Columns(registry(t)) {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "name,definition,description,parent,owners,tags,area,regulated" {
		t.Fatalf("columns = %v; asset-only fields must not appear", ids)
	}
	if got := strings.Join(ids[:6], ","); got != strings.Join(func() []string {
		out := []string{}
		for _, c := range Columns(metamodel.Native()) {
			out = append(out, c.ID)
		}
		return out
	}(), ",") {
		t.Fatalf("without a profile only the term's own columns remain: %s", got)
	}
}

func TestTemplateXLSX(t *testing.T) {
	reg := registry(t)
	var buf bytes.Buffer
	if err := Write(&buf, FormatXLSX, Columns(reg), NewTexts(reg.Schema(), "es"), nil); err != nil {
		t.Fatal(err)
	}
	template := bytes.Clone(buf.Bytes())
	f, err := excelize.OpenReader(bytes.NewReader(template))
	if err != nil {
		t.Fatal(err)
	}
	header, _ := f.GetRows(termsSheet)
	if strings.Join(header[0], ",") != "name,definition,description,parent,owners,tags,area,regulated" {
		t.Fatalf("header = %v", header[0])
	}
	dvs, err := f.GetDataValidations(termsSheet)
	if err != nil || len(dvs) != 2 {
		t.Fatalf("drop-downs = %d (%v), want enum and boolean", len(dvs), err)
	}
	comments, _ := f.GetComments(termsSheet)
	var areaComment string
	for _, c := range comments {
		if c.Cell == "G1" {
			areaComment = c.Text
		}
	}
	if !strings.Contains(areaComment, "Área de negocio") || !strings.Contains(areaComment, "(required)") {
		t.Fatalf("the area header should carry its localized label: %q", areaComment)
	}
	guide, _ := f.GetRows(guideSheet)
	if len(guide) != 9 || guide[7][4] != "finance, legal" {
		t.Fatalf("guide = %v", guide)
	}
	sheet, err := Read(bytes.NewReader(template), FormatXLSX)
	if err != nil || len(sheet.Rows) != 0 {
		t.Fatalf("an empty template reads back as a header only: %v, %v", sheet, err)
	}
}

func TestReadCSV(t *testing.T) {
	sheet := csvSheet(t, "\xef\xbb\xbfName;Definition;owners\n'=cmd;A formula;ana\n\n")
	if strings.Join(sheet.Header, ",") != "name,definition,owners" {
		t.Fatalf("header = %v", sheet.Header)
	}
	if len(sheet.Rows) != 1 || sheet.Rows[0][0] != "=cmd" {
		t.Fatalf("rows = %v; ';' must be detected and the formula escape removed", sheet.Rows)
	}
	var many strings.Builder
	many.WriteString("name\n")
	for i := 0; i <= MaxRows; i++ {
		many.WriteString("t\n")
	}
	if _, err := Read(strings.NewReader(many.String()), FormatCSV); !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("err = %v, want ErrTooManyRows", err)
	}
	if _, err := Read(bytes.NewReader(make([]byte, MaxBytes+1)), FormatCSV); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestValidate(t *testing.T) {
	existing := fakeTerms{
		"Invoice": {{ID: "t-invoice", Name: "Invoice", Metadata: map[string]interface{}{"governance": map[string]interface{}{"area": "finance"}, "other": "kept"}}},
		"Twin":    {{ID: "t-1", Name: "Twin"}, {ID: "t-2", Name: "Twin"}},
	}
	im := New(registry(t), existing, fakeOwners{})
	ctx := context.Background()
	content := strings.Join([]string{
		"name,definition,parent,owners,area,regulated,extra",
		"Payment,Money moving,Invoice,ana|team:finance,finance,sí,x",
		"Card payment,Paid by card,Payment,ana,legal,,",
		"Invoice,Reworded,,,,true,",
		"Twin,Ambiguous,,ana,,,",
		"Orphan,No parent,Nowhere,ana,finance,,",
		"Nobody,No owner,,ghost,finance,,",
		"Wrong,Bad enum,,ana,sales,maybe,",
		"Loop A,A,Loop B,ana,finance,,",
		"Loop B,B,Loop A,ana,finance,,",
		"Draft,No area yet,,ana,,,",
		"Payment,Again,,ana,finance,,",
	}, "\n")
	result, err := im.Validate(ctx, csvSheet(t, content), OnExistingSkip)
	if err != nil {
		t.Fatal(err)
	}
	byLine := map[int]Row{}
	for _, r := range result.Rows {
		byLine[r.Line] = r
	}
	codes := func(r Row) string {
		out := []string{}
		for _, p := range r.Errors {
			out = append(out, p.Code)
		}
		return strings.Join(out, ",")
	}
	for line, want := range map[int]string{
		2:  "duplicate_in_file",
		3:  "",
		4:  "",
		5:  "ambiguous_name",
		6:  "parent_not_found",
		7:  "owner_not_found",
		8:  "type,type",
		9:  "parent_cycle",
		10: "parent_cycle",
		11: "",
		12: "duplicate_in_file",
	} {
		if got := codes(byLine[line]); got != want {
			t.Errorf("line %d: errors %q, want %q", line, got, want)
		}
	}
	if byLine[4].Action != ActionSkip {
		t.Errorf("an existing term is skipped by default, got %s", byLine[4].Action)
	}
	if len(byLine[11].Warnings) != 1 || byLine[11].Warnings[0].Code != "missing" {
		t.Errorf("a missing required profile field is a warning: %+v", byLine[11].Warnings)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Column != "extra" || result.Valid() {
		t.Errorf("unknown columns warn; row errors block: %+v", result)
	}

	updated, err := im.Validate(ctx, csvSheet(t, "name,definition,regulated\nInvoice,Reworded,true\n"), OnExistingUpdate)
	if err != nil || !updated.Valid() || updated.Rows[0].Action != ActionUpdate {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	term := updated.Terms()[0]
	if term.ExistingID != "t-invoice" || *term.Update.Definition != "Reworded" || term.Update.Owners != nil {
		t.Fatalf("empty cells keep current values: %+v", term.Update)
	}
	if v, _ := metamodel.ValueAt(term.Update.Metadata, "metadata.regulated"); v != true {
		t.Fatalf("regulated = %v", v)
	}
	if v, _ := metamodel.ValueAt(term.Update.Metadata, "metadata.other"); v != "kept" {
		t.Fatalf("metadata outside the file's columns must be kept: %v", term.Update.Metadata)
	}

	if missing, _ := im.Validate(ctx, csvSheet(t, "definition\nNo name column\n"), OnExistingSkip); len(missing.Problems) != 1 || missing.Valid() {
		t.Fatalf("a file without a name column cannot be applied: %+v", missing)
	}
}

type recordingGlossary struct {
	glossary.Service
	terms    []*glossary.GlossaryTerm
	imported []glossary.ImportTerm
}

func (g *recordingGlossary) Import(_ context.Context, terms []glossary.ImportTerm) ([]*glossary.GlossaryTerm, error) {
	g.imported = terms
	return nil, nil
}

func (g *recordingGlossary) List(_ context.Context, offset, limit int) (*glossary.ListResult, error) {
	end := min(offset+limit, len(g.terms))
	if offset > end {
		offset = end
	}
	return &glossary.ListResult{Terms: g.terms[offset:end], Total: len(g.terms)}, nil
}

func (g *recordingGlossary) Get(_ context.Context, id string) (*glossary.GlossaryTerm, error) {
	for _, t := range g.terms {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, glossary.ErrTermNotFound
}

func TestApplyOnlyValidFiles(t *testing.T) {
	im := New(registry(t), fakeTerms{}, fakeOwners{})
	ctx := context.Background()
	svc := &recordingGlossary{}
	bad, _ := im.Validate(ctx, csvSheet(t, "name,definition,owners\nA,,ana\n"), OnExistingSkip)
	if err := im.Apply(ctx, svc, bad); err == nil || svc.imported != nil || bad.Applied {
		t.Fatal("a file with errors must not be written")
	}
	good, _ := im.Validate(ctx, csvSheet(t, "name,definition,owners,area\nA,Defined,ana,legal\n"), OnExistingSkip)
	if err := im.Apply(ctx, svc, good); err != nil || len(svc.imported) != 1 || !good.Applied {
		t.Fatalf("apply: %v, %+v", err, svc.imported)
	}
}

func TestExportImportsBackUnchanged(t *testing.T) {
	reg := registry(t)
	ana := "ana"
	parent := "t-invoice"
	svc := &recordingGlossary{terms: []*glossary.GlossaryTerm{
		{ID: "t-invoice", Name: "Invoice", Definition: "=HYPERLINK(\"x\")", Owners: []glossary.Owner{{Type: "user", Username: &ana}}, Metadata: map[string]interface{}{"governance": map[string]interface{}{"area": "finance"}}},
		{ID: "t-card", Name: "Card", Definition: "Paid by card", ParentTermID: &parent, Owners: []glossary.Owner{{Type: "team", Name: "finance"}}, Tags: []string{"pci", "cards"}, Metadata: map[string]interface{}{"governance": map[string]interface{}{"area": "legal"}, "regulated": true}},
	}}
	terms := fakeTerms{"Invoice": {svc.terms[0]}, "Card": {svc.terms[1]}}
	im := New(reg, terms, fakeOwners{})
	ctx := context.Background()
	rows, err := im.Export(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, FormatCSV, im.Columns(), NewTexts(reg.Schema(), "en"), rows); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"'=HYPERLINK(""x"")"`) {
		t.Fatalf("a formula-looking cell must be escaped in CSV:\n%s", buf.String())
	}
	sheet, err := Read(&buf, FormatCSV)
	if err != nil {
		t.Fatal(err)
	}
	result, err := im.Validate(ctx, sheet, OnExistingUpdate)
	if err != nil || !result.Valid() || result.Summary.Update != 2 {
		t.Fatalf("round trip: %+v, %v", result, err)
	}
	card := result.Terms()[1]
	if card.ParentName != "Invoice" || strings.Join(card.Update.Tags, ",") != "pci,cards" || card.Update.Owners[0].ID != "t-finance" {
		t.Fatalf("card = %+v", card)
	}
	if *result.Terms()[0].Update.Definition != `=HYPERLINK("x")` {
		t.Fatalf("the escape must be undone on import: %q", *result.Terms()[0].Update.Definition)
	}
}
