package domain

import (
	"context"
	"fmt"
)

// PipelineAssignment is where a pipeline puts the assets it creates, and how
// many assets it ingested earlier are still in that domain.
type PipelineAssignment struct {
	ScheduleID     string `json:"schedule_id"`
	DomainID       string `json:"domain_id"`
	AssetsInDomain int    `json:"assets_in_domain"`
}

type PipelineMoveResult struct {
	DomainID    string `json:"domain_id"`
	MovedAssets int    `json:"moved_assets"`
}

func (s *service) PipelineAssignment(ctx context.Context, scheduleID string) (*PipelineAssignment, error) {
	exists, err := s.repo.EntityExists(ctx, KindIngestionSchedule, scheduleID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrEntityNotFound
	}
	domainID, err := s.repo.DomainOf(ctx, KindIngestionSchedule, scheduleID)
	if err != nil {
		return nil, err
	}
	ids, err := s.repo.PipelineAssetsIn(ctx, scheduleID, domainID)
	if err != nil {
		return nil, err
	}
	return &PipelineAssignment{ScheduleID: scheduleID, DomainID: domainID, AssetsInDomain: len(ids)}, nil
}

// AssignPipeline sets a pipeline's domain. Its already ingested assets only
// follow when moveAssets is set, and only those still in the pipeline's
// previous domain: an asset a steward placed elsewhere stays there.
func (s *service) AssignPipeline(ctx context.Context, scheduleID, domainID string, moveAssets bool) (*PipelineMoveResult, error) {
	if domainID != UnassignedID {
		if _, err := s.repo.Get(ctx, domainID); err != nil {
			return nil, err
		}
	}
	moved, err := s.repo.AssignPipeline(ctx, scheduleID, domainID, moveAssets)
	if err != nil {
		return nil, fmt.Errorf("assigning pipeline domain: %w", err)
	}
	return &PipelineMoveResult{DomainID: domainID, MovedAssets: moved}, nil
}
