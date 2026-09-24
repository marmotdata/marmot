package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/glossary/importer"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type oneOwner struct{ id string }

func (o oneOwner) ResolveOwner(context.Context, string) (glossary.OwnerInput, error) {
	return glossary.OwnerInput{ID: o.id, Type: "user"}, nil
}

func TestGlossaryImportByDomain(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	repo := domain.NewPostgresRepository(pool)
	svc := domain.NewService(repo)
	guard := domain.NewGuard(svc, repo, principalFrom)
	operator := auth.NewOperatorPrincipal()

	finance := mustCreate(t, svc, "Finance", nil)
	payments := mustCreate(t, svc, "Payments", finance)
	legal := mustCreate(t, svc, "Legal", nil)
	steward := person(t, pool, "sofia")
	if _, err := svc.GrantRole(ctx, operator, finance.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: steward.ID(), Role: domain.RoleSteward}); err != nil {
		t.Fatal(err)
	}

	terms := domain.GuardGlossary(glossary.NewService(glossary.NewPostgresRepository(pool, noopRecorder{})), guard)
	im := importer.New(metamodel.Native(), terms, oneOwner{steward.ID()}, domain.GlossaryImportColumns(svc, guard))
	contract, err := terms.Create(ctx, glossary.CreateTermInput{Name: "Contract", Definition: "An agreement", Owners: []glossary.OwnerInput{{ID: steward.ID(), Type: "user"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Assign(ctx, domain.KindGlossaryTerm, []string{contract.ID}, legal.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetWriteEnforced(ctx, true, ""); err != nil {
		t.Fatal(err)
	}

	validate := func(c context.Context, content string, onExisting importer.OnExisting) *importer.Result {
		t.Helper()
		sheet, err := importer.Read(strings.NewReader(content), importer.FormatCSV)
		if err != nil {
			t.Fatal(err)
		}
		result, err := im.Validate(c, sheet, onExisting)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	codes := func(r *importer.Result) map[string]string {
		out := map[string]string{}
		for _, row := range r.Rows {
			var cs []string
			for _, p := range row.Errors {
				cs = append(cs, p.Code)
			}
			out[row.Name] = strings.Join(cs, ",")
		}
		return out
	}

	t.Run("the preview shows domain errors before anything is written", func(t *testing.T) {
		c := as(ctx, steward)
		got := codes(validate(c, strings.Join([]string{
			"name,definition,owners,domain",
			"Ledger,Book of accounts,sofia,finance/PAYMENTS",
			"Clause,Part of a contract,sofia,Legal",
			"Nowhere,Lost,sofia,Atlantis",
			"Orphan,No domain given,sofia,",
		}, "\n"), importer.OnExistingSkip))
		want := map[string]string{"Ledger": "", "Clause": "domain_forbidden", "Nowhere": "domain_not_found", "Orphan": "domain_forbidden"}
		for name, code := range want {
			if got[name] != code {
				t.Errorf("%s: %q, want %q", name, got[name], code)
			}
		}
		defaulted := codes(validate(domain.WithTarget(c, finance.ID), "name,definition,owners\nOrphan,No domain given,sofia\n", importer.OnExistingSkip))
		if defaulted["Orphan"] != "" {
			t.Errorf("the page's default domain lets a row without a column land in Finance: %q", defaulted["Orphan"])
		}
		moving := codes(validate(c, "name,definition,owners,domain\nContract,Reworded,sofia,Finance\n", importer.OnExistingUpdate))
		if moving["Contract"] != "domain_forbidden" {
			t.Errorf("moving a Legal term needs write in Legal: %q", moving["Contract"])
		}
	})

	t.Run("an applied file places each term and audits it", func(t *testing.T) {
		c := domain.WithTarget(as(ctx, steward), finance.ID)
		result := validate(c, "name,definition,owners,domain\nLedger,Book of accounts,sofia,Finance/Payments\nJournal,Daily record,sofia,\n", importer.OnExistingSkip)
		if !result.Valid() {
			t.Fatalf("result = %+v", result)
		}
		if err := im.Apply(c, terms, result); err != nil {
			t.Fatal(err)
		}
		placed := func(name string) string {
			t.Helper()
			term, err := terms.GetByName(ctx, name)
			if err != nil {
				t.Fatal(err)
			}
			id, _ := svc.DomainOf(ctx, domain.KindGlossaryTerm, term.ID)
			return id
		}
		if placed("Ledger") != payments.ID || placed("Journal") != finance.ID {
			t.Fatalf("Ledger in %s, Journal in %s", placed("Ledger"), placed("Journal"))
		}
		ledger, _ := terms.GetByName(ctx, "Ledger")
		if log, err := guard.AuditLog(ctx, string(domain.KindGlossaryTerm), ledger.ID); err != nil || len(log) != 1 || log[0].Action != domain.AuditCreate {
			t.Fatalf("audit = %+v, %v", log, err)
		}
		rows, err := im.Export(ctx, terms)
		if err != nil {
			t.Fatal(err)
		}
		col := len(im.Columns()) - 1
		for _, row := range rows {
			if row[0] == "Ledger" && row[col] != "Finance/Payments" {
				t.Fatalf("export writes the domain path: %v", row)
			}
		}
	})

	t.Run("the decorator refuses what validation would have caught", func(t *testing.T) {
		_, err := terms.Import(as(ctx, steward), []glossary.ImportTerm{{Name: "Sneaky", Extra: map[string]string{"domain": "Legal"}, Create: glossary.CreateTermInput{Name: "Sneaky", Definition: "x", Owners: []glossary.OwnerInput{{ID: steward.ID(), Type: "user"}}}}})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("err = %v, want ErrForbidden", err)
		}
		if _, err := terms.GetByName(ctx, "Sneaky"); !errors.Is(err, glossary.ErrTermNotFound) {
			t.Fatal("a refused import wrote a term")
		}
	})
}
