package domain_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func seed(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	return id
}

func person(t *testing.T, pool *pgxpool.Pool, username string) auth.Principal {
	t.Helper()
	id := seed(t, pool, "INSERT INTO users (username, name) VALUES ($1, $1) RETURNING id", username)
	return auth.NewUserPrincipal(&user.User{ID: id, Username: username, Name: username})
}

func TestScopeAndGrants(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	svc := domain.NewService(domain.NewPostgresRepository(pool))
	admin := auth.NewOperatorPrincipal()

	finance := mustCreate(t, svc, "Finance", nil)
	payments := mustCreate(t, svc, "Payments", finance)
	cards := mustCreate(t, svc, "Cards", payments)
	legal := mustCreate(t, svc, "Legal", nil)

	alice := person(t, pool, "alice") // steward of Payments
	bob := person(t, pool, "bob")     // in a team that stewards Legal
	carol := person(t, pool, "carol") // domain admin of Payments
	team := seed(t, pool, "INSERT INTO teams (name) VALUES ('legal-team') RETURNING id")
	if _, err := pool.Exec(ctx, "INSERT INTO team_members (team_id, user_id, role, source) VALUES ($1, $2, 'member', 'manual')", team, bob.ID()); err != nil {
		t.Fatal(err)
	}
	robot := seed(t, pool, "INSERT INTO service_accounts (name) VALUES ('ingest-bot') RETURNING id")

	grant := func(d *domain.Domain, st domain.SubjectType, id string, role domain.Role) *domain.RoleAssignment {
		t.Helper()
		ra, err := svc.GrantRole(ctx, admin, d.ID, domain.GrantInput{SubjectType: st, SubjectID: id, Role: role})
		if err != nil {
			t.Fatalf("granting %s on %s: %v", role, d.Name, err)
		}
		return ra
	}
	aliceGrant := grant(payments, domain.SubjectUser, alice.ID(), domain.RoleSteward)
	grant(legal, domain.SubjectTeam, team, domain.RoleSteward)
	grant(payments, domain.SubjectUser, carol.ID(), domain.RoleDomainAdmin)
	grant(finance, domain.SubjectServiceAccount, robot, domain.RoleSteward)

	scopeOf := func(p auth.Principal) *domain.Scope {
		t.Helper()
		sc, err := svc.Scope(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return sc
	}

	for _, tc := range []struct {
		name   string
		p      auth.Principal
		action domain.Action
		d      *domain.Domain
		want   bool
	}{
		{"steward writes its domain", alice, domain.ActionWrite, payments, true},
		{"grants apply to the subtree", alice, domain.ActionWrite, cards, true},
		{"grants do not climb", alice, domain.ActionWrite, finance, false},
		{"grants do not cross", alice, domain.ActionWrite, legal, false},
		{"steward cannot administer", alice, domain.ActionAdmin, payments, false},
		{"team grant reaches its members", bob, domain.ActionWrite, legal, true},
		{"team grant stays in its domain", bob, domain.ActionWrite, finance, false},
		{"domain admin writes", carol, domain.ActionWrite, cards, true},
		{"domain admin administers the subtree", carol, domain.ActionAdmin, cards, true},
		{"service account grant", auth.NewServiceAccountPrincipal(robot, "ingest-bot", nil, nil), domain.ActionWrite, cards, true},
		{"operator is global", admin, domain.ActionAdmin, legal, true},
		{"native admin is global", auth.NewUserPrincipal(&user.User{ID: "x", Roles: []user.Role{{Name: auth.AdminRoleName}}}), domain.ActionAdmin, legal, true},
		{"no principal, no scope", nil, domain.ActionWrite, legal, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeOf(tc.p).Can(tc.action, tc.d.Path); got != tc.want {
				t.Fatalf("Can(%s, %s) = %v, want %v", tc.action, tc.d.Name, got, tc.want)
			}
		})
	}

	t.Run("a revocation applies to the next scope", func(t *testing.T) {
		if err := svc.RevokeRole(ctx, admin, payments.ID, aliceGrant.ID); err != nil {
			t.Fatal(err)
		}
		if scopeOf(alice).Can(domain.ActionWrite, payments.Path) {
			t.Fatal("revoked grant still applies")
		}
	})

	t.Run("a domain admin cannot grant above its subtree", func(t *testing.T) {
		_, err := svc.GrantRole(ctx, carol, finance.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: carol.ID(), Role: domain.RoleDomainAdmin})
		wantErr(t, err, domain.ErrForbidden)
		if _, err := svc.GrantRole(ctx, carol, cards.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: alice.ID(), Role: domain.RoleSteward}); err != nil {
			t.Fatalf("delegating inside its subtree: %v", err)
		}
		_, err = svc.GrantRole(ctx, bob, legal.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: bob.ID(), Role: domain.RoleDomainAdmin})
		wantErr(t, err, domain.ErrForbidden)
	})

	t.Run("invalid grants", func(t *testing.T) {
		in := domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: carol.ID(), Role: domain.RoleDomainAdmin}
		_, err := svc.GrantRole(ctx, admin, payments.ID, in)
		wantErr(t, err, domain.ErrDuplicate)
		_, err = svc.GrantRole(ctx, admin, payments.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: "00000000-0000-4000-8000-00000000ffff", Role: domain.RoleSteward})
		wantErr(t, err, domain.ErrEntityNotFound)
		_, err = svc.GrantRole(ctx, admin, payments.ID, domain.GrantInput{SubjectType: domain.SubjectUser, SubjectID: carol.ID(), Role: "owner"})
		wantErr(t, err, domain.ErrInvalidInput)
		wantErr(t, svc.RevokeRole(ctx, admin, legal.ID, aliceGrant.ID), domain.ErrNotFound)
	})

	t.Run("listing shows inherited roles and deleted subjects", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "UPDATE service_accounts SET deleted_at = now() WHERE id = $1", robot); err != nil {
			t.Fatal(err)
		}
		roles, err := svc.Roles(ctx, cards.ID)
		if err != nil {
			t.Fatal(err)
		}
		var sawInherited, sawMissing, sawOwn bool
		for _, r := range roles {
			if r.Inherited && r.SubjectName == "carol" && r.DomainName == "Payments" {
				sawInherited = true
			}
			if r.SubjectType == domain.SubjectServiceAccount && r.SubjectMissing {
				sawMissing = true
			}
			if !r.Inherited && r.SubjectName == "alice" {
				sawOwn = true
			}
		}
		if !sawInherited || !sawMissing || !sawOwn {
			t.Fatalf("roles = %+v", roles)
		}
	})
}
