package auth

import (
	"slices"
	"sort"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
)

// Pinning the wire values: these strings are emitted in JWT claims, so a
// silent rename would invalidate every token in flight.
func TestPrincipalTypeConstants(t *testing.T) {
	cases := []struct {
		got  PrincipalType
		want string
	}{
		{PrincipalTypeUser, "user"},
		{PrincipalTypeOperator, "operator"},
		{PrincipalTypeOIDCTrust, "oidc_trust"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("PrincipalType %q, want %q", c.got, c.want)
		}
	}
}

func TestUserPrincipal(t *testing.T) {
	u := &user.User{
		ID:       "user-123",
		Username: "alice",
		Name:     "Alice Example",
		Active:   true,
		Roles: []user.Role{
			{
				Name: "editor",
				Permissions: []user.Permission{
					{ResourceType: "assets", Action: "read"},
					{ResourceType: "assets", Action: "write"},
				},
			},
			{
				Name: "viewer",
				Permissions: []user.Permission{
					{ResourceType: "assets", Action: "read"}, // duplicate — must dedup
					{ResourceType: "lineage", Action: "read"},
				},
			},
		},
	}

	p := NewUserPrincipal(u)
	if p == nil {
		t.Fatal("NewUserPrincipal returned nil for non-nil user")
	}

	if got, want := p.ID(), "user-123"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
	if got, want := p.Type(), PrincipalTypeUser; got != want {
		t.Errorf("Type() = %q, want %q", got, want)
	}
	if got, want := p.DisplayName(), "Alice Example"; got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
	if got, want := p.AuditSubject(), "user:alice"; got != want {
		t.Errorf("AuditSubject() = %q, want %q", got, want)
	}

	if p.IsAdmin() {
		t.Errorf("IsAdmin() = true, want false")
	}
	if !p.HasPermission("assets", "read") {
		t.Errorf("HasPermission(assets, read) = false, want true")
	}
	if !p.HasPermission("lineage", "read") {
		t.Errorf("HasPermission(lineage, read) = false, want true")
	}
	if p.HasPermission("assets", "delete") {
		t.Errorf("HasPermission(assets, delete) = true, want false")
	}

	gotRoles := append([]string(nil), p.Roles()...)
	sort.Strings(gotRoles)
	wantRoles := []string{"editor", "viewer"}
	if !slices.Equal(gotRoles, wantRoles) {
		t.Errorf("Roles() = %v, want %v", gotRoles, wantRoles)
	}

	gotPerms := append([]string(nil), p.Permissions()...)
	sort.Strings(gotPerms)
	wantPerms := []string{"assets:read", "assets:write", "lineage:read"}
	if !slices.Equal(gotPerms, wantPerms) {
		t.Errorf("Permissions() = %v, want %v", gotPerms, wantPerms)
	}

	if p.AsUser() != u {
		t.Errorf("AsUser() did not return the wrapped user")
	}
}

func TestUserPrincipal_DisplayNameFallsBackToUsername(t *testing.T) {
	p := NewUserPrincipal(&user.User{Username: "bob"})
	if got, want := p.DisplayName(), "bob"; got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
}

func TestUserPrincipal_AdminGetsAllPermissions(t *testing.T) {
	p := NewUserPrincipal(&user.User{
		Username: "root",
		Roles:    []user.Role{{Name: AdminRoleName}},
	})
	if !p.IsAdmin() {
		t.Fatal("IsAdmin() = false, want true")
	}
	if !p.HasPermission("anything", "anywhere") {
		t.Errorf("admin should short-circuit HasPermission")
	}
}

func TestUserPrincipal_AuditSubject_EscapesColons(t *testing.T) {
	p := NewUserPrincipal(&user.User{Username: "weird:name"})
	got := p.AuditSubject()
	if got == "user:weird:name" {
		t.Errorf("AuditSubject() left `:` unescaped: %q", got)
	}
}

func TestNewUserPrincipal_NilUser(t *testing.T) {
	if p := NewUserPrincipal(nil); p != nil {
		t.Errorf("NewUserPrincipal(nil) = %v, want nil", p)
	}
}

func TestOperatorPrincipal(t *testing.T) {
	p := NewOperatorPrincipal()

	if got, want := p.ID(), OperatorPrincipalID; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
	if got, want := p.Type(), PrincipalTypeOperator; got != want {
		t.Errorf("Type() = %q, want %q", got, want)
	}
	if got, want := p.DisplayName(), "Marmot Operator"; got != want {
		t.Errorf("DisplayName() = %q, want %q", got, want)
	}
	if got, want := p.AuditSubject(), "operator"; got != want {
		t.Errorf("AuditSubject() = %q, want %q", got, want)
	}
	if got, want := p.Roles(), []string{"admin"}; !slices.Equal(got, want) {
		t.Errorf("Roles() = %v, want %v", got, want)
	}
	if p.AsUser() != nil {
		t.Errorf("AsUser() should be nil for operator")
	}
	if got := p.Permissions(); len(got) != 0 {
		t.Errorf("Permissions() = %v, want empty", got)
	}
	if !p.IsAdmin() {
		t.Errorf("IsAdmin() = false, want true")
	}
	if !p.HasPermission("assets", "delete") {
		t.Errorf("HasPermission bypass failed")
	}
}

// Pinned so a future refactor cannot break the audit trail linking the
// operator Principal back to the legacy singleton user_id.
func TestOperatorPrincipalID_MatchesLegacySingleton(t *testing.T) {
	const legacy = "00000000-0000-0000-0000-000000000001"
	if OperatorPrincipalID != legacy {
		t.Errorf("OperatorPrincipalID = %q, want %q", OperatorPrincipalID, legacy)
	}
}

func TestAuditSubject_NeverLeaksUUID(t *testing.T) {
	u := &user.User{ID: "11111111-2222-3333-4444-555555555555", Username: "carol"}
	if got := NewUserPrincipal(u).AuditSubject(); got == u.ID {
		t.Errorf("AuditSubject() leaked raw UUID: %q", got)
	}
	if got := NewOperatorPrincipal().AuditSubject(); got == OperatorPrincipalID {
		t.Errorf("operator AuditSubject() leaked raw UUID: %q", got)
	}
}
