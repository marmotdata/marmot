package auth

import (
	"strings"

	"github.com/marmotdata/marmot/internal/core/user"
)

// AdminRoleName is the role that unconditionally passes permission checks.
// Must stay in sync with the `admin` row seeded in migration 000002.
const AdminRoleName = "admin"

// PrincipalType identifies the kind of principal a Marmot session represents.
type PrincipalType string

const (
	PrincipalTypeUser           PrincipalType = "user"
	PrincipalTypeOperator       PrincipalType = "operator"
	PrincipalTypeOIDCTrust      PrincipalType = "oidc_trust"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
)

// Principal is any entity that can hold roles/permissions and act against the API.
// The interface is deliberately narrow — anything user-specific belongs behind AsUser.
type Principal interface {
	// ID: user UUID, oidc-trust ID, or a fixed sentinel for operator.
	ID() string
	Type() PrincipalType
	DisplayName() string

	// AuditSubject returns a readable log/audit identifier (e.g. "user:alice").
	// Never a raw UUID — guardrail against UUID leakage into audit trails.
	AuditSubject() string

	Roles() []string

	// Permissions may be incomplete for admin principals. Use IsAdmin/HasPermission
	// to gate access; never iterate this slice for authorization decisions.
	Permissions() []string

	// IsAdmin bypasses fine-grained checks (Vault's root-token pattern).
	IsAdmin() bool
	HasPermission(resourceType, action string) bool

	// AsUser returns the underlying user record, or nil for machine principals.
	// User-only handlers (profile, personal API keys) should 403 on nil.
	AsUser() *user.User
}

type userPrincipal struct {
	u *user.User
}

// NewUserPrincipal converts at the middleware boundary. Handlers should not
// scatter this conversion — they receive a Principal from context.
func NewUserPrincipal(u *user.User) Principal {
	if u == nil {
		return nil
	}
	return userPrincipal{u: u}
}

func (p userPrincipal) ID() string          { return p.u.ID }
func (p userPrincipal) Type() PrincipalType { return PrincipalTypeUser }

func (p userPrincipal) DisplayName() string {
	if p.u.Name != "" {
		return p.u.Name
	}
	return p.u.Username
}

// AuditSubject percent-encodes `:` in the username so an unexpected colon
// cannot masquerade as the type/name delimiter. The users table has no
// explicit constraint against `:` — defense in depth.
func (p userPrincipal) AuditSubject() string {
	return "user:" + strings.ReplaceAll(p.u.Username, ":", "%3A")
}

func (p userPrincipal) Roles() []string {
	names := make([]string, len(p.u.Roles))
	for i, r := range p.u.Roles {
		names[i] = r.Name
	}
	return names
}

func (p userPrincipal) Permissions() []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, r := range p.u.Roles {
		for _, perm := range r.Permissions {
			key := perm.ResourceType + ":" + perm.Action
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

func (p userPrincipal) IsAdmin() bool {
	for _, r := range p.u.Roles {
		if r.Name == AdminRoleName {
			return true
		}
	}
	return false
}

// HasPermission reads pre-loaded role permissions from the *user.User struct.
// user.Service.Get eager-loads Roles[].Permissions[] — see internal/core/user/store.go
// GetUser. Callers that need a fresh check after a mid-session permission
// change should route through user.Service.HasPermission instead.
func (p userPrincipal) HasPermission(resourceType, action string) bool {
	if p.IsAdmin() {
		return true
	}
	for _, r := range p.u.Roles {
		for _, perm := range r.Permissions {
			if perm.ResourceType == resourceType && perm.Action == action {
				return true
			}
		}
	}
	return false
}

func (p userPrincipal) AsUser() *user.User { return p.u }

// operatorPrincipal replaces the GetOperatorUser singleton at the Principal
// boundary. The singleton user record is retained in internal/api/v1/common/auth.go
// for the flag-off path; Phase 6 removes it.
type operatorPrincipal struct{}

func NewOperatorPrincipal() Principal { return operatorPrincipal{} }

// OperatorPrincipalID matches the legacy singleton user UUID so audit trails
// and any user_id foreign keys remain linkable across the PRINCIPAL_V2 flip.
const OperatorPrincipalID = "00000000-0000-0000-0000-000000000001"

func (operatorPrincipal) ID() string                     { return OperatorPrincipalID }
func (operatorPrincipal) Type() PrincipalType            { return PrincipalTypeOperator }
func (operatorPrincipal) DisplayName() string            { return "Marmot Operator" }
func (operatorPrincipal) AuditSubject() string           { return "operator" }
func (operatorPrincipal) Roles() []string                { return []string{AdminRoleName} }
func (operatorPrincipal) Permissions() []string          { return nil }
func (operatorPrincipal) IsAdmin() bool                  { return true }
func (operatorPrincipal) HasPermission(_, _ string) bool { return true }
func (operatorPrincipal) AsUser() *user.User             { return nil }

type serviceAccountPrincipal struct {
	id          string
	name        string
	roleNames   []string
	permissions map[string]struct{}
	isAdmin     bool
}

func NewServiceAccountPrincipal(id, name string, roleNames []string, permKeys []string) Principal {
	perms := make(map[string]struct{}, len(permKeys))
	for _, k := range permKeys {
		perms[k] = struct{}{}
	}
	admin := false
	for _, r := range roleNames {
		if r == AdminRoleName {
			admin = true
			break
		}
	}
	return serviceAccountPrincipal{
		id:          id,
		name:        name,
		roleNames:   roleNames,
		permissions: perms,
		isAdmin:     admin,
	}
}

func (p serviceAccountPrincipal) ID() string          { return p.id }
func (p serviceAccountPrincipal) Type() PrincipalType { return PrincipalTypeServiceAccount }
func (p serviceAccountPrincipal) DisplayName() string { return p.name }
func (p serviceAccountPrincipal) AuditSubject() string {
	return "service_account:" + strings.ReplaceAll(p.name, ":", "%3A")
}
func (p serviceAccountPrincipal) Roles() []string { return p.roleNames }

func (p serviceAccountPrincipal) Permissions() []string {
	out := make([]string, 0, len(p.permissions))
	for k := range p.permissions {
		out = append(out, k)
	}
	return out
}

func (p serviceAccountPrincipal) IsAdmin() bool { return p.isAdmin }

func (p serviceAccountPrincipal) HasPermission(resourceType, action string) bool {
	if p.isAdmin {
		return true
	}
	_, ok := p.permissions[resourceType+":"+action]
	return ok
}

func (p serviceAccountPrincipal) AsUser() *user.User { return nil }

