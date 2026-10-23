package pluginserve

import (
	"context"
	"errors"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/core/user"
)

// Principals answers whether an enrolment's principal may still serve, from Marmot's users and service accounts. It is checked at every connection and every sweep, so deactivating a user, revoking their sessions, disabling a service account or taking away ingestion:serve closes their serving processes within one sweep.
type Principals struct {
	Users           user.Service
	ServiceAccounts serviceaccount.Service
}

func (p Principals) MayServe(ctx context.Context, e *Enrolment) (bool, error) {
	switch auth.PrincipalType(e.PrincipalType) {
	case auth.PrincipalTypeUser:
		u, err := p.Users.Get(ctx, e.PrincipalID)
		if errors.Is(err, user.ErrUserNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if auth.CheckUserSession(u, e.CreatedAt) != nil {
			return false, nil
		}
		return p.Users.HasPermission(ctx, u.ID, "ingestion", "serve")

	case auth.PrincipalTypeServiceAccount:
		if p.ServiceAccounts == nil {
			return false, nil
		}
		sa, err := p.ServiceAccounts.Get(ctx, e.PrincipalID)
		if errors.Is(err, serviceaccount.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !sa.Active {
			return false, nil
		}
		roleNames := make([]string, 0, len(sa.Roles))
		var perms []string
		for _, r := range sa.Roles {
			roleNames = append(roleNames, r.Name)
			for _, perm := range r.Permissions {
				perms = append(perms, perm.ResourceType+":"+perm.Action)
			}
		}
		return auth.NewServiceAccountPrincipal(sa.ID, sa.Name, roleNames, perms).HasPermission("ingestion", "serve"), nil

	case auth.PrincipalTypeOperator:
		return auth.NewOperatorPrincipal().HasPermission("ingestion", "serve"), nil
	}
	return false, nil
}
