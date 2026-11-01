package dataquery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
)

// fakePrincipal is a service account that is either an admin or holds only assets:query.
type fakePrincipal struct {
	admin    bool
	canQuery bool
}

func (p fakePrincipal) ID() string               { return "00000000-0000-0000-0000-000000000042" }
func (p fakePrincipal) Type() auth.PrincipalType { return auth.PrincipalTypeServiceAccount }
func (p fakePrincipal) DisplayName() string      { return "test-agent" }
func (p fakePrincipal) AuditSubject() string     { return "service_account:test-agent" }
func (p fakePrincipal) Roles() []string          { return nil }
func (p fakePrincipal) Permissions() []string    { return nil }
func (p fakePrincipal) IsAdmin() bool            { return p.admin }
func (p fakePrincipal) AsUser() *user.User       { return nil }
func (p fakePrincipal) HasPermission(resource, action string) bool {
	return p.admin || (p.canQuery && resource == "assets" && action == "query")
}

const (
	orders   = "mrn://table/postgresql/orders"
	salaries = "mrn://table/postgresql/salaries"
)

func TestPermissionAuthorizer(t *testing.T) {
	resources := []string{orders}

	d, err := permissionAuthorizer{}.Authorize(context.Background(), fakePrincipal{canQuery: true}, &Source{}, resources)
	require.NoError(t, err)
	assert.True(t, d.Allowed)

	d, err = permissionAuthorizer{}.Authorize(context.Background(), fakePrincipal{}, &Source{}, resources)
	require.NoError(t, err)
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "assets:query")
}

func TestSourceMRN(t *testing.T) {
	assert.Equal(t, "mrn://source/warehouse", SourceMRN("Warehouse"))
	assert.True(t, isSourceMRN(SourceMRN("warehouse")))
	assert.False(t, isSourceMRN(orders))
}
