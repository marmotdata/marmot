package lineage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type Service interface {
	HandleLineageEvent(ctx context.Context, event interface{}) error
	GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error)
	CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string) (string, error)
	EdgeExists(ctx context.Context, source, target string) (bool, error)
	DeleteDirectLineage(ctx context.Context, edgeID string) error
	GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error)
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

func (s *service) HandleLineageEvent(ctx context.Context, event interface{}) error {
	var typedEvent RunEvent
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	if err := json.Unmarshal(eventJSON, &typedEvent); err != nil {
		return fmt.Errorf("unmarshaling event: %w", err)
	}

	// First detect job type
	//TODO: fix so we're just passing job into the func
	jobDetector := NewJobDetector(typedEvent.Job.Facets, &typedEvent.Job)
	pipeline, err := jobDetector.DetectJob()
	if err != nil {
		return fmt.Errorf("failed to detect job type: %w", err)
	}

	// Create job asset
	jobMRN := fmt.Sprintf("mrn://%s/job/%s/%s",
		typedEvent.Job.Namespace,
		strings.ToLower(pipeline.Service),
		typedEvent.Job.Name)

	log.Info().
		Str("mrn", jobMRN).
		Str("type", "JOB").
		Str("service", pipeline.Service).
		Msg("Ensuring job asset exists")

	if err := s.ensureJobAsset(ctx, jobMRN, typedEvent.Job.Name, pipeline.Service); err != nil {
		return fmt.Errorf("ensuring job asset: %w", err)
	}

	// Process inputs
	for _, input := range typedEvent.Inputs {
		if err := s.processDataset(ctx, input.Namespace, input.Name, input.Facets); err != nil {
			return fmt.Errorf("ensuring input asset: %w", err)
		}
	}

	// Process outputs
	for _, output := range typedEvent.Outputs {
		if err := s.processDataset(ctx, output.Namespace, output.Name, output.Facets); err != nil {
			return fmt.Errorf("ensuring output asset: %w", err)
		}
	}

	return nil
}

func (s *service) processDataset(ctx context.Context, namespace, name string, facets map[string]json.RawMessage) error {
	// Extract type from facets
	var assetType, service string
	if typeFacet, ok := facets["type"]; ok {
		// Remove quotes from the type string
		typeStr := strings.Trim(string(typeFacet), "\"")
		assetType = typeStr

		// Determine service based on namespace and type
		switch strings.ToLower(namespace) {
		case "kafka":
			service = "Kafka"
		case "aws":
			service = "AWS"
		default:
			service = strings.ToTitle(namespace)
		}
	} else {
		return fmt.Errorf("missing type facet for dataset")
	}

	assetMRN := fmt.Sprintf("mrn://%s/%s/%s",
		strings.ToLower(namespace),
		strings.ToLower(assetType),
		name)

	description := fmt.Sprintf("%s %s in %s", service, assetType, namespace)

	log.Info().
		Str("mrn", assetMRN).
		Str("type", assetType).
		Str("service", service).
		Msg("Ensuring dataset asset exists")

	input := asset.CreateInput{
		Name:        &name,
		Type:        assetType,
		Providers:   []string{service},
		MRN:         &assetMRN,
		Description: &description,
		CreatedBy:   "system",
		Tags:        []string{strings.ToLower(service)},
		Metadata: map[string]interface{}{
			"namespace": namespace,
		},
	}

	_, err := s.assetSvc.Create(ctx, input)
	if err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
		return fmt.Errorf("creating asset: %w", err)
	}

	return nil
}

func (s *service) ensureJobAsset(ctx context.Context, mrn, name, service string) error {
	description := fmt.Sprintf("%s job from OpenLineage", service)
	input := asset.CreateInput{
		Name:        &name,
		Type:        "JOB",
		Providers:   []string{service},
		MRN:         &mrn,
		Description: &description,
		CreatedBy:   "system",
		Tags:        []string{strings.ToLower(service)},
	}

	_, err := s.assetSvc.Create(ctx, input)
	if err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
		return fmt.Errorf("creating asset: %w", err)
	}

	return nil
}

func (s *service) ensureAsset(ctx context.Context, input *asset.CreateInput) error {
	log.Info().
		Str("mrn", *input.MRN).
		Str("type", input.Type).
		Str("service", strings.Join(input.Providers[:], ",")).
		Msg("Ensuring asset exists")

	_, err := s.assetSvc.GetByMRN(ctx, *input.MRN)
	if err == nil {
		return nil
	}
	if !errors.Is(err, asset.ErrAssetNotFound) {
		return err
	}

	created, err := s.assetSvc.Create(ctx, *input)
	if err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
		return err
	}

	if created != nil {
		log.Info().
			Str("mrn", *input.MRN).
			Str("type", input.Type).
			Str("service", strings.Join(input.Providers[:], ",")).
			Msg("Asset created")
	}

	return nil
}

func (s *service) ensureDataset(ctx context.Context, namespace, name string, facets map[string]json.RawMessage) error {
	assetMRN := fmt.Sprintf("%s/%s", namespace, name)
	_, err := s.assetSvc.GetByMRN(ctx, assetMRN)
	if err == nil {
		return nil // Asset exists
	}
	if !errors.Is(err, asset.ErrAssetNotFound) {
		return err
	}

	assetInput, err := detectDatasetAsset(namespace, name, facets)
	if err != nil {
		return fmt.Errorf("asset not found and cannot be detected: %w", err)
	}

	_, err = s.assetSvc.Create(ctx, *assetInput)
	if err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
		return err
	}
	return nil
}
