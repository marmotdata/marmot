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

func (a *teamServiceAdapter) GetTeam(ctx context.Context, id string) (*mcp.Team, error) {
	t, err := a.teamService.GetTeam(ctx, id)
	if err != nil {
		return nil, err
	}
	return &mcp.Team{ID: t.ID, Name: t.Name}, nil
}

func (a *teamServiceAdapter) GetTeamByName(ctx context.Context, name string) (*mcp.Team, error) {
	t, err := a.teamService.GetTeamByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return &mcp.Team{ID: t.ID, Name: t.Name}, nil
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
