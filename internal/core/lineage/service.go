package lineage

import (
	"context"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

// LineageChangeObserver is notified when lineage edges are created or deleted.
// Observers must be registered via SetLineageChangeObserver before any lineage
// mutations occur (i.e., during server initialization, before Start is called).
type LineageChangeObserver interface {
	OnEdgeCreated(ctx context.Context, sourceMRN, targetMRN, edgeType string)
	OnEdgeDeleted(ctx context.Context, sourceMRN, targetMRN string)
}

type Service interface {
	GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error)
	CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string, lineageType string) (string, error)
	EdgeExists(ctx context.Context, source, target string) (bool, error)
	DeleteDirectLineage(ctx context.Context, edgeID string) error
	GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error)
	GetImmediateNeighbors(ctx context.Context, assetMRN string, direction string) ([]string, error)
	SetLineageChangeObserver(observer LineageChangeObserver)
	ProcessOpenLineageEvent(ctx context.Context, event *RunEvent, createdBy string) error
	StoreRunHistory(ctx context.Context, entry *RunHistoryEntry) error
}

type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, err error, fields ...interface{})
}

type MetricsClient interface {
	Count(name string, value int64, tags ...string)
	Timing(name string, value time.Duration, tags ...string)
}

type service struct {
	repo            Repository
	validator       *validator.Validate
	metrics         MetricsClient
	assetSvc        asset.Service
	lineageObserver LineageChangeObserver
}

type ServiceOption func(*service)

func NewService(repo Repository, assetSvc asset.Service, opts ...ServiceOption) Service {
	s := &service{
		repo:      repo,
		validator: validator.New(),
		assetSvc:  assetSvc,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func WithMetrics(metrics MetricsClient) ServiceOption {
	return func(s *service) {
		s.metrics = metrics
	}
}

func (s *service) GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error) {
	return s.repo.GetAssetLineage(ctx, assetID, limit, direction)
}

func (s *service) GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error) {
	return s.repo.GetDirectLineage(ctx, edgeID)
}

func (s *service) CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string, lineageType string) (string, error) {
	existed, err := s.repo.EdgeExists(ctx, sourceMRN, targetMRN)
	if err != nil {
		return "", err
	}

	edgeID, err := s.repo.CreateDirectLineage(ctx, sourceMRN, targetMRN, lineageType)
	if err != nil {
		return "", err
	}

	if !existed && s.lineageObserver != nil {
		s.lineageObserver.OnEdgeCreated(ctx, sourceMRN, targetMRN, lineageType)
	}

	return edgeID, nil
}

func (s *service) DeleteDirectLineage(ctx context.Context, edgeID string) error {
	var sourceMRN, targetMRN string
	if s.lineageObserver != nil {
		edge, err := s.repo.GetDirectLineage(ctx, edgeID)
		if err != nil {
			log.Warn().Err(err).Str("edge_id", edgeID).Msg("Failed to get edge details before deletion")
		} else if edge != nil {
			sourceMRN = edge.Source
			targetMRN = edge.Target
		}
	}

	if err := s.repo.DeleteDirectLineage(ctx, edgeID); err != nil {
		return err
	}

	if s.lineageObserver != nil && sourceMRN != "" && targetMRN != "" {
		s.lineageObserver.OnEdgeDeleted(ctx, sourceMRN, targetMRN)
	}

	return nil
}

func (s *service) EdgeExists(ctx context.Context, source, target string) (bool, error) {
	return s.repo.EdgeExists(ctx, source, target)
}

func (s *service) GetImmediateNeighbors(ctx context.Context, assetMRN string, direction string) ([]string, error) {
	return s.repo.GetImmediateNeighbors(ctx, assetMRN, direction)
}

// SetLineageChangeObserver registers an observer for lineage mutations.
// Must be called during initialization before any lineage operations begin.
func (s *service) SetLineageChangeObserver(observer LineageChangeObserver) {
	s.lineageObserver = observer
}

func (s *service) StoreRunHistory(ctx context.Context, entry *RunHistoryEntry) error {
	return s.repo.StoreRunHistory(ctx, entry)
}
