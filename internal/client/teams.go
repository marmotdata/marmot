package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/marmotdata/marmot/internal/core/team"
)

// TeamsListResponse represents the response from the teams list endpoint.
type TeamsListResponse struct {
	Teams []team.Team `json:"teams"`
	Total int         `json:"total"`
}

// ListTeams lists all teams with pagination.
func (c *Client) ListTeams(ctx context.Context, limit, offset int) (*TeamsListResponse, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp TeamsListResponse
	if err := c.get(ctx, "/teams", q, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTeam retrieves a team by ID.
func (c *Client) GetTeam(ctx context.Context, id string) (*team.Team, error) {
	var t team.Team
	if err := c.get(ctx, "/teams/"+id, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTeamMembers lists the members of a team.
func (c *Client) ListTeamMembers(ctx context.Context, teamID string) ([]team.TeamMemberWithUser, error) {
	var members []team.TeamMemberWithUser
	if err := c.get(ctx, "/teams/"+teamID+"/members", nil, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// ListTeamsRaw lists teams and returns raw JSON.
func (c *Client) ListTeamsRaw(ctx context.Context, limit, offset int) ([]byte, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return c.getRaw(ctx, "/teams", q)
}

// GetTeamRaw retrieves a team as raw JSON.
func (c *Client) GetTeamRaw(ctx context.Context, id string) ([]byte, error) {
	return c.getRaw(ctx, "/teams/"+id, nil)
}

// ListTeamMembersRaw lists team members and returns raw JSON.
func (c *Client) ListTeamMembersRaw(ctx context.Context, teamID string) ([]byte, error) {
	return c.getRaw(ctx, "/teams/"+teamID+"/members", nil)
}
