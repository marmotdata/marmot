package importer

import (
	"context"
	"errors"
	"strings"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
)

type UserLookup interface {
	GetUserByUsername(ctx context.Context, username string) (*user.User, error)
}

type TeamLookup interface {
	GetTeamByName(ctx context.Context, name string) (*team.Team, error)
}

// ServiceOwners resolves owner cells with Marmot's user and team services:
// a username, or team:<name>.
type ServiceOwners struct {
	Users UserLookup
	Teams TeamLookup
}

const teamPrefix = "team:"

func (o ServiceOwners) ResolveOwner(ctx context.Context, ref string) (glossary.OwnerInput, error) {
	if name, ok := strings.CutPrefix(ref, teamPrefix); ok {
		t, err := o.Teams.GetTeamByName(ctx, strings.TrimSpace(name))
		if errors.Is(err, team.ErrTeamNotFound) {
			return glossary.OwnerInput{}, ErrOwnerNotFound
		}
		if err != nil {
			return glossary.OwnerInput{}, err
		}
		return glossary.OwnerInput{ID: t.ID, Type: "team"}, nil
	}
	u, err := o.Users.GetUserByUsername(ctx, ref)
	if errors.Is(err, user.ErrUserNotFound) {
		return glossary.OwnerInput{}, ErrOwnerNotFound
	}
	if err != nil {
		return glossary.OwnerInput{}, err
	}
	return glossary.OwnerInput{ID: u.ID, Type: "user"}, nil
}
