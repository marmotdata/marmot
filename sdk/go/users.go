package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/users"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// User is a single user account.
type User = models.User

// UserList is a paginated set of users.
type UserList = models.ListUsersResponse

// UsersListOptions filters UsersService.List.
type UsersListOptions struct {
	Query   string
	Active  *bool
	RoleIDs []string
	Limit   int64
	Offset  int64
}

// CreateUserInput is the input for UsersService.Create.
type CreateUserInput struct {
	Name           string
	Username       string
	Password       string
	RoleNames      []string
	ProfilePicture string
}

// UpdateUserInput is the input for UsersService.Update.
type UpdateUserInput struct {
	Name           string
	Email          string
	Password       string
	RoleNames      []string
	ProfilePicture string
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

// Create creates a new user.
func (s *UsersService) Create(ctx context.Context, in CreateUserInput) (*User, error) {
	name, username := in.Name, in.Username
	body := &models.CreateUserInput{
		Name:           &name,
		Username:       &username,
		Password:       in.Password,
		RoleNames:      in.RoleNames,
		ProfilePicture: in.ProfilePicture,
	}
	p := users.NewPostUsersParams().WithContext(ctx).WithUser(body)
	resp, err := s.gen.Users.PostUsers(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Update modifies an existing user.
func (s *UsersService) Update(ctx context.Context, id string, in UpdateUserInput) (*User, error) {
	body := &models.UpdateUserInput{
		Name:           in.Name,
		Email:          in.Email,
		Password:       in.Password,
		RoleNames:      in.RoleNames,
		ProfilePicture: in.ProfilePicture,
	}
	p := users.NewPutUsersIDParams().WithContext(ctx).WithID(id).WithUser(body)
	resp, err := s.gen.Users.PutUsersID(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete removes a user.
func (s *UsersService) Delete(ctx context.Context, id string) error {
	p := users.NewDeleteUsersIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Users.DeleteUsersID(p)
	return mapErr(err)
}
