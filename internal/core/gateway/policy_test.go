package gateway

import (
	"testing"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
)

type fakePrincipal struct {
	admin bool
}

func (p fakePrincipal) ID() string                                 { return "00000000-0000-0000-0000-000000000042" }
func (p fakePrincipal) Type() auth.PrincipalType                   { return auth.PrincipalTypeServiceAccount }
func (p fakePrincipal) DisplayName() string                        { return "test-agent" }
func (p fakePrincipal) AuditSubject() string                       { return "service_account:test-agent" }
func (p fakePrincipal) Roles() []string                            { return nil }
func (p fakePrincipal) Permissions() []string                      { return nil }
func (p fakePrincipal) IsAdmin() bool                              { return p.admin }
func (p fakePrincipal) HasPermission(resource, action string) bool { return p.admin }
func (p fakePrincipal) AsUser() *user.User                         { return nil }

func grant(id, selector string, actions ...string) *Grant {
	if len(actions) == 0 {
		actions = []string{ActionQuery}
	}
	return &Grant{ID: id, ResourceSelector: selector, Actions: actions}
}

func TestMatchSelector(t *testing.T) {
	cases := []struct {
		selector string
		mrn      string
		want     bool
	}{
		{"mrn://postgresql/ecommerce/**", "mrn://postgresql/ecommerce/orders", true},
		{"mrn://postgresql/ecommerce/**", "mrn://postgresql/ecommerce/public.orders", true},
		{"mrn://postgresql/ecommerce/**", "mrn://postgresql/other/orders", false},
		{"mrn://postgresql/*/orders", "mrn://postgresql/ecommerce/orders", true},
		{"mrn://postgresql/*/orders", "mrn://postgresql/a/b/orders", false},
		{"**", "mrn://anything/at/all", true},
		{"mrn://target/trino-local", "mrn://target/trino-local", true},
		{"mrn://target/trino-local", "mrn://target/trino-local-2", false},
		{"MRN://Postgresql/Ecommerce/**", "mrn://postgresql/ecommerce/orders", true},
		{"mrn://postgresql/ecommerce/orders", "MRN://POSTGRESQL/ECOMMERCE/ORDERS", true},
		// Regex metacharacters in selectors stay literal.
		{"mrn://postgresql/e.commerce/**", "mrn://postgresql/eXcommerce/orders", false},
	}
	for _, tc := range cases {
		if got := MatchSelector(tc.selector, tc.mrn); got != tc.want {
			t.Errorf("MatchSelector(%q, %q) = %v, want %v", tc.selector, tc.mrn, got, tc.want)
		}
	}
}

func TestDecide(t *testing.T) {
	agent := fakePrincipal{}
	admin := fakePrincipal{admin: true}

	t.Run("denies with no grants", func(t *testing.T) {
		d := Decide(agent, nil, []string{"mrn://postgresql/ecommerce/orders"})
		if d.Allowed {
			t.Fatal("expected deny")
		}
		if d.Reason == "" {
			t.Fatal("expected a deny reason")
		}
	})

	t.Run("allows when every resource is covered", func(t *testing.T) {
		grants := []*Grant{grant("g1", "mrn://postgresql/ecommerce/**")}
		d := Decide(agent, grants, []string{
			"mrn://postgresql/ecommerce/orders",
			"mrn://postgresql/ecommerce/customers",
		})
		if !d.Allowed {
			t.Fatalf("expected allow, got deny: %s", d.Reason)
		}
		if d.MatchedGrants["mrn://postgresql/ecommerce/orders"] != "g1" {
			t.Fatalf("expected g1 to match, got %v", d.MatchedGrants)
		}
	})

	t.Run("denies when any resource is uncovered", func(t *testing.T) {
		grants := []*Grant{grant("g1", "mrn://postgresql/ecommerce/orders")}
		d := Decide(agent, grants, []string{
			"mrn://postgresql/ecommerce/orders",
			"mrn://postgresql/hr/salaries",
		})
		if d.Allowed {
			t.Fatal("expected deny: one referenced resource has no grant")
		}
	})

	t.Run("ignores grants without the query action", func(t *testing.T) {
		grants := []*Grant{grant("g1", "**", "browse")}
		d := Decide(agent, grants, []string{"mrn://postgresql/ecommerce/orders"})
		if d.Allowed {
			t.Fatal("expected deny: grant lacks the query action")
		}
	})

	t.Run("admin bypasses grants", func(t *testing.T) {
		d := Decide(admin, nil, []string{"mrn://postgresql/hr/salaries"})
		if !d.Allowed {
			t.Fatal("expected admin to be allowed")
		}
	})
}
