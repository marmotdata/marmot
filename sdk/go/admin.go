package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/admin"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// ReindexAccepted is the response from AdminService.Reindex.
type ReindexAccepted = models.V1AdminReindexAcceptedResponse

// ReindexStatus reports reindex progress.
type ReindexStatus = models.V1AdminReindexStatusResponse

// AdminService exposes administrative operations.
type AdminService struct {
	gen *apiclient.Marmot
}

// Reindex triggers a full search reindex.
func (s *AdminService) Reindex(ctx context.Context) (*ReindexAccepted, error) {
	p := admin.NewPostAdminSearchReindexParams().WithContext(ctx)
	resp, err := s.gen.Admin.PostAdminSearchReindex(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// ReindexStatus returns the current reindex progress.
func (s *AdminService) ReindexStatus(ctx context.Context) (*ReindexStatus, error) {
	p := admin.NewGetAdminSearchReindexParams().WithContext(ctx)
	resp, err := s.gen.Admin.GetAdminSearchReindex(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}
