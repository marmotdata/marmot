package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/users"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// User is a single user account.
type User = models.UserUser

// UserList is a paginated set of users.
type UserList = models.V1UsersListUsersResponse

// UsersListOptions filters UsersService.List.
type UsersListOptions struct {
	Query   string
	Active  *bool
	RoleIDs []string
	Limit   int64
	Offset  int64
}

// UsersService exposes user listing and identity queries.
type UsersService struct {
	gen *apiclient.Marmot
}

// List returns paginated users.
func (s *UsersService) List(ctx context.Context, opts UsersListOptions) (*UserList, error) {
	p := users.NewGetUsersParams().WithContext(ctx)
	if opts.Query != "" {
		p = p.WithQuery(&opts.Query)
	}
	if opts.Active != nil {
		p = p.WithActive(opts.Active)
	}
	if len(opts.RoleIDs) > 0 {
		p = p.WithRoleIds(opts.RoleIDs)
	}
	if opts.Limit > 0 {
		p = p.WithLimit(&opts.Limit)
	}
	if opts.Offset > 0 {
		p = p.WithOffset(&opts.Offset)
	}
	resp, err := s.gen.Users.GetUsers(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a user by ID.
func (s *UsersService) Get(ctx context.Context, id string) (*User, error) {
	p := users.NewGetUsersIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.Users.GetUsersID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Me returns the currently authenticated user.
func (s *UsersService) Me(ctx context.Context) (*User, error) {
	p := users.NewGetUsersMeParams().WithContext(ctx)
	resp, err := s.gen.Users.GetUsersMe(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
