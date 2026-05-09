package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
)

// EdgeTypeAgentLookup is the lineage type emitted for tool-call observations
// where the agent touched a catalogued asset.
const EdgeTypeAgentLookup = "AGENT_LOOKUP"

// RunInput is what the SDK posts when an agent run completes.
type RunInput struct {
	AgentMRN  string          `json:"agent_mrn"`
	RunID     string          `json:"run_id"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Status    string          `json:"status"`
	Model     string          `json:"model,omitempty"`
	TokensIn  int             `json:"tokens_in"`
	TokensOut int             `json:"tokens_out"`
	Error     string          `json:"error,omitempty"`
	ToolCalls []ToolCallInput `json:"tool_calls,omitempty"`
	// ObservedAssets are MRNs the agent touched at runtime that aren't already
	// surfaced through ToolCalls' target_mrn — typically extracted from tool
	// outputs (e.g. catalog-traversal tools that return MRNs). The server
	// emits an AGENT_LOOKUP observed edge for each.
	ObservedAssets []string `json:"observed_assets,omitempty"`
}

type ToolCallInput struct {
	ToolName   string    `json:"tool_name"`
	TargetMRN  string    `json:"target_mrn,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs *int      `json:"duration_ms,omitempty"`
	Status     string    `json:"status"`
}

type Service interface {
	RecordRun(ctx context.Context, input RunInput) (*Run, error)
	ListRuns(ctx context.Context, assetID string, period time.Duration, limit int) ([]*Run, error)
	BucketRuns(ctx context.Context, assetID string, period time.Duration) ([]Bucket, error)
	Stats(ctx context.Context, assetID string, period time.Duration) (*Stats, error)
}

type service struct {
	repo       Repository
	assetSvc   asset.Service
	lineageSvc lineage.Service
}

func NewService(repo Repository, assetSvc asset.Service, lineageSvc lineage.Service) Service {
	return &service{repo: repo, assetSvc: assetSvc, lineageSvc: lineageSvc}
}

func (s *service) RecordRun(ctx context.Context, in RunInput) (*Run, error) {
	if in.AgentMRN == "" {
		return nil, fmt.Errorf("agent_mrn is required")
	}
	if in.RunID == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	if in.StartedAt.IsZero() {
		return nil, fmt.Errorf("started_at is required")
	}
	if in.Status == "" {
		return nil, fmt.Errorf("status is required")
	}

	agent, err := s.assetSvc.GetByMRN(ctx, in.AgentMRN)
	if err != nil {
		return nil, fmt.Errorf("looking up agent asset: %w", err)
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}

	run := &Run{
		AgentID:   agent.ID,
		RunID:     in.RunID,
		StartedAt: in.StartedAt,
		EndedAt:   in.EndedAt,
		Status:    in.Status,
		Model:     in.Model,
		TokensIn:  in.TokensIn,
		TokensOut: in.TokensOut,
		Error:     in.Error,
	}
	if in.EndedAt != nil {
		ms := int(in.EndedAt.Sub(in.StartedAt) / time.Millisecond)
		run.DurationMs = &ms
	}

	for i, tc := range in.ToolCalls {
		run.ToolCalls = append(run.ToolCalls, ToolCall{
			Ordinal:    i,
			ToolName:   tc.ToolName,
			TargetMRN:  tc.TargetMRN,
			StartedAt:  tc.StartedAt,
			DurationMs: tc.DurationMs,
			Status:     tc.Status,
		})
	}

	if err := s.repo.InsertRun(ctx, run); err != nil {
		return nil, err
	}

	// Emit one observed lineage edge per tool call that resolved to a real asset
	// plus any explicitly reported observed_assets (e.g. MRNs walked out of a
	// tool's output by the SDK). Repeated lookups bump observation_count via
	// the unique partial index.
	seen := make(map[string]struct{})
	var observed []lineage.ObservedEdge
	push := func(mrn string) {
		if mrn == "" {
			return
		}
		if _, ok := seen[mrn]; ok {
			return
		}
		seen[mrn] = struct{}{}
		observed = append(observed, lineage.ObservedEdge{
			Source: mrn,
			Target: in.AgentMRN,
			Type:   EdgeTypeAgentLookup,
		})
	}
	for _, tc := range in.ToolCalls {
		push(tc.TargetMRN)
	}
	for _, mrn := range in.ObservedAssets {
		push(mrn)
	}
	if len(observed) > 0 {
		if err := s.lineageSvc.BatchObservedLineage(ctx, observed); err != nil {
			// Lineage failure should not unwind the run record — log and move on.
			return run, fmt.Errorf("recording observed lineage: %w", err)
		}
	}

	return run, nil
}

func (s *service) ListRuns(ctx context.Context, assetID string, period time.Duration, limit int) ([]*Run, error) {
	since := time.Now().Add(-period)
	return s.repo.ListRuns(ctx, assetID, since, limit)
}

func (s *service) BucketRuns(ctx context.Context, assetID string, period time.Duration) ([]Bucket, error) {
	since := time.Now().Add(-period)
	return s.repo.BucketRuns(ctx, assetID, since)
}

func (s *service) Stats(ctx context.Context, assetID string, period time.Duration) (*Stats, error) {
	since := time.Now().Add(-period)
	return s.repo.Stats(ctx, assetID, since)
}

// IsNotFound returns true if err is one of the package's not-found sentinels.
// Useful for handlers that translate to 404.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrAgentNotFound) || errors.Is(err, ErrRunNotFound)
}
