package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/service_accounts"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// ServiceAccount is a machine principal with roles and API keys.
type ServiceAccount = models.ServiceAccount

// ServiceAccountAPIKey is an API key belonging to a service account.
type ServiceAccountAPIKey = models.ServiceAccountAPIKey

// CreateServiceAccountInput is the input for ServiceAccountsService.Create.
type CreateServiceAccountInput struct {
	Name        string
	Description string
	RoleIDs     []string
}

// UpdateServiceAccountInput is the input for ServiceAccountsService.Update.
type UpdateServiceAccountInput struct {
	Name        string
	Description string
	Active      bool
	RoleIDs     []string
}

// CreateServiceAccountAPIKeyInput is the input for ServiceAccountsService.CreateAPIKey.
type CreateServiceAccountAPIKeyInput struct {
	Name          string
	ExpiresInDays int64
}

// ServiceAccountsService manages service accounts and their API keys.
type ServiceAccountsService struct {
	gen *apiclient.Marmot
}

// List returns all service accounts.
func (s *ServiceAccountsService) List(ctx context.Context) ([]*ServiceAccount, error) {
	p := service_accounts.NewGetServiceAccountsParams().WithContext(ctx)
	resp, err := s.gen.ServiceAccounts.GetServiceAccountsContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Get fetches a service account by ID.
func (s *ServiceAccountsService) Get(ctx context.Context, id string) (*ServiceAccount, error) {
	p := service_accounts.NewGetServiceAccountsIDParams().WithContext(ctx).WithID(id)
	resp, err := s.gen.ServiceAccounts.GetServiceAccountsIDContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Create creates a new service account.
func (s *ServiceAccountsService) Create(ctx context.Context, in CreateServiceAccountInput) (*ServiceAccount, error) {
	body := &models.CreateServiceAccountRequest{
		Name:        in.Name,
		Description: in.Description,
		RoleIds:     in.RoleIDs,
	}
	p := service_accounts.NewPostServiceAccountsParams().WithContext(ctx).WithAccount(body)
	resp, err := s.gen.ServiceAccounts.PostServiceAccountsContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Update modifies a service account.
func (s *ServiceAccountsService) Update(ctx context.Context, id string, in UpdateServiceAccountInput) (*ServiceAccount, error) {
	body := &models.UpdateServiceAccountRequest{
		Name:        in.Name,
		Description: in.Description,
		Active:      in.Active,
		RoleIds:     in.RoleIDs,
	}
	p := service_accounts.NewPatchServiceAccountsIDParams().WithContext(ctx).WithID(id).WithAccount(body)
	resp, err := s.gen.ServiceAccounts.PatchServiceAccountsIDContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Delete soft-deletes a service account.
func (s *ServiceAccountsService) Delete(ctx context.Context, id string) error {
	p := service_accounts.NewDeleteServiceAccountsIDParams().WithContext(ctx).WithID(id)
	_, err := s.gen.ServiceAccounts.DeleteServiceAccountsIDContext(ctx, p)
	return mapErr(err)
}

// ListAPIKeys returns all API keys for a service account.
func (s *ServiceAccountsService) ListAPIKeys(ctx context.Context, saID string) ([]*ServiceAccountAPIKey, error) {
	p := service_accounts.NewGetServiceAccountsIDAPIKeysParams().WithContext(ctx).WithID(saID)
	resp, err := s.gen.ServiceAccounts.GetServiceAccountsIDAPIKeysContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// CreateAPIKey creates a new API key for a service account. The plaintext key is only returned once.
func (s *ServiceAccountsService) CreateAPIKey(ctx context.Context, saID string, in CreateServiceAccountAPIKeyInput) (*ServiceAccountAPIKey, error) {
	body := &models.CreateServiceAccountAPIKeyRequest{
		Name:          in.Name,
		ExpiresInDays: in.ExpiresInDays,
	}
	p := service_accounts.NewPostServiceAccountsIDAPIKeysParams().WithContext(ctx).WithID(saID).WithKey(body)
	resp, err := s.gen.ServiceAccounts.PostServiceAccountsIDAPIKeysContext(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// DeleteAPIKey deletes an API key from a service account.
func (s *ServiceAccountsService) DeleteAPIKey(ctx context.Context, saID, keyID string) error {
	p := service_accounts.NewDeleteServiceAccountsIDAPIKeysKeyIDParams().WithContext(ctx).WithID(saID).WithKeyID(keyID)
	_, err := s.gen.ServiceAccounts.DeleteServiceAccountsIDAPIKeysKeyIDContext(ctx, p)
	return mapErr(err)
}
