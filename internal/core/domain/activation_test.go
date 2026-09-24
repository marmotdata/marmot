package domain_test

import (
	"context"
	"slices"
	"testing"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestEnforcementPlanAndActivation(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	repo := domain.NewPostgresRepository(pool)
	svc := domain.NewService(repo)
	operator := auth.NewOperatorPrincipal()

	finance := mustCreate(t, svc, "Finance", nil)
	payments := mustCreate(t, svc, "Payments", finance)
	legal := mustCreate(t, svc, "Legal", nil)

	editorRole := seed(t, pool, "INSERT INTO roles (name, description) VALUES ('editor', 'edits assets') RETURNING id")
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT $1, id FROM permissions WHERE resource_type = 'assets' AND action IN ('view', 'manage')`, editorRole); err != nil {
		t.Fatal(err)
	}
	editor := person(t, pool, "erin")
	if _, err := pool.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)", editor.ID(), editorRole); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantRole(ctx, operator, payments.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: editor.ID(), Role: domain.RoleSteward}); err != nil {
		t.Fatal(err)
	}
	adminID := person(t, pool, "ada").ID()
	if _, err := pool.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'admin'", adminID); err != nil {
		t.Fatal(err)
	}
	admin := auth.NewUserPrincipal(&user.User{ID: adminID, Username: "ada", Roles: []user.Role{{Name: auth.AdminRoleName}}})

	schedule := seed(t, pool, `INSERT INTO ingestion_schedules (name, plugin_id, cron_expression) VALUES ('ledger', 'postgresql', '0 * * * *') RETURNING id`)
	moved := pgtest.SeedAsset(t, pool)
	ingest(t, pool, schedule, moved, pgtest.SeedAsset(t, pool))
	if _, err := svc.AssignPipeline(ctx, schedule, finance.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Assign(ctx, domain.KindAsset, []string{moved}, legal.ID); err != nil {
		t.Fatal(err)
	}

	plan, err := svc.EnforcementPlan(ctx, operator)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Principals) != 1 || plan.Principals[0].SubjectID != editor.ID() {
		t.Fatalf("principals = %+v, want only the editor: admins keep global scope", plan.Principals)
	}
	refs := func(list []domain.DomainRef) map[string]bool {
		out := map[string]bool{}
		for _, d := range list {
			out[d.Path] = true
		}
		return out
	}
	keeps, loses := refs(plan.Principals[0].Keeps), refs(plan.Principals[0].Loses)
	if len(keeps) != 1 || !keeps["Finance / Payments"] {
		t.Fatalf("keeps = %v", keeps)
	}
	if len(loses) != 3 || !loses["Finance"] || !loses["Legal"] || !loses["Unassigned"] {
		t.Fatalf("loses = %v", loses)
	}
	if len(plan.Pipelines) != 1 || plan.Pipelines[0].AssetsOutside != 1 || plan.Pipelines[0].Domain.Path != "Finance" {
		t.Fatalf("pipelines = %+v", plan.Pipelines)
	}

	t.Run("only global scope plans or switches", func(t *testing.T) {
		_, err := svc.EnforcementPlan(ctx, editor)
		wantErr(t, err, domain.ErrForbidden)
		_, err = svc.SetWriteEnforcement(ctx, editor, false, "")
		wantErr(t, err, domain.ErrForbidden)
	})
	t.Run("a plan that changed is refused", func(t *testing.T) {
		if _, err := svc.GrantRole(ctx, operator, legal.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: editor.ID(), Role: domain.RoleSteward}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.SetWriteEnforcement(ctx, admin, true, plan.Hash)
		wantErr(t, err, domain.ErrPlanChanged)
		if on, _ := repo.WriteEnforced(ctx); on {
			t.Fatal("enforcement turned on with a stale plan")
		}
	})
	t.Run("the reviewed plan activates, and both directions are audited", func(t *testing.T) {
		fresh, err := svc.EnforcementPlan(ctx, admin)
		if err != nil {
			t.Fatal(err)
		}
		if fresh.Hash == plan.Hash {
			t.Fatal("the hash ignored a new grant")
		}
		state, err := svc.SetWriteEnforcement(ctx, admin, true, fresh.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if !state.Write || state.UpdatedBy == nil || *state.UpdatedBy != "user:"+admin.ID() {
			t.Fatalf("state = %+v", state)
		}
		w, err := svc.WritableDomains(ctx, editor)
		if err != nil || w.All || !w.Enforced || !slices.Contains(w.DomainIDs, payments.ID) || !slices.Contains(w.DomainIDs, legal.ID) || slices.Contains(w.DomainIDs, finance.ID) {
			t.Fatalf("writable for the editor = %+v, %v", w, err)
		}
		if w, err := svc.WritableDomains(ctx, admin); err != nil || !w.All {
			t.Fatalf("writable for an admin = %+v, %v", w, err)
		}
		if state, err = svc.SetWriteEnforcement(ctx, operator, false, ""); err != nil || state.Write {
			t.Fatalf("turning off: %+v, %v", state, err)
		}
		log, err := repo.AuditLog(ctx, "setting", "write_enforcement")
		if err != nil || len(log) != 2 || log[0].Action != domain.AuditEnableEnforcement || log[1].Action != domain.AuditDisableEnforcement {
			t.Fatalf("audit = %+v, %v", log, err)
		}
	})
}
