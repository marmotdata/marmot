package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/settings"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestInvitations_Postgres(t *testing.T) {
	runInvitationChecks(t, pgtest.TempDB(t))
}

// runInvitationChecks walks an invitation from creation to acceptance against a real schema, since most of what can go wrong is in the SQL: the status filters, the one-use spend and the token hashing.
func runInvitationChecks(t *testing.T, db *pgxpool.Pool) {
	ctx := context.Background()
	repo := NewPostgresRepository(db)
	store := settings.NewPostgresStore(db)
	svc := NewService(repo, WithSettings(store))

	inv, err := svc.Invite(ctx, InviteInput{Email: "joe.bloggs@example.com", RoleNames: []string{"user"}}, "")
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if inv.User.Status != StatusInvited || inv.User.Active || inv.User.Name != "joe.bloggs" || inv.User.InviteExpiresAt == nil {
		t.Fatalf("invited user = %+v", inv.User)
	}

	var stored string
	if err := db.QueryRow(ctx, `SELECT token_hash FROM user_invites WHERE user_id = $1`, inv.User.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == inv.Token || stored != hashInviteToken(inv.Token) {
		t.Fatal("the raw token was stored, or the stored hash does not match it")
	}

	if _, err := svc.Invite(ctx, InviteInput{Email: "joe.bloggs@example.com", RoleNames: []string{"user"}}, ""); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("inviting the same email twice: err = %v", err)
	}

	// An invited member cannot sign in with a password before accepting.
	if _, err := svc.Authenticate(ctx, "joe.bloggs@example.com", ""); err == nil {
		t.Fatal("invited member authenticated before accepting")
	}

	for status, want := range map[string]int{StatusInvited: 1, StatusActive: 1, StatusDeactivated: 0} {
		_, total, err := svc.List(ctx, Filter{Status: status, Limit: 50})
		if err != nil {
			t.Fatalf("List(%s): %v", status, err)
		}
		// The migrations create the admin user, the one active member.
		if total != want {
			t.Fatalf("List(%s) total = %d, want %d", status, total, want)
		}
	}
	if _, total, err := svc.List(ctx, Filter{NoTeam: true, Limit: 50}); err != nil || total != 2 {
		t.Fatalf("List(no team) = %d, %v; want 2", total, err)
	}

	renewed, err := svc.RenewInvite(ctx, inv.User.ID, "")
	if err != nil {
		t.Fatalf("RenewInvite: %v", err)
	}
	if _, err := svc.GetInvite(ctx, inv.Token); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("old link after renewal: err = %v", err)
	}
	info, err := svc.GetInvite(ctx, renewed.Token)
	if err != nil || info.Email != "joe.bloggs@example.com" || !info.PasswordSignIn {
		t.Fatalf("GetInvite = %+v, %v", info, err)
	}

	if _, err := svc.AcceptInvite(ctx, renewed.Token, "short", ""); err == nil {
		t.Fatal("accepted a password under 8 characters")
	}
	accepted, err := svc.AcceptInvite(ctx, renewed.Token, "a-long-password", "Joe Bloggs")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if accepted.Status != StatusActive || !accepted.Active || accepted.Name != "Joe Bloggs" {
		t.Fatalf("accepted user = %+v", accepted)
	}
	if _, err := svc.AcceptInvite(ctx, renewed.Token, "a-long-password", ""); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("second use of a link: err = %v", err)
	}
	if _, err := svc.Authenticate(ctx, "joe.bloggs@example.com", "a-long-password"); err != nil {
		t.Fatalf("Authenticate after accepting: %v", err)
	}

	// An expired link is refused, and withdrawing the invitation removes the pending account.
	late, err := svc.Invite(ctx, InviteInput{Email: "late@example.com", RoleNames: []string{"user"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE user_invites SET expires_at = NOW() - interval '1 minute' WHERE user_id = $1`, late.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetInvite(ctx, late.Token); !errors.Is(err, ErrInviteExpired) {
		t.Fatalf("expired link: err = %v", err)
	}
	if err := svc.RevokeInvite(ctx, late.User.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	if _, err := svc.Get(ctx, late.User.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("pending account after withdrawal: err = %v", err)
	}
	if err := svc.RevokeInvite(ctx, accepted.ID); !errors.Is(err, ErrNotInvited) {
		t.Fatalf("withdrawing an accepted invitation: err = %v", err)
	}

	// Single sign-on claims an invitation for the same address, even after the link expired.
	sso, err := svc.Invite(ctx, InviteInput{Email: "Sam@Example.com", RoleNames: []string{"user"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE user_invites SET expires_at = NOW() - interval '1 day' WHERE user_id = $1`, sso.User.ID); err != nil {
		t.Fatal(err)
	}
	linked, created, err := svc.ProvisionSSO(ctx, CreateUserInput{
		Username: "sam@example.com", Name: "Sam", OAuthProvider: "okta", OAuthProviderID: "sub-1",
		OAuthProviderData: map[string]interface{}{"email_verified": true},
	})
	if err != nil || created || linked.ID != sso.User.ID || linked.Status != StatusActive {
		t.Fatalf("ProvisionSSO for an invited address = %+v, %v, %v", linked, created, err)
	}

	// With sign-up off, someone uninvited is refused; with it on they get the default role.
	if err := store.Set(ctx, settings.KeySignIn, settings.SignIn{CreateOnFirstSSO: false, DefaultRole: "user", AllowPasswordSignIn: true}, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ProvisionSSO(ctx, CreateUserInput{Username: "stranger@example.com", Name: "S", OAuthProvider: "okta", OAuthProviderID: "sub-2"}); !errors.Is(err, ErrSignupDisabled) {
		t.Fatalf("uninvited sign-in with sign-up off: err = %v", err)
	}
	if err := store.Set(ctx, settings.KeySignIn, settings.SignIn{CreateOnFirstSSO: true, DefaultRole: "admin", AllowPasswordSignIn: false}, ""); err != nil {
		t.Fatal(err)
	}
	fresh, created, err := svc.ProvisionSSO(ctx, CreateUserInput{Username: "new@example.com", Name: "New", OAuthProvider: "okta", OAuthProviderID: "sub-3", RoleNames: []string{"user"}})
	if err != nil || !created || len(fresh.Roles) != 1 || fresh.Roles[0].Name != "admin" {
		t.Fatalf("ProvisionSSO with a default role = %+v, %v, %v", fresh, created, err)
	}

	// Password sign-in is off now, but administrators keep it.
	if _, err := svc.Authenticate(ctx, "joe.bloggs@example.com", "a-long-password"); !errors.Is(err, ErrPasswordSignInDisabled) {
		t.Fatalf("password sign-in while off: err = %v", err)
	}
	if err := svc.SetRolesFromSSO(ctx, accepted.ID, []string{"admin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, "joe.bloggs@example.com", "a-long-password"); err != nil {
		t.Fatalf("administrator password sign-in while off: %v", err)
	}

	tracker := NewLastSeenTracker(repo)
	if err := repo.TouchLastSeen(ctx, accepted.ID, time.Minute); err != nil {
		t.Fatal(err)
	}
	tracker.Touch("") // a no-op, not a panic
	got, err := svc.GetMember(ctx, accepted.ID)
	if err != nil || got.LastSeenAt == nil || time.Since(*got.LastSeenAt) > time.Minute {
		t.Fatalf("last seen = %+v, %v", got.LastSeenAt, err)
	}

	counts, err := svc.CountIdentitiesByProvider(ctx)
	if err != nil || counts["okta"] != 2 {
		t.Fatalf("identities by provider = %v, %v", counts, err)
	}
}
