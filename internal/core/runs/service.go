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
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/metrics"
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
	Sources       []asset.AssetSource    `json:"sources"`
	ExternalLinks []map[string]string    `json:"external_links"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
	// Terms are the names of glossary terms this asset was assigned.
	// Empty means the source has nothing to say about terms, which is
	// not the same as saying the asset has none.
	Terms []string `json:"terms,omitempty"`
	// PreviousMRNs are MRNs earlier versions of the plugin gave this asset; an existing asset under one of them moves to the new MRN instead of being replaced.
	PreviousMRNs []string `json:"previous_mrns,omitempty"`
}

// GlossaryTermInput is one business term a run discovered. Identity is
// Name, so Parent names another term in the same batch or one an earlier
// run already stored.
type GlossaryTermInput struct {
	Name        string                 `json:"name"`
	Definition  string                 `json:"definition"`
	Description string                 `json:"description,omitempty"`
	Parent      string                 `json:"parent,omitempty"`
	Synonyms    []string               `json:"synonyms,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GlossaryResult reports what a run did to the glossary.
type GlossaryResult struct {
	TermsCreated   int `json:"terms_created"`
	TermsUpdated   int `json:"terms_updated"`
	TermsUnchanged int `json:"terms_unchanged"`
	AssetsLinked   int `json:"assets_linked"`
}

type ProcessAssetsResponse struct {
	Assets               []AssetResult         `json:"assets"`
	Lineage              []LineageResult       `json:"lineage"`
	Documentation        []DocumentationResult `json:"documentation"`
	StaleEntitiesRemoved []string              `json:"stale_entities_removed,omitempty"`
	// StaleContributionsWithdrawn are assets this run no longer found but another writer still describes, so they were kept.
	StaleContributionsWithdrawn []string        `json:"stale_contributions_withdrawn,omitempty"`
	Glossary                    *GlossaryResult `json:"glossary,omitempty"`
	// Warnings are identity problems the run worked around, such as an old asset several new ones could be.
	Warnings []string `json:"warnings,omitempty"`
}

type AssetResult struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Provider string      `json:"provider"`
	MRN      string      `json:"mrn"`
	Asset    interface{} `json:"asset"`
	Status   string      `json:"status"`
	Error    string      `json:"error,omitempty"`
	// MovedFrom is the MRN the asset had before this run gave it a new one.
	MovedFrom string `json:"moved_from,omitempty"`
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
	Source        string               `json:"source"`
	Target        string               `json:"target"`
	Type          string               `json:"type"`
	JobMRN        string               `json:"job_mrn,omitempty"`
	ColumnLineage []lineage.ColumnEdge `json:"column_lineage,omitempty"`
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
	// StartRunAs starts a run that belongs to owner: "schedule:<id>" for a scheduled run, "<principal type>:<id>" for a CLI or API run.
	StartRunAs(ctx context.Context, pipelineName, sourceName, createdBy, owner string, config plugin.RawPluginConfig) (*plugin.Run, error)
	RecordZoneBindings(ctx context.Context, bindings []asset.ZoneBinding) error
	CompleteRun(ctx context.Context, runID string, status plugin.RunStatus, summary *plugin.RunSummary, errorMessage string) error
	ProcessAssets(ctx context.Context, runID string, assets []CreateAssetInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error)
	ProcessEntities(ctx context.Context, runID string, assets []CreateAssetInput, lineage []LineageInput, docs []DocumentationInput, stats []StatisticInput, terms []GlossaryTermInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error)
	ProcessRunHistory(ctx context.Context, runHistory []RunHistoryInput) (int, error)
	AddCheckpoint(ctx context.Context, runID, entityType, entityMRN, operation string, sourceFields []string) error
	GetLastRunCheckpoints(ctx context.Context, pipelineName, sourceName string) (map[string]*plugin.RunCheckpoint, error)
	GetStaleEntities(ctx context.Context, lastCheckpoints map[string]*plugin.RunCheckpoint, currentEntityMRNs []string) []string
	DestroyPipeline(ctx context.Context, pipelineName string) (*DestroyRunResponse, error)
	CleanupStaleRuns(ctx context.Context, timeout time.Duration) (int, error)
	ListRuns(ctx context.Context, pipelineName string, limit, offset int) ([]*plugin.Run, int, error)
	ListRunsWithFilters(ctx context.Context, pipelines, statuses []string, limit, offset int) ([]*plugin.Run, int, []string, error)
	GetRun(ctx context.Context, id string) (*plugin.Run, error)
	GetByRunID(ctx context.Context, runID string) (*plugin.Run, error)
	ListRunEntities(ctx context.Context, runID, entityType, status string, limit, offset int) ([]*RunEntity, int, error)
	SetCompletionObserver(observer RunCompletionObserver)
}

// RunCompletionObserver is notified when runs complete.
type RunCompletionObserver interface {
	OnRunCompleted(ctx context.Context, run *plugin.Run)
}

// GlossarySyncer upserts the terms a run discovered and reports where
// they landed. It is the glossary service narrowed to what a run needs.
type GlossarySyncer interface {
	SyncTerms(ctx context.Context, source string, inputs []glossary.TermInput) (*glossary.SyncResult, error)
}

type service struct {
	repo               Repository
	assetService       asset.Service
	lineageService     lineage.Service
	glossaryService    GlossarySyncer
	metricsRecorder    metrics.Recorder
	validator          *validator.Validate
	completionObserver RunCompletionObserver
}

func NewService(repo Repository, assetService asset.Service, lineageService lineage.Service, glossaryService GlossarySyncer, metricsRecorder metrics.Recorder) Service {
	return &service{
		repo:            repo,
		assetService:    assetService,
		lineageService:  lineageService,
		glossaryService: glossaryService,
		metricsRecorder: metricsRecorder,
		validator:       validator.New(),
	}
}

func (s *service) SetCompletionObserver(observer RunCompletionObserver) {
	s.completionObserver = observer
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
	return s.StartRunAs(ctx, pipelineName, sourceName, createdBy, "", config)
}

func (s *service) StartRunAs(ctx context.Context, pipelineName, sourceName, createdBy, owner string, config plugin.RawPluginConfig) (*plugin.Run, error) {
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
		Owner:        owner,
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

	if s.completionObserver != nil {
		s.completionObserver.OnRunCompleted(ctx, run)
	}

	return nil
}

func (s *service) ProcessEntities(ctx context.Context, runID string, assets []CreateAssetInput, lineage []LineageInput, docs []DocumentationInput, stats []StatisticInput, terms []GlossaryTermInput, pipelineName, sourceName string) (*ProcessAssetsResponse, error) {
	run, err := s.repo.GetByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("getting run: %w", err)
	}

	// This error must not be dropped. An empty checkpoint map reads as
	// "this pipeline has never created anything", which switches off
	// stale-entity removal, so a transient database error that came back
	// as an empty map would quietly re-create the whole catalog alongside
	// the rows already in it.
	lastCheckpoints, baselineOwner, err := s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName, run.Owner)
	if err != nil {
		return nil, fmt.Errorf("getting last run checkpoints: %w", err)
	}
	writer := WriterKey(pipelineName, sourceName, run.Owner)
	role := writerRole(sourceName)
	// The first run of an owner takes over an earlier baseline: the same person's runs before they set a zone, or the runs from before runs had owners. That baseline may hold what someone else's run found, so this run adopts it without treating anything in it as stale.
	inheritedBaseline := baselineOwner != run.Owner && len(lastCheckpoints) > 0
	legacyWriter := WriterKey(pipelineName, sourceName, baselineOwner)

	response := &ProcessAssetsResponse{
		Assets:        make([]AssetResult, 0, len(assets)),
		Lineage:       make([]LineageResult, 0, len(lineage)),
		Documentation: make([]DocumentationResult, 0, len(docs)),
	}

	// The glossary is synced before the assets so that an asset can be
	// linked to its terms the moment it is written.
	termIDsByName := s.syncGlossary(ctx, sourceName, terms, response)

	identities, unresolved, err := s.resolveAssetIdentities(ctx, assets, response)
	if err != nil {
		return nil, err
	}

	// Checkpoints name assets by the MRN they had when written, which may be a legacy MRN or one the asset has since moved from, so they are compared by the asset's MRN today.
	assetCheckpoints, err := s.currentAssetCheckpoints(ctx, lastCheckpoints)
	if err != nil {
		return nil, err
	}

	currentMRNs := make([]string, 0, len(assets))
	for i, ast := range assets {
		id := identities[i]
		assetMRN := id.mrn
		assetHash := s.hashAsset(ast)
		assetError := id.err
		status := StatusCreated
		existingAsset := id.existing

		if assetError != "" {
			status = StatusFailed
		} else if checkpoint, exists := assetCheckpoints[assetMRN]; exists && checkpoint.Operation != StatusDeleted && existingAsset != nil {
			if len(checkpoint.SourceFields) > 0 && checkpoint.SourceFields[0] == assetHash {
				status = StatusUnchanged
			} else {
				status = StatusUpdated
			}
		} else if existingAsset != nil {
			status = StatusUpdated
		}

		// The asset's own id, needed to attach glossary terms. Each
		// branch below already knows it, so nothing is looked up twice.
		assetID := ""
		if existingAsset != nil {
			assetID = existingAsset.ID
		}
		fields := contributionFields(ast)

		switch status {
		case StatusUnchanged:
			// Nothing new to say, but a writer that has never contributed to this asset, such as an owner inheriting the baseline from before owners existed, still has to be recorded as one.
			touched, err := s.assetService.TouchContribution(ctx, assetID, writer)
			if err == nil && !touched {
				_, err = s.assetService.Contribute(ctx, assetID, writer, role, fields, true)
			}
			if err != nil {
				log.Warn().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to record the run's contribution")
			}
		case StatusCreated:
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
				Sources:       ast.Sources,
				CreatedBy:     run.CreatedBy,
				Writer:        writer,
				Role:          role,
			}
			created, err := s.assetService.Create(ctx, createInput)
			if created != nil {
				assetID = created.ID
			}

			// The asset can already exist even though this pipeline has
			// never seen it: another source discovered the same thing
			// first. That is how a catalog imported from elsewhere hands
			// over to the native plugin for the same technology, so the
			// second source adds its contribution rather than failing.
			var exists *asset.ExistsError
			if errors.As(err, &exists) && exists.Asset != nil {
				existingAsset = exists.Asset
				assetID = existingAsset.ID
				status = StatusUpdated
				err = nil
			}

			if err != nil {
				log.Error().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to create asset")
				assetError = err.Error()
				status = StatusFailed
			}
		}

		if status == StatusUpdated {
			if _, err := s.assetService.Contribute(ctx, assetID, writer, role, fields, true); err != nil {
				log.Error().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to update asset")
				assetError = err.Error()
				status = StatusFailed
			} else if len(ast.Sources) > 0 {
				if _, err := s.assetService.Update(ctx, assetID, asset.UpdateInput{Sources: ast.Sources, SkipNotification: true}); err != nil {
					log.Warn().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to record the asset's sources")
				}
			}
		}

		if status != StatusFailed && inheritedBaseline && assetID != "" && writer != legacyWriter {
			// This owner now speaks for the asset, so the contribution recorded by the baseline's owner is withdrawn. Another owner who also saw the asset keeps it alive with their own contribution.
			if _, err := s.assetService.Withdraw(ctx, assetID, legacyWriter); err != nil {
				log.Warn().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to hand over the pre-owner contribution")
			}
		}

		if status != StatusFailed && len(ast.Terms) > 0 && assetID != "" {
			if s.linkTerms(ctx, assetID, ast.Terms, termIDsByName, sourceName) && response.Glossary != nil {
				response.Glossary.AssetsLinked++
			}
		}

		result := AssetResult{
			Name:      ast.Name,
			Type:      ast.Type,
			Provider:  firstProvider(ast.Providers),
			MRN:       assetMRN,
			Status:    status,
			Asset:     ast,
			Error:     assetError,
			MovedFrom: id.movedFrom,
		}
		response.Assets = append(response.Assets, result)

		if assetMRN == "" {
			continue
		}
		if status != StatusFailed {
			currentMRNs = append(currentMRNs, assetMRN)
		}

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

		if status != StatusFailed {
			if err := s.AddCheckpoint(ctx, runID, "asset", assetMRN, result.Status, []string{assetHash}); err != nil {
				log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", assetMRN).Msg("Failed to add checkpoint")
			}
		}
	}

	if !inheritedBaseline {
		s.removeStale(ctx, run, runID, writer, assetCheckpoints, currentMRNs, unresolved, response)
	}

	for _, lin := range lineage {
		refs, err := s.assetService.ResolveMRNs(ctx, []string{lin.Source, lin.Target})
		if err != nil {
			return nil, fmt.Errorf("resolving lineage MRNs: %w", err)
		}
		lineageKey := lineageCheckpointKey(lin.Type, refs[0], refs[1])

		status := StatusCreated
		if checkpoint, exists := lastCheckpoints[lineageKey]; exists && checkpoint.Operation != StatusDeleted {
			status = StatusUpdated
		}

		// Every run writes the edge, whatever the checkpoint says. An edge
		// is rejected when either end is missing, which happens routinely:
		// a plugin emits an edge to a table another source has not been
		// ingested yet. The failure is still checkpointed, and a checkpoint
		// alone used to be taken as proof the edge was already there, so
		// the edge was never written again under that pipeline name.
		// Writing an edge that exists returns the existing one.
		if _, err := s.lineageService.CreateDeclaredLineage(ctx, refs[0], refs[1], lin.Type, lin.JobMRN, pipelineName, lin.ColumnLineage); err != nil {
			log.Error().Err(err).Str("source", refs[0]).Str("target", refs[1]).Str("type", lin.Type).Msg("Failed to create lineage")
			status = StatusFailed
		}

		result := LineageResult{
			Source: refs[0],
			Target: refs[1],
			Type:   lin.Type,
			Status: status,
		}
		response.Lineage = append(response.Lineage, result)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "lineage",
			EntityMRN:  lineageKey,
			EntityName: fmt.Sprintf("%s -> %s", refs[0], refs[1]),
			Status:     result.Status,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", lineageKey).Msg("Failed to add lineage run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "lineage", lineageKey, result.Status, []string{"source", "target", "type"}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", lineageKey).Msg("Failed to add lineage checkpoint")
		}
	}

	for _, doc := range docs {
		docAsset := doc.AssetMRN
		if resolved, err := s.assetService.ResolveMRN(ctx, doc.AssetMRN); err == nil && resolved != "" {
			docAsset = resolved
		}
		docKey := documentationCheckpointKey(doc.Type, docAsset)

		status := StatusCreated
		if checkpoint, exists := lastCheckpoints[docKey]; exists && checkpoint.Operation != StatusDeleted {
			status = StatusUpdated
		}

		result := DocumentationResult{
			AssetMRN: docAsset,
			Type:     doc.Type,
			Status:   status,
		}
		response.Documentation = append(response.Documentation, result)

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "documentation",
			EntityMRN:  docKey,
			EntityName: fmt.Sprintf("%s (%s)", docAsset, doc.Type),
			Status:     result.Status,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", docKey).Msg("Failed to add documentation run entity")
		}

		if err := s.AddCheckpoint(ctx, runID, "documentation", docKey, result.Status, []string{"asset_mrn", "type"}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", docKey).Msg("Failed to add documentation checkpoint")
		}
	}

	if len(stats) > 0 {
		s.processStatistics(ctx, stats)
	}

	return response, nil
}

// syncGlossary upserts the terms a run discovered and returns their ids
// by name, ready for the assets that reference them. A glossary is
// supporting detail: failing to write it is reported but never fails the
// run, so an ingestion still lands its assets.
func (s *service) syncGlossary(ctx context.Context, sourceName string, terms []GlossaryTermInput, response *ProcessAssetsResponse) map[string]string {
	if len(terms) == 0 || s.glossaryService == nil {
		return nil
	}

	inputs := make([]glossary.TermInput, 0, len(terms))
	for _, term := range terms {
		inputs = append(inputs, glossary.TermInput{
			Name:        term.Name,
			Definition:  term.Definition,
			Description: term.Description,
			Parent:      term.Parent,
			Synonyms:    term.Synonyms,
			Tags:        term.Tags,
			Metadata:    term.Metadata,
		})
	}

	result, err := s.glossaryService.SyncTerms(ctx, sourceName, inputs)
	if err != nil {
		log.Error().Err(err).Str("source", sourceName).Msg("Failed to sync glossary terms")
		return nil
	}

	response.Glossary = &GlossaryResult{
		TermsCreated:   result.Created,
		TermsUpdated:   result.Updated,
		TermsUnchanged: result.Unchanged,
	}

	return result.IDsByName
}

// linkTerms attaches an asset to the glossary terms it names, and
// reports whether any link was written. Existing links survive: the
// write only adds rows, so a term someone attached by hand stays put
// with its own source on it.
func (s *service) linkTerms(ctx context.Context, assetID string, names []string, idsByName map[string]string, sourceName string) bool {
	termIDs := resolveTermIDs(names, idsByName)
	if len(termIDs) == 0 {
		return false
	}

	if err := s.assetService.AddTerms(ctx, assetID, termIDs, sourceName, ""); err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to attach glossary terms to asset")
		return false
	}

	return true
}

// resolveTermIDs turns the term names an asset carries into term ids,
// dropping duplicates and any name this run's glossary does not know.
func resolveTermIDs(names []string, idsByName map[string]string) []string {
	termIDs := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))

	for _, name := range names {
		termID, ok := idsByName[strings.TrimSpace(name)]
		if !ok {
			log.Debug().Str("term", name).Msg("Asset references an unknown glossary term")
			continue
		}
		if seen[termID] {
			continue
		}
		seen[termID] = true
		termIDs = append(termIDs, termID)
	}

	return termIDs
}

func (s *service) processStatistics(ctx context.Context, statistics []StatisticInput) {
	if len(statistics) == 0 {
		return
	}

	metricsToRecord := make([]metrics.Metric, 0, len(statistics))
	now := time.Now()

	refs := make([]string, len(statistics))
	for i, stat := range statistics {
		refs[i] = stat.AssetMRN
	}
	if resolved, err := s.assetService.ResolveMRNs(ctx, refs); err == nil {
		for i := range statistics {
			if resolved[i] != "" {
				statistics[i].AssetMRN = resolved[i]
			}
		}
	}

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
	return s.ProcessEntities(ctx, runID, assets, nil, nil, nil, nil, pipelineName, sourceName)
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

	checkpoints, _, err := s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName, "")
	return checkpoints, err
}

// DestroyPipeline removes everything a pipeline put in the catalog, under every source and every owner. Its contributions are withdrawn from the assets it described, so an asset another writer also describes stays, and one only this pipeline described is deleted.
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
		if run.SourceName != "destroy" {
			sourceNames[run.SourceName] = true
		}
	}

	response := &DestroyRunResponse{DeletedEntityMRNs: []string{}}

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
	track := func(entityType, entityMRN, status, message string) {
		entity := &RunEntity{ID: uuid.New().String(), RunID: destroyRunID, EntityType: entityType, EntityMRN: entityMRN, Status: status, ErrorMessage: message, CreatedAt: time.Now()}
		if err := s.repo.AddRunEntity(ctx, destroyRun.ID, entity); err != nil {
			log.Error().Err(err).Str("destroy_run_id", destroyRunID).Str("entity_mrn", entityMRN).Msg("Failed to track entity deletion")
		}
	}

	// Edges and documentation are found through the checkpoints of every owner's latest run.
	for sourceName := range sourceNames {
		owners, err := s.repo.ListOwners(ctx, pipelineName, sourceName)
		if err != nil {
			log.Warn().Err(err).Str("pipeline", pipelineName).Str("source", sourceName).Msg("Failed to list run owners, skipping")
			continue
		}
		for _, owner := range owners {
			checkpoints, baselineOwner, err := s.repo.GetLastRunCheckpoints(ctx, pipelineName, sourceName, owner)
			if err != nil || baselineOwner != owner {
				continue
			}
			for key, checkpoint := range checkpoints {
				if checkpoint.Operation == StatusDeleted {
					continue
				}
				switch checkpoint.EntityType {
				case "lineage":
					if s.deleteCheckpointedEdge(ctx, key) {
						response.LineageDeleted++
						response.DeletedEntityMRNs = append(response.DeletedEntityMRNs, key)
						track("lineage", key, StatusDeleted, "")
					}
				case "documentation":
					response.DocumentationDeleted++
				}
			}
		}
	}

	assetIDs, err := s.assetService.AssetIDsByWriter(ctx, writerPrefix(pipelineName)+"*")
	if err != nil {
		return nil, fmt.Errorf("listing the pipeline's assets: %w", err)
	}
	for _, assetID := range assetIDs {
		a, err := s.assetService.Get(ctx, assetID)
		if err != nil {
			continue
		}
		contribs, err := s.assetService.Contributions(ctx, assetID)
		if err != nil {
			continue
		}
		deleted := false
		for _, c := range contribs {
			if !strings.HasPrefix(c.Writer, writerPrefix(pipelineName)) {
				continue
			}
			gone, err := s.assetService.Withdraw(ctx, assetID, c.Writer)
			if err != nil {
				log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to withdraw the pipeline's contribution")
				track("asset", *a.MRN, StatusFailed, err.Error())
				break
			}
			deleted = deleted || gone
		}
		if deleted {
			response.AssetsDeleted++
			response.DeletedEntityMRNs = append(response.DeletedEntityMRNs, *a.MRN)
			track("asset", *a.MRN, StatusDeleted, "")
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
		Dur("duration", completedAt.Sub(now)).
		Msg("Pipeline destruction completed")

	return response, nil
}

// deleteCheckpointedEdge deletes the edge a lineage checkpoint names. Keys from before kinded MRNs mangled the MRNs inside them beyond recovery and are skipped.
func (s *service) deleteCheckpointedEdge(ctx context.Context, key string) bool {
	lineageType, source, target, ok := parseLineageCheckpointKey(key)
	if !ok {
		return false
	}
	sourceAsset, err := s.assetService.GetByMRN(ctx, source)
	if err != nil {
		return false
	}
	resolved, err := s.assetService.ResolveMRNs(ctx, []string{source, target})
	if err != nil {
		return false
	}
	lineageResp, err := s.lineageService.GetAssetLineage(ctx, sourceAsset.ID, 1000, "downstream")
	if err != nil {
		return false
	}
	for _, edge := range lineageResp.Edges {
		if edge.Source == resolved[0] && edge.Target == resolved[1] && strings.EqualFold(edge.Type, lineageType) {
			if err := s.lineageService.DeleteDirectLineage(ctx, edge.ID); err != nil {
				log.Error().Err(err).Str("edge_id", edge.ID).Msg("Failed to delete lineage edge")
				return false
			}
			return true
		}
	}
	return false
}

func (s *service) GetStaleEntities(ctx context.Context, lastCheckpoints map[string]*plugin.RunCheckpoint, currentEntityMRNs []string) []string {
	currentSet := make(map[string]bool)
	for _, mrn := range currentEntityMRNs {
		currentSet[mrn] = true
	}

	var staleEntities []string
	for mrn, checkpoint := range lastCheckpoints {
		// The checkpoint map is keyed by MRN alone, so it holds lineage and
		// documentation rows alongside assets, while currentEntityMRNs only
		// ever holds assets. Without this guard every lineage checkpoint
		// looks stale and is handed to the asset deleter, which both inflates
		// the reported deletion count and leaves lineage permanently absent:
		// the lineage loop only recreates an edge whose checkpoint is
		// missing, and a deleted-then-recorded checkpoint is not missing.
		if checkpoint.EntityType != "asset" {
			continue
		}
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
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting run: %w", err)
	}

	return run, nil
}

func (s *service) GetByRunID(ctx context.Context, runID string) (*plugin.Run, error) {
	if runID == "" {
		return nil, fmt.Errorf("%w: run_id is required", ErrInvalidInput)
	}
	return s.repo.GetByRunID(ctx, runID)
}

func (s *service) ListRunEntities(ctx context.Context, runID, entityType, status string, limit, offset int) ([]*RunEntity, int, error) {
	if runID == "" {
		return nil, 0, fmt.Errorf("%w: run_id is required", ErrInvalidInput)
	}

	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, 0, ErrNotFound
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
	// Only the names take part in the hash. Widening it to the whole source
	// would change every existing asset's hash and mark the entire catalog
	// as modified on the next run.
	sourceNames := make([]string, len(asset.Sources))
	for i, src := range asset.Sources {
		sourceNames[i] = src.Name
	}

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
		Sources:       sourceNames,
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
