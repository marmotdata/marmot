package lineage

import (
	"context"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/core/asset"
)

type Service interface {
	GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error)
	CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string) (string, error)
	EdgeExists(ctx context.Context, source, target string) (bool, error)
	DeleteDirectLineage(ctx context.Context, edgeID string) error
	GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error)
	ProcessOpenLineageEvent(ctx context.Context, event *RunEvent, createdBy string) error
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
	repo      Repository
	validator *validator.Validate
	metrics   MetricsClient
	assetSvc  asset.Service
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

func (s *service) CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string) (string, error) {
	return s.repo.CreateDirectLineage(ctx, sourceMRN, targetMRN)
}

func (s *service) DeleteDirectLineage(ctx context.Context, edgeID string) error {
	return s.repo.DeleteDirectLineage(ctx, edgeID)
}

func (s *service) EdgeExists(ctx context.Context, source, target string) (bool, error) {
	return s.repo.EdgeExists(ctx, source, target)
}
