package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

type principalKey struct{}

func as(ctx context.Context, p auth.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(auth.Principal)
	return p, ok
}

// The inner services record what reached them; the guard decides what does.

type innerAssets struct {
	asset.Service
	pool   *pgxpool.Pool
	writes int
}

func (s *innerAssets) Create(context.Context, asset.CreateInput) (*asset.Asset, error) {
	s.writes++
	return &asset.Asset{}, nil
}

func (s *innerAssets) Update(context.Context, string, asset.UpdateInput) (*asset.Asset, error) {
	s.writes++
	return &asset.Asset{}, nil
}

func (s *innerAssets) AddTag(context.Context, string, string) (*asset.Asset, error) {
	s.writes++
	return &asset.Asset{}, nil
}

func (s *innerAssets) DeleteByMRN(context.Context, string) error {
	s.writes++
	return nil
}

func (s *innerAssets) GetByMRN(ctx context.Context, mrn string) (*asset.Asset, error) {
	var id string
	if err := s.pool.QueryRow(ctx, "SELECT id FROM assets WHERE mrn = $1", mrn).Scan(&id); err != nil {
		return nil, asset.ErrAssetNotFound
	}
	return &asset.Asset{ID: id}, nil
}

type innerProducts struct {
	dataproduct.Service
	pool    *pgxpool.Pool
	creator string
	writes  int
}

func (s *innerProducts) Create(ctx context.Context, in dataproduct.CreateInput) (*dataproduct.DataProduct, error) {
	s.writes++
	var id string
	if err := s.pool.QueryRow(ctx, "INSERT INTO data_products (name, created_by) VALUES ($1, $2) RETURNING id", in.Name, s.creator).Scan(&id); err != nil {
		return nil, err
	}
	return &dataproduct.DataProduct{ID: id}, nil
}

func (s *innerProducts) AddAssets(context.Context, string, []string, string) error {
	s.writes++
	return nil
}

func (s *innerProducts) CreateRule(context.Context, string, dataproduct.RuleInput) (*dataproduct.Rule, error) {
	s.writes++
	return &dataproduct.Rule{}, nil
}

type innerGlossary struct {
	glossary.Service
	writes int
}

func (s *innerGlossary) SyncTerms(context.Context, string, []glossary.TermInput) (*glossary.SyncResult, error) {
	s.writes++
	return &glossary.SyncResult{}, nil
}

type innerRules struct {
	assetrule.Service
	writes int
}

func (s *innerRules) Delete(context.Context, string) error {
	s.writes++
	return nil
}

type innerLineage struct {
	lineage.Service
	target string
	writes int
}

func (s *innerLineage) GetDirectLineage(context.Context, string) (*lineage.LineageEdge, error) {
	return &lineage.LineageEdge{Target: s.target}, nil
}

func (s *innerLineage) DeleteDirectLineage(context.Context, string) error {
	s.writes++
	return nil
}

func (s *innerLineage) BatchObservedLineage(context.Context, []lineage.ObservedEdge) error {
	s.writes++
	return nil
}

type innerAssetDocs struct {
	assetdocs.Service
	writes int
}

func (s *innerAssetDocs) Create(context.Context, assetdocs.Documentation) error {
	s.writes++
	return nil
}

func (s *innerAssetDocs) CreateGlobal(context.Context, assetdocs.GlobalDocumentation) error {
	s.writes++
	return nil
}

func TestWriteEnforcement(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	repo := domain.NewPostgresRepository(pool)
	svc := domain.NewService(repo)
	guard := domain.NewGuard(svc, repo, principalFrom)
	operator := auth.NewOperatorPrincipal()

	finance := mustCreate(t, svc, "Finance", nil)
	payments := mustCreate(t, svc, "Payments", finance)
	legal := mustCreate(t, svc, "Legal", nil)

	inFinance, inLegal, unassigned := pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool), pgtest.SeedAsset(t, pool)
	place := func(kind domain.Kind, id, domainID string) {
		t.Helper()
		if err := svc.Assign(ctx, kind, []string{id}, domainID); err != nil {
			t.Fatal(err)
		}
	}
	place(domain.KindAsset, inFinance, finance.ID)
	place(domain.KindAsset, inLegal, legal.ID)

	steward := person(t, pool, "fiona")
	grant := func(d *domain.Domain, st domain.SubjectType, id string, role domain.Role) {
		t.Helper()
		if _, err := svc.GrantRole(ctx, operator, d.ID, domain.GrantInput{SubjectType: st, SubjectID: id, Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	grant(finance, domain.SubjectUser, steward.ID(), domain.RoleSteward)
	lawyer := person(t, pool, "lara")
	grant(legal, domain.SubjectUser, lawyer.ID(), domain.RoleSteward)

	// The OpenLineage emitter only onboards new assets.
	robot := seed(t, pool, "INSERT INTO service_accounts (name) VALUES ('lineage-bot') RETURNING id")
	unassignedDomain, err := svc.Get(ctx, domain.UnassignedID)
	if err != nil {
		t.Fatal(err)
	}
	grant(unassignedDomain, domain.SubjectServiceAccount, robot, domain.RoleSteward)
	emitter := auth.NewServiceAccountPrincipal(robot, "lineage-bot", nil, nil)

	schedule := seed(t, pool, `INSERT INTO ingestion_schedules (name, plugin_id, cron_expression) VALUES ('ledger', 'postgresql', '0 * * * *') RETURNING id`)
	if _, err := svc.AssignPipeline(ctx, schedule, finance.ID, false); err != nil {
		t.Fatal(err)
	}
	scheduled := domain.WithPipeline(ctx, "ledger")

	assets := &innerAssets{pool: pool}
	guardedAssets := domain.GuardAssets(assets, guard)
	products := &innerProducts{pool: pool, creator: steward.ID()}
	guardedProducts := domain.GuardDataProducts(products, guard)
	terms := &innerGlossary{}
	guardedTerms := domain.GuardGlossary(terms, guard)
	rules := &innerRules{}
	guardedRules := domain.GuardAssetRules(rules, guard)

	allowed := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("want allowed, got %v", err)
		}
	}
	denied := func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	}
	mrnOf := func(id string) string {
		var mrn string
		if err := pool.QueryRow(ctx, "SELECT mrn FROM assets WHERE id = $1", id).Scan(&mrn); err != nil {
			t.Fatal(err)
		}
		return mrn
	}

	t.Run("off: every write delegates, even without a principal", func(t *testing.T) {
		before := assets.writes
		_, err := guardedAssets.Update(ctx, inLegal, asset.UpdateInput{})
		allowed(t, err)
		_, err = guardedAssets.Create(ctx, asset.CreateInput{})
		allowed(t, err)
		if assets.writes != before+2 {
			t.Fatal("writes did not reach the inner service")
		}
	})

	if err := repo.SetWriteEnforced(ctx, true, operator.ID()); err != nil {
		t.Fatal(err)
	}

	t.Run("REST: a steward writes in its subtree only", func(t *testing.T) {
		c := as(ctx, steward)
		_, err := guardedAssets.Update(c, inFinance, asset.UpdateInput{})
		allowed(t, err)
		_, err = guardedAssets.AddTag(c, inLegal, "pii")
		denied(t, err)
		_, err = guardedAssets.Update(c, unassigned, asset.UpdateInput{})
		denied(t, err)
		_, err = guardedAssets.Create(c, asset.CreateInput{})
		denied(t, err)
	})
	t.Run("REST: native admins and the operator are global", func(t *testing.T) {
		admin := auth.NewUserPrincipal(&user.User{ID: "x", Roles: []user.Role{{Name: auth.AdminRoleName}}})
		_, err := guardedAssets.Update(as(ctx, admin), inLegal, asset.UpdateInput{})
		allowed(t, err)
		allowed(t, guardedRules.Delete(as(ctx, operator), "rule"))
	})
	t.Run("no principal and no pipeline writes nothing", func(t *testing.T) {
		_, err := guardedAssets.Update(ctx, inFinance, asset.UpdateInput{})
		denied(t, err)
	})
	t.Run("OpenLineage: an onboarding account creates but cannot edit governed assets", func(t *testing.T) {
		c := as(ctx, emitter)
		_, err := guardedAssets.Create(c, asset.CreateInput{})
		allowed(t, err)
		_, err = guardedAssets.Update(c, unassigned, asset.UpdateInput{})
		allowed(t, err)
		_, err = guardedAssets.Update(c, inFinance, asset.UpdateInput{})
		denied(t, err)
	})
	t.Run("ingestion: a scheduled run writes in its schedule's domain", func(t *testing.T) {
		_, err := guardedAssets.Create(scheduled, asset.CreateInput{})
		allowed(t, err)
		allowed(t, guardedAssets.DeleteByMRN(scheduled, mrnOf(inFinance)))
		denied(t, guardedAssets.DeleteByMRN(scheduled, mrnOf(inLegal)))
		_, err = guardedAssets.Update(scheduled, inLegal, asset.UpdateInput{})
		denied(t, err)
	})
	t.Run("ingestion: naming a pipeline grants the caller nothing", func(t *testing.T) {
		_, err := guardedAssets.Create(as(scheduled, lawyer), asset.CreateInput{})
		denied(t, err)
		_, err = guardedAssets.Create(as(scheduled, steward), asset.CreateInput{})
		allowed(t, err)
	})
	t.Run("ingestion: an unassigned pipeline onboards into Unassigned", func(t *testing.T) {
		c := domain.WithPipeline(ctx, "adhoc")
		_, err := guardedAssets.Update(c, unassigned, asset.UpdateInput{})
		allowed(t, err)
		_, err = guardedAssets.Update(c, inFinance, asset.UpdateInput{})
		denied(t, err)
	})
	t.Run("ingestion: a glossary sync is all or nothing", func(t *testing.T) {
		term := seed(t, pool, "INSERT INTO glossary_terms (name, definition) VALUES ('Invoice', 'A bill') RETURNING id")
		place(domain.KindGlossaryTerm, term, legal.ID)
		_, err := guardedTerms.SyncTerms(scheduled, "postgresql", []glossary.TermInput{{Name: "Ledger"}, {Name: " Invoice "}})
		denied(t, err)
		_, err = guardedTerms.SyncTerms(scheduled, "postgresql", []glossary.TermInput{{Name: "Ledger"}})
		allowed(t, err)
		if terms.writes != 1 {
			t.Fatalf("inner sync ran %d times, want 1", terms.writes)
		}
	})
	t.Run("products: membership follows the product's domain; rules are global", func(t *testing.T) {
		product := seed(t, pool, "INSERT INTO data_products (name, created_by) VALUES ('Ledger', $1) RETURNING id", steward.ID())
		place(domain.KindDataProduct, product, payments.ID)
		allowed(t, guardedProducts.AddAssets(as(ctx, steward), product, []string{inLegal}, ""))
		denied(t, guardedProducts.AddAssets(as(ctx, lawyer), product, []string{inLegal}, ""))
		_, err := guardedProducts.CreateRule(as(ctx, steward), product, dataproduct.RuleInput{})
		denied(t, err)
	})
	t.Run("transfers need both ends and are audited", func(t *testing.T) {
		moving := pgtest.SeedAsset(t, pool)
		place(domain.KindAsset, moving, finance.ID)
		denied(t, guard.Transfer(as(ctx, steward), domain.KindAsset, []string{moving}, legal.ID))
		allowed(t, guard.Transfer(as(ctx, steward), domain.KindAsset, []string{moving}, payments.ID))
		allowed(t, guard.Transfer(as(ctx, operator), domain.KindAsset, []string{moving}, legal.ID))

		log, err := guard.AuditLog(ctx, string(domain.KindAsset), moving)
		if err != nil {
			t.Fatal(err)
		}
		if len(log) != 2 || *log[0].ToDomain != payments.ID || *log[1].FromDomain != payments.ID || log[1].Actor != "operator:"+operator.ID() {
			t.Fatalf("audit = %+v", log)
		}
		if in, _ := svc.DomainOf(ctx, domain.KindAsset, moving); in != legal.ID {
			t.Fatalf("asset ended in %s", in)
		}
	})
	t.Run("lineage: the target's domain decides", func(t *testing.T) {
		edges := &innerLineage{}
		guarded := domain.GuardLineage(edges, guard)
		c := as(ctx, steward)
		allowed(t, guard.AuthorizeEdge(c, mrnOf(inLegal), mrnOf(inFinance)))
		denied(t, guard.AuthorizeEdge(c, mrnOf(inFinance), mrnOf(inLegal)))
		denied(t, guard.AuthorizeEdge(c, mrnOf(inFinance), "mrn://table/nowhere/stub"))
		edges.target = mrnOf(inLegal)
		denied(t, guarded.DeleteDirectLineage(c, "edge"))
		denied(t, guarded.BatchObservedLineage(c, []lineage.ObservedEdge{
			{Source: mrnOf(inLegal), Target: mrnOf(inFinance)},
			{Source: mrnOf(inFinance), Target: mrnOf(inLegal)},
		}))
		allowed(t, guarded.BatchObservedLineage(c, []lineage.ObservedEdge{{Source: mrnOf(inLegal), Target: mrnOf(inFinance)}}))
		if edges.writes != 1 {
			t.Fatalf("inner writes = %d, want 1", edges.writes)
		}
	})
	t.Run("documentation follows the owning entity", func(t *testing.T) {
		docs := domain.GuardAssetDocs(&innerAssetDocs{}, guard)
		c := as(ctx, steward)
		allowed(t, docs.Create(c, assetdocs.Documentation{MRN: mrnOf(inFinance)}))
		denied(t, docs.Create(c, assetdocs.Documentation{MRN: mrnOf(inLegal)}))
		denied(t, docs.CreateGlobal(c, assetdocs.GlobalDocumentation{}))

		page := seed(t, pool, "INSERT INTO doc_pages (entity_type, entity_id) VALUES ('asset', $1) RETURNING id", mrnOf(inLegal))
		image := seed(t, pool, "INSERT INTO doc_images (page_id, filename, content_type, size_bytes, data) VALUES ($1, 'a.png', 'image/png', 1, 'x') RETURNING id", page)
		denied(t, guard.AuthorizeDoc(c, "", "", page, ""))
		denied(t, guard.AuthorizeDoc(c, "", "", "", image))
		allowed(t, guard.AuthorizeDoc(as(ctx, lawyer), "", "", "", image))
		allowed(t, guard.AuthorizeDoc(c, "asset", mrnOf(inFinance), "", ""))
		allowed(t, guard.AuthorizeDoc(c, "", "", "00000000-0000-4000-8000-00000000ffff", ""))
	})
	t.Run("a create can target a domain the actor writes in", func(t *testing.T) {
		c := as(ctx, steward)
		before := products.writes
		_, err := guardedProducts.Create(domain.WithTarget(c, legal.ID), dataproduct.CreateInput{Name: "Elsewhere"})
		denied(t, err)
		if products.writes != before {
			t.Fatal("a refused create reached the inner service")
		}
		dp, err := guardedProducts.Create(domain.WithTarget(c, payments.ID), dataproduct.CreateInput{Name: "Settlements"})
		allowed(t, err)
		if in, _ := svc.DomainOf(ctx, domain.KindDataProduct, dp.ID); in != payments.ID {
			t.Fatalf("created in %s, want Payments", in)
		}
		log, err := guard.AuditLog(ctx, string(domain.KindDataProduct), dp.ID)
		if err != nil || len(log) != 1 || log[0].Action != domain.AuditCreate || log[0].FromDomain != nil {
			t.Fatalf("audit = %+v, %v", log, err)
		}

		if err := repo.SetWriteEnforced(ctx, false, ""); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = repo.SetWriteEnforced(ctx, true, "") }()
		dp, err = guardedProducts.Create(domain.WithTarget(ctx, legal.ID), dataproduct.CreateInput{Name: "Unenforced"})
		allowed(t, err)
		if in, _ := svc.DomainOf(ctx, domain.KindDataProduct, dp.ID); in != legal.ID {
			t.Fatalf("with enforcement off the target still applies; got %s", in)
		}
	})
	t.Run("a pipeline moves only with write on both domains", func(t *testing.T) {
		_, err := guard.AssignPipeline(as(ctx, steward), schedule, legal.ID, true)
		denied(t, err)
		if _, err := guard.AssignPipeline(as(ctx, steward), schedule, payments.ID, false); err != nil {
			t.Fatal(err)
		}
		log, err := guard.AuditLog(ctx, string(domain.KindIngestionSchedule), schedule)
		if err != nil || len(log) != 1 || log[0].Action != domain.AuditAssignPipeline {
			t.Fatalf("audit = %+v, %v", log, err)
		}
	})
}
