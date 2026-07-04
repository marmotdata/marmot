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

// CreateTeamInput is the input for TeamsService.Create.
type CreateTeamInput struct {
	Name        string
	Description string
}

// UpdateTeamInput is the input for TeamsService.Update.
type UpdateTeamInput struct {
	Name        string
	Description string
	Metadata    map[string]any
	Tags        []string
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

// Create creates a new team.
func (s *TeamsService) Create(ctx context.Context, in CreateTeamInput) (*Team, error) {
	body := &models.CreateTeamRequest{
		Name:        in.Name,
		Description: in.Description,
	}
	p := teams.NewPostTeamsParams().WithContext(ctx).WithTeam(body)
	resp, err := s.gen.Teams.PostTeams(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Update modifies an existing team. The update endpoint returns only a status
// message, so Update re-fetches the team to return its current state.
func (s *TeamsService) Update(ctx context.Context, id string, in UpdateTeamInput) (*Team, error) {
	body := &models.UpdateTeamRequest{
		Name:        in.Name,
		Description: in.Description,
		Tags:        in.Tags,
	}
	if len(in.Metadata) > 0 {
		body.Metadata = in.Metadata
	}
	p := teams.NewPutTeamsIDParams().WithContext(ctx).WithID(id).WithTeam(body)
	if _, err := s.gen.Teams.PutTeamsID(p); err != nil {
		return nil, mapErr(err)
	}
	return s.Get(ctx, id)
}

// Delete removes a team.
func (s *TeamsService) Delete(ctx context.Context, id string) error {
	p := teams.NewDeleteTeamsIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Teams.DeleteTeamsID(p)
	return mapErr(err)
}
