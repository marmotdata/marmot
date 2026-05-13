package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/teams"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// Team is a single team.
type Team = models.Team

// TeamList is a paginated set of teams.
type TeamList = models.ListTeamsResponse

// TeamMembers is the response from TeamsService.Members.
type TeamMembers = models.ListMembersResponse

// TeamsListOptions paginates TeamsService.List.
type TeamsListOptions struct {
	Limit  int64
	Offset int64
}

// TeamsService lists teams and their members.
type TeamsService struct {
	gen *apiclient.Marmot
}

// List returns paginated teams.
func (s *TeamsService) List(ctx context.Context, opts TeamsListOptions) (*TeamList, error) {
	p := teams.NewGetTeamsParams().WithContext(ctx)
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Teams.GetTeams(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a team by ID.
func (s *TeamsService) Get(ctx context.Context, id string) (*Team, error) {
	p := teams.NewGetTeamsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Teams.GetTeamsID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Members returns the members of a team.
func (s *TeamsService) Members(ctx context.Context, id string) (*TeamMembers, error) {
	p := teams.NewGetTeamsIDMembersParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Teams.GetTeamsIDMembers(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
