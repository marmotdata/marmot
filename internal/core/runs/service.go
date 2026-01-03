package runs

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
	StatusDeleted   = "deleted"
	StatusFailed    = "failed"
)

var (
	ErrRunNotFound   = errors.New("run not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInvalidStatus = errors.New("invalid status transition")
)

type CreateAssetInput struct {
	Name          string                 `json:"name"`
	MRN           *string                `json:"mrn,omitempty"`
	Type          string                 `json:"type"`
	Providers     []string               `json:"providers"`
	Description   *string                `json:"description"`
	Metadata      map[string]interface{} `json:"metadata"`
	Schema        map[string]interface{} `json:"schema"`
	Tags          []string               `json:"tags"`
	Sources       []string               `json:"sources"`
	ExternalLinks []map[string]string    `json:"external_links"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
}

type ProcessAssetsResponse struct {
	Assets               []AssetResult         `json:"assets"`
	Lineage              []LineageResult       `json:"lineage"`
	Documentation        []DocumentationResult `json:"documentation"`
	StaleEntitiesRemoved []string              `json:"stale_entities_removed,omitempty"`
}

type AssetResult struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Provider string      `json:"provider"`
	MRN      string      `json:"mrn"`
	Asset    interface{} `json:"asset"`
	Status   string      `json:"status"`
	Error    string      `json:"error,omitempty"`
}

type LineageResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DocumentationResult struct {
	AssetMRN string `json:"asset_mrn"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

type LineageInput struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type DocumentationInput struct {
	AssetMRN string `json:"asset_mrn"`
	Content  string `json:"content"`
	Type     string `json:"type"`
}

type StatisticInput struct {
	AssetMRN   string  `json:"asset_mrn"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

type RunHistoryInput struct {
	AssetMRN     string                 `json:"asset_mrn"`
	RunID        string                 `json:"run_id"`
	JobNamespace string                 `json:"job_namespace"`
	JobName      string                 `json:"job_name"`
	EventType    string                 `json:"event_type"`
	EventTime    time.Time              `json:"event_time"`
	RunFacets    map[string]interface{} `json:"run_facets,omitempty"`
	JobFacets    map[string]interface{} `json:"job_facets,omitempty"`
}

type DestroyRunResponse struct {
	AssetsDeleted        int      `json:"assets_deleted"`
	LineageDeleted       int      `json:"lineage_deleted"`
	DocumentationDeleted int      `json:"documentation_deleted"`
	DeletedEntityMRNs    []string `json:"deleted_entity_mrns"`
}

type Service interface {
	StartRun(ctx context.Context, pipelineName, sourceName, createdBy string, config plugin.RawPluginConfig) (*plugin.Run, error)
	CompleteRun(ctx context.Context, runID string, status plugin.RunStatus, summary *plugin.RunSummary, errorMessage string) error
	ProcessAssets(ctx context.Context, runID string, assets []CreateAssetInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error)
	ProcessEntities(ctx context.Context, runID string, assets []CreateAssetInput, lineage []LineageInput, docs []DocumentationInput, stats []StatisticInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error)
	ProcessRunHistory(ctx context.Context, runHistory []RunHistoryInput) (int, error)
	AddCheckpoint(ctx context.Context, runID, entityType, entityMRN, operation string, sourceFields []string) error
	GetLastRunCheckpoints(ctx context.Context, pipelineName, sourceName string) (map[string]*plugin.RunCheckpoint, error)
	GetStaleEntities(ctx context.Context, lastCheckpoints map[string]*plugin.RunCheckpoint, currentEntityMRNs []string) []string
	DestroyPipeline(ctx context.Context, pipelineName string) (*DestroyRunResponse, error)
	CleanupStaleRuns(ctx context.Context, timeout time.Duration) (int, error)
	ListRuns(ctx context.Context, pipelineName string, limit, offset int) ([]*plugin.Run, int, error)
	ListRunsWithFilters(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]*plugin.Run, int, []string, error)
	GetRun(ctx context.Context, id string) (*plugin.Run, error)
	ListRunEntities(ctx context.Context, runID, entityType, status string, limit, offset int) ([]*RunEntity, int, error)
}

type service struct {
	repo            Repository
	assetService    asset.Service
	lineageService  lineage.Service
	metricsRecorder metrics.Recorder
	validator       *validator.Validate
}

func NewService(repo Repository, assetService asset.Service, lineageService lineage.Service, metricsRecorder metrics.Recorder) Service {
	return &service{
		repo:            repo,
		assetService:    assetService,
		lineageService:  lineageService,
		metricsRecorder: metricsRecorder,
		validator:       validator.New(),
	}
}

func (s *service) ListRunsWithFilters(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]*plugin.Run, int, []string, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	if offset < 0 {
		offset = 0
	}

	return s.repo.ListWithFilters(ctx, pipelines, statuses, limit, offset)
}

func (s *service) StartRun(ctx context.Context, pipelineName, sourceName, createdBy string, config plugin.RawPluginConfig) (*plugin.Run, error) {
	if pipelineName == "" || sourceName == "" || createdBy == "" {
		return nil, fmt.Errorf("%w: pipeline_name, source_name, and created_by are required", ErrInvalidInput)
	}

	runID := uuid.New().String()
	now := time.Now()

	run := &plugin.Run{
		ID:           uuid.New().String(),
		PipelineName: pipelineName,
		SourceName:   sourceName,
		RunID:        runID,
		Status:       plugin.StatusRunning,
		StartedAt:    now,
		Config:       config,
		CreatedBy:    createdBy,
	}

	if err := s.repo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("creating run: %w", err)
	}

	return run, nil
}

func (s *service) CompleteRun(ctx context.Context, runID string, status plugin.RunStatus, summary *plugin.RunSummary, errorMessage string) error {
	if runID == "" {
		return fmt.Errorf("%w: run_id is required", ErrInvalidInput)
	}

	if status != plugin.StatusCompleted && status != plugin.StatusFailed && status != plugin.StatusCancelled {
		return fmt.Errorf("%w: invalid completion status %s", ErrInvalidStatus, status)
	}

	run, err := s.repo.GetByRunID(ctx, runID)
	if err != nil {
		return fmt.Errorf("getting run: %w", err)
	}

	if run.Status != plugin.StatusRunning {
		return fmt.Errorf("%w: cannot complete run with status %s", ErrInvalidStatus, run.Status)
	}

	now := time.Now()
	run.Status = status
	run.CompletedAt = &now
	run.Summary = summary
	run.ErrorMessage = errorMessage

	if err := s.repo.Update(ctx, run); err != nil {
		return fmt.Errorf("updating run: %w", err)
	}

	return nil
}

func (s *service) ProcessEntities(ctx context.Context, runID string, assets []CreateAssetInput, lineage []LineageInput, docs []DocumentationInput, stats []StatisticInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error) {
	run, err := s.repo.GetByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("getting run: %w", err)
	}

	lastCheckpoints, _ := s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName)

	response := &ProcessAssetsResponse{
		Assets:        make([]AssetResult, 0, len(assets)),
		Lineage:       make([]LineageResult, 0, len(lineage)),
		Documentation: make([]DocumentationResult, 0, len(docs)),
	}

	currentMRNs := make([]string, 0, len(assets))
	for _, ast := range assets {
		var assetMRN string
		if ast.MRN != nil && *ast.MRN != "" {
			assetMRN = *ast.MRN
		} else {
			assetMRN = mrn.New(ast.Type, ast.Providers[0], ast.Name)
		}
		currentMRNs = append(currentMRNs, assetMRN)

		assetHash := s.hashAsset(ast)

		status := StatusCreated
		if checkpoint, exists := lastCheckpoints[assetMRN]; exists && checkpoint.Operation != StatusDeleted {
			if len(checkpoint.SourceFields) > 0 && checkpoint.SourceFields[0] == assetHash {
				status = StatusUnchanged
			} else {
				status = StatusUpdated
			}
		}

		if status == StatusCreated {
			createInput := asset.CreateInput{
				Name:          &ast.Name,
				MRN:           &assetMRN,
				Type:          ast.Type,
				Providers:     ast.Providers,
				Description:   ast.Description,
				Metadata:      ast.Metadata,
				Schema:        convertSchemaToStringMap(ast.Schema),
				Tags:          ast.Tags,
				ExternalLinks: convertToAssetExternalLinks(ast.ExternalLinks),
				Query:         ast.Query,
				QueryLanguage: ast.QueryLanguage,
				CreatedBy:     run.CreatedBy,
			}
			if _, err := s.assetService.Create(ctx, createInput); err != nil {
				log.Error().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to create asset")
				status = StatusFailed
			}
		} else if status == StatusUpdated {
			updateInput := asset.UpdateInput{
				Name:          &ast.Name,
				Type:          ast.Type,
				Providers:     ast.Providers,
				Description:   ast.Description,
				Metadata:      ast.Metadata,
				Schema:        convertSchemaToStringMap(ast.Schema),
				Tags:          ast.Tags,
				ExternalLinks: convertToAssetExternalLinks(ast.ExternalLinks),
				Query:         ast.Query,
				QueryLanguage: ast.QueryLanguage,
			}
			existingAsset, err := s.assetService.GetByMRN(ctx, assetMRN)
			if err != nil {
				log.Error().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to get existing asset for update")
				status = StatusFailed
			} else {
				if _, err := s.assetService.Update(ctx, existingAsset.ID, updateInput); err != nil {
					log.Error().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to update asset")
					status = StatusFailed
				}
			}
		}

		result := AssetResult{
			Name:     ast.Name,
			Type:     ast.Type,
			Provider: ast.Providers[0],
			MRN:      assetMRN,
			Status:   status,
			Asset:    ast,
		}
		response.Assets = append(response.Assets, result)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "asset",
			EntityMRN:  assetMRN,
			EntityName: ast.Name,
			Status:     result.Status,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", assetMRN).Msg("Failed to add run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "asset", assetMRN, result.Status, []string{assetHash}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", assetMRN).Msg("Failed to add checkpoint")
		}
	}

	staleEntities := s.GetStaleEntities(ctx, lastCheckpoints, currentMRNs)
	for _, staleMRN := range staleEntities {
		if err := s.assetService.DeleteByMRN(ctx, staleMRN); err != nil {
			if errors.Is(err, asset.ErrAssetNotFound) {
				log.Debug().Str("asset_mrn", staleMRN).Msg("Stale asset already deleted")
			} else {
				log.Error().Err(err).Str("asset_mrn", staleMRN).Msg("Failed to delete stale asset")
			}
		}

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "asset",
			EntityMRN:  staleMRN,
			Status:     StatusDeleted,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", staleMRN).Msg("Failed to add deleted run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "asset", staleMRN, StatusDeleted, []string{}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", staleMRN).Msg("Failed to add deleted checkpoint")
		}
	}
	response.StaleEntitiesRemoved = staleEntities

	for _, lin := range lineage {
		lineageMRN := mrn.New("lineage", strings.ToLower(lin.Type), fmt.Sprintf("%s->%s", lin.Source, lin.Target))

		status := StatusCreated
		if checkpoint, exists := lastCheckpoints[lineageMRN]; exists && checkpoint.Operation != StatusDeleted {
			status = StatusUpdated
		}

		if status == StatusCreated {
			if _, err := s.lineageService.CreateDirectLineage(ctx, lin.Source, lin.Target, lin.Type); err != nil {
				log.Error().Err(err).Str("source", lin.Source).Str("target", lin.Target).Str("type", lin.Type).Msg("Failed to create lineage")
				status = StatusFailed
			}
		}

		result := LineageResult{
			Source: lin.Source,
			Target: lin.Target,
			Type:   lin.Type,
			Status: status,
		}
		response.Lineage = append(response.Lineage, result)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "lineage",
			EntityMRN:  lineageMRN,
			EntityName: fmt.Sprintf("%s -> %s", lin.Source, lin.Target),
			Status:     result.Status,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", lineageMRN).Msg("Failed to add lineage run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "lineage", lineageMRN, result.Status, []string{"source", "target", "type"}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", lineageMRN).Msg("Failed to add lineage checkpoint")
		}
	}

	for _, doc := range docs {
		docMRN := mrn.New("documentation", strings.ToLower(doc.Type), doc.AssetMRN)

		status := StatusCreated
		if checkpoint, exists := lastCheckpoints[docMRN]; exists && checkpoint.Operation != StatusDeleted {
			status = StatusUpdated
		}

		result := DocumentationResult{
			AssetMRN: doc.AssetMRN,
			Type:     doc.Type,
			Status:   status,
		}
		response.Documentation = append(response.Documentation, result)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "documentation",
			EntityMRN:  docMRN,
			EntityName: fmt.Sprintf("%s (%s)", doc.AssetMRN, doc.Type),
			Status:     result.Status,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", docMRN).Msg("Failed to add documentation run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "documentation", docMRN, result.Status, []string{"asset_mrn", "type"}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", docMRN).Msg("Failed to add documentation checkpoint")
		}
	}

	if len(stats) > 0 {
		s.processStatistics(ctx, stats)
	}

	return response, nil
}

func (s *service) processStatistics(ctx context.Context, statistics []StatisticInput) {
	if len(statistics) == 0 {
		return
	}

	metricsToRecord := make([]metrics.Metric, 0, len(statistics))
	now := time.Now()

	for _, stat := range statistics {
		metricsToRecord = append(metricsToRecord, metrics.Metric{
			Name:  stat.MetricName,
			Type:  metrics.Gauge,
			Value: stat.Value,
			Labels: map[string]string{
				"asset_mrn": stat.AssetMRN,
			},
			Timestamp: now,
		})
	}

	if err := s.metricsRecorder.RecordCustomMetrics(ctx, metricsToRecord); err != nil {
		log.Warn().Err(err).Msg("Failed to record asset statistics")
	}
}

func convertSchemaToStringMap(schema map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range schema {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func convertToAssetExternalLinks(links []map[string]string) []asset.ExternalLink {
	result := make([]asset.ExternalLink, 0, len(links))
	for _, link := range links {
		result = append(result, asset.ExternalLink{
			Name: link["name"],
			URL:  link["url"],
		})
	}
	return result
}

func (s *service) ProcessAssets(ctx context.Context, runID string, assets []CreateAssetInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error) {
	return s.ProcessEntities(ctx, runID, assets, nil, nil, nil, pipelineName, sourceName)
}

func (s *service) AddCheckpoint(ctx context.Context, runID, entityType, entityMRN, operation string, sourceFields []string) error {
	if runID == "" || entityType == "" || entityMRN == "" || operation == "" {
		return fmt.Errorf("%w: runID, entityType, entityMRN, and operation are required", ErrInvalidInput)
	}

	run, err := s.repo.GetByRunID(ctx, runID)
	if err != nil {
		return fmt.Errorf("getting run: %w", err)
	}

	checkpoint := &plugin.RunCheckpoint{
		ID:           uuid.New().String(),
		RunID:        runID,
		EntityType:   entityType,
		EntityMRN:    entityMRN,
		Operation:    operation,
		SourceFields: sourceFields,
		CreatedAt:    time.Now(),
	}

	return s.repo.AddCheckpoint(ctx, run.ID, checkpoint)
}

func (s *service) GetLastRunCheckpoints(ctx context.Context, pipelineName, sourceName string) (map[string]*plugin.RunCheckpoint, error) {
	if pipelineName == "" || sourceName == "" {
		return nil, fmt.Errorf("%w: pipeline_name and source_name are required", ErrInvalidInput)
	}

	return s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName)
}

func (s *service) DestroyPipeline(ctx context.Context, pipelineName string) (*DestroyRunResponse, error) {
	if pipelineName == "" {
		return nil, fmt.Errorf("%w: pipeline_name is required", ErrInvalidInput)
	}

	allRuns, _, err := s.repo.List(ctx, pipelineName, 10000, 0)
	if err != nil {
		return nil, fmt.Errorf("listing runs for pipeline: %w", err)
	}

	sourceNames := make(map[string]bool)
	for _, run := range allRuns {
		sourceNames[run.SourceName] = true
	}

	allCurrentEntities := make(map[string]*plugin.RunCheckpoint)
	for sourceName := range sourceNames {
		checkpoints, err := s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName)
		if err != nil {
			log.Warn().Err(err).Str("pipeline", pipelineName).Str("source", sourceName).Msg("Failed to get checkpoints for source, skipping")
			continue
		}

		for mrn, checkpoint := range checkpoints {
			if checkpoint.Operation != StatusDeleted {
				allCurrentEntities[mrn] = checkpoint
			}
		}
	}

	response := &DestroyRunResponse{
		AssetsDeleted:        0,
		LineageDeleted:       0,
		DocumentationDeleted: 0,
		DeletedEntityMRNs:    make([]string, 0, len(allCurrentEntities)),
	}

	if len(allCurrentEntities) == 0 {
		log.Info().Str("pipeline", pipelineName).Msg("No entities found to delete for pipeline")
		return response, nil
	}

	log.Info().Str("pipeline", pipelineName).Int("entity_count", len(allCurrentEntities)).Msg("Starting pipeline destruction")

	destroyRunID := uuid.New().String()
	now := time.Now()

	destroyRun := &plugin.Run{
		ID:           uuid.New().String(),
		PipelineName: pipelineName,
		SourceName:   "destroy",
		RunID:        destroyRunID,
		Status:       plugin.StatusRunning,
		StartedAt:    now,
		Config:       plugin.RawPluginConfig{"operation": "destroy_pipeline"},
		CreatedBy:    "system",
	}

	if err := s.repo.Create(ctx, destroyRun); err != nil {
		return nil, fmt.Errorf("creating destroy run: %w", err)
	}

	for entityMRN, checkpoint := range allCurrentEntities {
		switch checkpoint.EntityType {
		case "asset":
			if err := s.assetService.DeleteByMRN(ctx, entityMRN); err != nil {
				log.Error().Err(err).Str("entity_mrn", entityMRN).Msg("Failed to delete asset")
				entity := &RunEntity{
					ID:           uuid.New().String(),
					RunID:        destroyRunID,
					EntityType:   checkpoint.EntityType,
					EntityMRN:    entityMRN,
					Status:       StatusFailed,
					ErrorMessage: fmt.Sprintf("Failed to delete asset: %v", err),
					CreatedAt:    time.Now(),
				}
				if err := s.repo.AddRunEntity(ctx, destroyRun.ID, entity); err != nil {
					log.Error().Err(err).Str("destroy_run_id", destroyRunID).Str("entity_mrn", entityMRN).Msg("Failed to track failed deletion")
				}
				continue
			}
			response.AssetsDeleted++
		case "lineage":
			parsedMRN, err := mrn.Parse(entityMRN)
			if err != nil {
				log.Error().Err(err).Str("entity_mrn", entityMRN).Msg("Failed to parse lineage MRN")
				continue
			}

			parts := strings.Split(parsedMRN.Name, "->")
			if len(parts) != 2 {
				log.Error().Str("entity_mrn", entityMRN).Str("parsed_name", parsedMRN.Name).Msg("Invalid lineage MRN format")
				continue
			}

			sourceAsset, err := s.assetService.GetByMRN(ctx, parts[0])
			if err != nil {
				log.Error().Err(err).Str("source_mrn", parts[0]).Msg("Failed to get source asset for lineage deletion")
				continue
			}

			lineageResp, err := s.lineageService.GetAssetLineage(ctx, sourceAsset.ID, 1000, "downstream")
			if err != nil {
				log.Error().Err(err).Str("source_asset_id", sourceAsset.ID).Msg("Failed to get lineage for deletion")
				continue
			}

			var edgeToDelete *lineage.LineageEdge
			for _, edge := range lineageResp.Edges {
				if edge.Source == parts[0] && edge.Target == parts[1] {
					edgeToDelete = &edge
					break
				}
			}

			if edgeToDelete != nil {
				if err := s.lineageService.DeleteDirectLineage(ctx, edgeToDelete.ID); err != nil {
					log.Error().Err(err).Str("edge_id", edgeToDelete.ID).Msg("Failed to delete lineage edge")
					continue
				}
			}
			response.LineageDeleted++
		case "documentation":
			response.DocumentationDeleted++
		}

		response.DeletedEntityMRNs = append(response.DeletedEntityMRNs, entityMRN)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      destroyRunID,
			EntityType: checkpoint.EntityType,
			EntityMRN:  entityMRN,
			Status:     StatusDeleted,
			CreatedAt:  time.Now(),
		}

		if err := s.repo.AddRunEntity(ctx, destroyRun.ID, entity); err != nil {
			log.Error().Err(err).Str("destroy_run_id", destroyRunID).Str("entity_mrn", entityMRN).Msg("Failed to track entity deletion")
		}
	}

	for sourceName := range sourceNames {
		if err := s.repo.DeleteCheckpoints(ctx, pipelineName, sourceName); err != nil {
			log.Error().Err(err).Str("pipeline", pipelineName).Str("source", sourceName).Msg("Failed to delete checkpoints")
		}
	}

	completedAt := time.Now()
	destroyRun.Status = plugin.StatusCompleted
	destroyRun.CompletedAt = &completedAt
	destroyRun.Summary = &plugin.RunSummary{
		AssetsDeleted:      response.AssetsDeleted,
		LineageUpdated:     response.LineageDeleted,
		DocumentationAdded: response.DocumentationDeleted,
		TotalEntities:      len(response.DeletedEntityMRNs),
		DurationSeconds:    int(completedAt.Sub(now).Seconds()),
	}

	if err := s.repo.Update(ctx, destroyRun); err != nil {
		log.Error().Err(err).Str("destroy_run_id", destroyRunID).Msg("Failed to complete destroy run")
	}

	log.Info().
		Str("pipeline", pipelineName).
		Int("assets_deleted", response.AssetsDeleted).
		Int("lineage_deleted", response.LineageDeleted).
		Int("documentation_deleted", response.DocumentationDeleted).
		Int("total_deleted", len(response.DeletedEntityMRNs)).
		Dur("duration", completedAt.Sub(now)).
		Msg("Pipeline destruction completed")

	return response, nil
}

func (s *service) GetStaleEntities(ctx context.Context, lastCheckpoints map[string]*plugin.RunCheckpoint, currentEntityMRNs []string) []string {
	currentSet := make(map[string]bool)
	for _, mrn := range currentEntityMRNs {
		currentSet[mrn] = true
	}

	var staleEntities []string
	for mrn, checkpoint := range lastCheckpoints {
		if checkpoint.Operation != StatusDeleted && !currentSet[mrn] {
			staleEntities = append(staleEntities, mrn)
		}
	}

	return staleEntities
}

func (s *service) CleanupStaleRuns(ctx context.Context, timeout time.Duration) (int, error) {
	return s.repo.CleanupStaleRuns(ctx, timeout)
}

func (s *service) ListRuns(ctx context.Context, pipelineName string, limit, offset int) ([]*plugin.Run, int, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	if offset < 0 {
		offset = 0
	}

	return s.repo.List(ctx, pipelineName, limit, offset)
}

func (s *service) GetRun(ctx context.Context, id string) (*plugin.Run, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidInput)
	}

	run, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return nil, ErrRunNotFound
		}
		return nil, fmt.Errorf("getting run: %w", err)
	}

	return run, nil
}

func (s *service) ListRunEntities(ctx context.Context, runID, entityType, status string, limit, offset int) ([]*RunEntity, int, error) {
	if runID == "" {
		return nil, 0, fmt.Errorf("%w: run_id is required", ErrInvalidInput)
	}

	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			return nil, 0, ErrRunNotFound
		}
		return nil, 0, fmt.Errorf("getting run: %w", err)
	}

	if limit <= 0 {
		limit = 100
	} else if limit > 1000 {
		limit = 1000
	}

	if offset < 0 {
		offset = 0
	}

	return s.repo.ListRunEntities(ctx, run.ID, entityType, status, limit, offset)
}

func (s *service) hashAsset(asset CreateAssetInput) string {
	normalized := struct {
		Name          string                 `json:"name"`
		Type          string                 `json:"type"`
		Providers     []string               `json:"providers"`
		Description   *string                `json:"description"`
		Metadata      map[string]interface{} `json:"metadata"`
		Schema        map[string]interface{} `json:"schema"`
		Tags          []string               `json:"tags"`
		Sources       []string               `json:"sources"`
		ExternalLinks []map[string]string    `json:"external_links"`
	}{
		Name:          asset.Name,
		Type:          asset.Type,
		Providers:     asset.Providers,
		Description:   asset.Description,
		Metadata:      asset.Metadata,
		Schema:        asset.Schema,
		Tags:          asset.Tags,
		Sources:       asset.Sources,
		ExternalLinks: asset.ExternalLinks,
	}

	data, _ := json.Marshal(normalized)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func (s *service) ProcessRunHistory(ctx context.Context, runHistory []RunHistoryInput) (int, error) {
	stored := 0
	for _, rh := range runHistory {
		// Get the asset by MRN to get its ID
		existingAsset, err := s.assetService.GetByMRN(ctx, rh.AssetMRN)
		if err != nil {
			log.Warn().Err(err).Str("asset_mrn", rh.AssetMRN).Msg("Failed to get asset for run history, skipping")
			continue
		}

		entry := &lineage.RunHistoryEntry{
			ID:           uuid.New().String(),
			AssetID:      existingAsset.ID,
			RunID:        rh.RunID,
			JobNamespace: rh.JobNamespace,
			JobName:      rh.JobName,
			EventType:    rh.EventType,
			EventTime:    rh.EventTime,
			Producer:     "marmot-plugin",
			RunFacets:    rh.RunFacets,
			JobFacets:    rh.JobFacets,
			CreatedAt:    time.Now(),
		}

		if err := s.lineageService.StoreRunHistory(ctx, entry); err != nil {
			log.Error().Err(err).Str("asset_mrn", rh.AssetMRN).Str("run_id", rh.RunID).Msg("Failed to store run history")
			continue
		}
		stored++
	}

	return stored, nil
}

