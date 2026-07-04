package mcp

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/mcp"
)

// teamServiceAdapter adapts team.Service to mcp.TeamService
type teamServiceAdapter struct {
	teamService *team.Service
}

func toMCPTeam(t *team.Team) *mcp.Team {
	return &mcp.Team{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Tags:        t.Tags,
	}
}

func (a *teamServiceAdapter) GetTeam(ctx context.Context, id string) (*mcp.Team, error) {
	t, err := a.teamService.GetTeam(ctx, id)
	if err != nil {
		return nil, err
	}
	return toMCPTeam(t), nil
}

func (a *teamServiceAdapter) GetTeamByName(ctx context.Context, name string) (*mcp.Team, error) {
	t, err := a.teamService.GetTeamByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return toMCPTeam(t), nil
}

func (a *teamServiceAdapter) ListTeams(ctx context.Context, limit, offset int) ([]*mcp.Team, int, error) {
	teams, total, err := a.teamService.ListTeams(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	mcpTeams := make([]*mcp.Team, len(teams))
	for i, t := range teams {
		mcpTeams[i] = toMCPTeam(t)
	}
	return mcpTeams, total, nil
}

func (a *teamServiceAdapter) ListMembers(ctx context.Context, teamID string) ([]*mcp.TeamMember, error) {
	members, err := a.teamService.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	mcpMembers := make([]*mcp.TeamMember, len(members))
	for i, m := range members {
		mcpMembers[i] = &mcp.TeamMember{
			UserID:   m.UserID,
			Username: m.Username,
			Name:     m.Name,
			Email:    m.Email,
			Role:     m.Role,
		}
	}
	return mcpMembers, nil
}

func (a *teamServiceAdapter) ListUserTeams(ctx context.Context, userID string) ([]*mcp.Team, error) {
	teams, err := a.teamService.ListUserTeams(ctx, userID)
	if err != nil {
		return nil, err
	}

	mcpTeams := make([]*mcp.Team, len(teams))
	for i, t := range teams {
		mcpTeams[i] = toMCPTeam(t)
	}
	return mcpTeams, nil
}

func (a *teamServiceAdapter) FindSimilarTeamNames(ctx context.Context, searchTerm string, limit int) ([]string, error) {
	return a.teamService.FindSimilarTeamNames(ctx, searchTerm, limit)
}

func (a *teamServiceAdapter) ListAssetOwners(ctx context.Context, assetID string) ([]mcp.Owner, error) {
	owners, err := a.teamService.ListAssetOwners(ctx, assetID)
	if err != nil {
		return nil, err
	}

	mcpOwners := make([]mcp.Owner, len(owners))
	for i, owner := range owners {
		mcpOwners[i] = mcp.Owner{
			Type:  owner.Type,
			ID:    owner.ID,
			Name:  owner.Name,
			Email: owner.Email,
		}
	}
	return mcpOwners, nil
}
