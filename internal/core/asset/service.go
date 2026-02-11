package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type AssetSource struct {
	Name       string                 `json:"name"`
	LastSyncAt time.Time              `json:"last_sync_at"`
	Properties map[string]interface{} `json:"properties"`
	Priority   int                    `json:"priority"`
}

type ExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

type Asset struct {
	ID              string                 `json:"id,omitempty"`
	ParentMRN       *string                `json:"parent_mrn,omitempty"`
	Name            *string                `json:"name,omitempty"`
	Description     *string                `json:"description,omitempty"`
	UserDescription *string                `json:"user_description,omitempty"`
	Type            string                 `json:"type"`
	Providers       []string               `json:"providers"`
	MRN             *string                `json:"mrn,omitempty"`
	Schema          map[string]string      `json:"schema,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Sources         []AssetSource          `json:"sources,omitempty"`
	Tags            []string               `json:"tags,omitempty"`
	Environments    map[string]Environment `json:"environments,omitempty"`
	Query           *string                `json:"query,omitempty"`
	QueryLanguage   *string                `json:"query_language,omitempty"`
	IsStub          bool                   `json:"is_stub"`
	ExternalLinks   []ExternalLink         `json:"external_links,omitempty"`
	HasRunHistory   bool                   `json:"has_run_history"`
	CreatedAt       time.Time              `json:"created_at,omitempty"`
	UpdatedAt       time.Time              `json:"updated_at,omitempty"`
	LastSyncAt      time.Time              `json:"last_sync_at,omitempty"`
	CreatedBy       string                 `json:"created_by,omitempty"`
}

type Environment struct {
	Name     string                 `json:"name"`
	Path     string                 `json:"path"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type CreateInput struct {
	Name          *string                `json:"name" validate:"required"`
	MRN           *string                `json:"mrn" validate:"required"`
	Type          string                 `json:"type" validate:"required"`
	Providers     []string               `json:"providers" validate:"required"`
	Description   *string                `json:"description"`
	Metadata      map[string]interface{} `json:"metadata"`
	Schema        map[string]string      `json:"schema"`
	Tags          []string               `json:"tags"`
	CreatedBy     string                 `json:"created_by" validate:"required"`
	Sources       []AssetSource          `json:"sources"`
	Environments  map[string]Environment `json:"environments"`
	ExternalLinks []ExternalLink         `json:"external_links"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
	IsStub        bool                   `json:"is_stub"`
}

type UpdateInput struct {
	Name             *string                `json:"name"`
	Description      *string                `json:"description"`
	UserDescription  *string                `json:"user_description"`
	Metadata         map[string]interface{} `json:"metadata"`
	Type             string                 `json:"type"`
	Providers        []string               `json:"providers"`
	Schema           map[string]string      `json:"schema"`
	Tags             []string               `json:"tags"`
	Sources          []AssetSource          `json:"sources"`
	Environments     map[string]Environment `json:"environments"`
	ExternalLinks    []ExternalLink         `json:"external_links"`
	Query            *string                `json:"query,omitempty"`
	QueryLanguage    *string                `json:"query_language,omitempty"`
	SkipNotification bool                   `json:"-"`
}

type Filter struct {
	Types        []string   `json:"types,omitempty"`
	Providers    []string   `json:"providers,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	ParentMRN    *string    `json:"parent_mrn,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	UpdatedAfter *time.Time `json:"updated_after,omitempty"`
	Limit        int        `json:"limit,omitempty"`
	Offset       int        `json:"offset,omitempty"`
	Environment  *string    `json:"environment,omitempty"`
	IncludeStubs bool       `json:"include_stubs,omitempty"`
	OwnerType    *string    `json:"owner_type,omitempty"`
	OwnerID      *string    `json:"owner_id,omitempty"`
}

type SearchFilter struct {
	Query        string   `json:"query" validate:"omitempty"`
	Types        []string `json:"types" validate:"omitempty"`
	Providers    []string `json:"providers" validate:"omitempty"`
	Tags         []string `json:"tags" validate:"omitempty"`
	Limit        int      `json:"limit" validate:"omitempty,gte=0"`
	Offset       int      `json:"offset" validate:"omitempty,gte=0"`
	IncludeStubs bool     `json:"include_stubs,omitempty"`
	OwnerType    *string  `json:"owner_type,omitempty"`
	OwnerID      *string  `json:"owner_id,omitempty"`
}

type MetadataContext struct {
	Query   string            `json:"query"`
	Filters map[string]string `json:"filters"`
}

type MetadataFieldSuggestion struct {
	Field     string      `json:"field"`
	Type      string      `json:"type"`
	Count     int         `json:"count"`
	Example   interface{} `json:"example"`
	PathParts []string    `json:"path_parts"`
	Types     []string    `json:"types"`
}

type MetadataValueSuggestion struct {
	Value   string `json:"value"`
	Count   int    `json:"count"`
	Example *Asset `json:"example,omitempty"`
}

type RunHistory struct {
	ID           string     `json:"id"`
	RunID        string     `json:"run_id"`
	JobName      string     `json:"job_name"`
	JobNamespace string     `json:"job_namespace"`
	Status       string     `json:"status"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	DurationMs   *int64     `json:"duration_ms,omitempty"`
	Type         string     `json:"type"`
	EventTime    time.Time  `json:"event_time"`
}

type HistogramBucket struct {
	Date     string `json:"date"`
	Total    int    `json:"total"`
	Complete int    `json:"complete"`
	Fail     int    `json:"fail"`
	Running  int    `json:"running"`
	Abort    int    `json:"abort"`
	Other    int    `json:"other"`
}

type AssetTerm struct {
	TermID            string    `json:"term_id"`
	TermName          string    `json:"term_name"`
	Definition        string    `json:"definition"`
	Source            string    `json:"source"` // "user" or "plugin:name"
	CreatedAt         time.Time `json:"created_at"`
	CreatedBy         *string   `json:"created_by,omitempty"`
	CreatedByUsername *string   `json:"created_by_username,omitempty"`
}

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrAssetNotFound = errors.New("asset not found")
	ErrAlreadyExists = errors.New("asset already exists")
)

type Service interface {
	Create(ctx context.Context, input CreateInput) (*Asset, error)
	Get(ctx context.Context, id string) (*Asset, error)
	GetByMRN(ctx context.Context, qualifiedName string) (*Asset, error)
	Search(ctx context.Context, filter SearchFilter, calculateCounts bool) ([]*Asset, int, AvailableFilters, error)
	GetMyAssets(ctx context.Context, userID string, teamIDs []string, limit, offset int) ([]*Asset, int, error)
	Summary(ctx context.Context) (*AssetSummary, error)
	Update(ctx context.Context, id string, input UpdateInput) (*Asset, error)
	Delete(ctx context.Context, id string) error
	DeleteByMRN(ctx context.Context, mrn string) error
	AddTag(ctx context.Context, id string, tag string) (*Asset, error)
	RemoveTag(ctx context.Context, id string, tag string) (*Asset, error)
	ListByPattern(ctx context.Context, pattern string, assetType string) ([]*Asset, error)
	GetByMRNs(ctx context.Context, mrns []string) (map[string]*Asset, error)
	GetByTypeAndName(ctx context.Context, assetType, name string) (*Asset, error)
	GetMetadataFields(ctx context.Context, queryContext *MetadataContext) ([]MetadataFieldSuggestion, error)
	GetMetadataValues(ctx context.Context, field string, prefix string, limit int, queryContext *MetadataContext) ([]MetadataValueSuggestion, error)
	GetTagSuggestions(ctx context.Context, prefix string, limit int) ([]string, error)
	GetRunHistory(ctx context.Context, assetID string, limit, offset int) ([]*RunHistory, int, error)
	GetRunHistoryHistogram(ctx context.Context, assetID string, days int) ([]HistogramBucket, error)

	AddTerms(ctx context.Context, assetID string, termIDs []string, source string, createdBy string) error
	RemoveTerm(ctx context.Context, assetID string, termID string) error
	GetTerms(ctx context.Context, assetID string) ([]AssetTerm, error)
	GetAssetsByTerm(ctx context.Context, termID string, limit, offset int) ([]*Asset, int, error)

	// SetMembershipObserver registers an observer for asset create/delete events.
	SetMembershipObserver(observer MembershipObserver)
	// AddMembershipObserver registers an additional observer for asset create/delete events.
	AddMembershipObserver(observer MembershipObserver)
	// SetNotificationObserver registers an observer for asset update notifications.
	SetNotificationObserver(observer NotificationObserver)
}

// MembershipObserver is notified when assets are created or deleted.
// This allows the data product membership service to update memberships.
type MembershipObserver interface {
	OnAssetCreated(ctx context.Context, asset *Asset)
	OnAssetDeleted(ctx context.Context, assetID string) error
}

// NotificationObserver is notified when assets are modified.
type NotificationObserver interface {
	OnAssetUpdated(ctx context.Context, asset *Asset, changeType string)
	OnAssetDeleted(ctx context.Context, asset *Asset)
}

// summaryCache holds cached summary data with TTL
type summaryCache struct {
	sync.RWMutex
	data      *AssetSummary
	expiresAt time.Time
}

// metadataFieldsCache holds cached metadata field suggestions with TTL
type metadataFieldsCache struct {
	sync.RWMutex
	data      []MetadataFieldSuggestion
	expiresAt time.Time
}

const summaryCacheTTL = 5 * time.Second
const metadataFieldsCacheTTL = 30 * time.Second

type service struct {
	repo                 Repository
	validator            *validator.Validate
	metrics              MetricsClient
	membershipObserver   MembershipObserver
	membershipObservers  []MembershipObserver
	notificationObserver NotificationObserver
	summaryCache         summaryCache
	metadataFieldsCache  metadataFieldsCache
}

type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, err error, fields ...interface{})
}

type MetricsClient interface {
	Count(name string, value int64, tags ...string)
	Timing(name string, value time.Duration, tags ...string)
}

type ServiceOption func(*service)

func NewService(repo Repository, opts ...ServiceOption) Service {
	s := &service{
		repo:      repo,
		validator: validator.New(),
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

func (s *service) SetMembershipObserver(observer MembershipObserver) {
	s.membershipObserver = observer
}

func (s *service) AddMembershipObserver(observer MembershipObserver) {
	s.membershipObservers = append(s.membershipObservers, observer)
}

func (s *service) SetNotificationObserver(observer NotificationObserver) {
	s.notificationObserver = observer
}

func (s *service) GetRunHistoryHistogram(ctx context.Context, assetID string, days int) ([]HistogramBucket, error) {
	if days <= 0 || days > 365 {
		return nil, fmt.Errorf("invalid days parameter: must be between 1 and 365")
	}

	return s.repo.GetRunHistoryHistogram(ctx, assetID, days)
}

func (s *service) GetRunHistory(ctx context.Context, assetID string, limit, offset int) ([]*RunHistory, int, error) {
	if limit <= 0 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.GetRunHistory(ctx, assetID, limit, offset)
}

func (s *service) GetMetadataFields(ctx context.Context, queryContext *MetadataContext) ([]MetadataFieldSuggestion, error) {
	if queryContext != nil && queryContext.Query != "" {
		fields, err := s.repo.GetMetadataFieldsWithContext(ctx, queryContext)
		if err != nil {
			return nil, fmt.Errorf("getting metadata fields with context: %w", err)
		}
		return fields, nil
	}

	s.metadataFieldsCache.RLock()
	if s.metadataFieldsCache.data != nil && time.Now().Before(s.metadataFieldsCache.expiresAt) {
		cached := s.metadataFieldsCache.data
		s.metadataFieldsCache.RUnlock()
		return cached, nil
	}
	s.metadataFieldsCache.RUnlock()

	fields, err := s.repo.GetMetadataFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting metadata fields: %w", err)
	}

	s.metadataFieldsCache.Lock()
	s.metadataFieldsCache.data = fields
	s.metadataFieldsCache.expiresAt = time.Now().Add(metadataFieldsCacheTTL)
	s.metadataFieldsCache.Unlock()

	return fields, nil
}

func (s *service) GetMetadataValues(ctx context.Context, field string, prefix string, limit int, queryContext *MetadataContext) ([]MetadataValueSuggestion, error) {
	if queryContext == nil || queryContext.Query == "" {
		return s.repo.GetMetadataValues(ctx, field, prefix, limit)
	}

	values, err := s.repo.GetMetadataValuesWithContext(ctx, field, prefix, limit, queryContext)
	if err != nil {
		return nil, fmt.Errorf("getting metadata values with context: %w", err)
	}

	return values, nil
}

func (s *service) GetTagSuggestions(ctx context.Context, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}

	tags, err := s.repo.GetTagSuggestions(ctx, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("getting tag suggestions: %w", err)
	}

	validTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			validTags = append(validTags, tag)
		}
	}

	return validTags, nil
}

func (s *service) GetByMRNs(ctx context.Context, mrns []string) (map[string]*Asset, error) {
	assets, err := s.repo.GetByMRNs(ctx, mrns)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*Asset)
	for _, ast := range assets {
		if ast.MRN != nil {
			result[*ast.MRN] = ast
		}
	}
	return result, nil
}

func (s *service) ListByPattern(ctx context.Context, pattern string, assetType string) ([]*Asset, error) {
	assets, err := s.repo.ListByPattern(ctx, pattern, assetType)
	if err != nil {
		return nil, fmt.Errorf("listing assets by pattern: %w", err)
	}
	return assets, nil
}

func (s *service) Create(ctx context.Context, input CreateInput) (*Asset, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	existing, err := s.repo.GetByMRN(ctx, *input.MRN)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("checking existing asset: %w", err)
	}

	if existing != nil {
		return nil, ErrAlreadyExists
	}

	if input.Schema == nil {
		input.Schema = make(map[string]string)
	}

	now := time.Now()
	asset := &Asset{
		ID:            uuid.New().String(),
		Name:          input.Name,
		MRN:           input.MRN,
		Type:          input.Type,
		Providers:     input.Providers,
		Description:   input.Description,
		Metadata:      input.Metadata,
		Schema:        input.Schema,
		Sources:       input.Sources,
		Environments:  input.Environments,
		Tags:          input.Tags,
		ExternalLinks: input.ExternalLinks,
		CreatedBy:     input.CreatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastSyncAt:    now,
		Query:         input.Query,
		QueryLanguage: input.QueryLanguage,
		IsStub:        input.IsStub,
	}
	if asset.Tags == nil {
		asset.Tags = []string{}
	}

	if err := s.repo.Create(ctx, asset); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}

	// Notify membership observers asynchronously
	if s.membershipObserver != nil {
		s.membershipObserver.OnAssetCreated(ctx, asset)
	}
	for _, observer := range s.membershipObservers {
		observer.OnAssetCreated(ctx, asset)
	}

	return asset, nil
}

func (s *service) GetByTypeAndName(ctx context.Context, assetType, name string) (*Asset, error) {
	asset, err := s.repo.GetByTypeAndName(ctx, assetType, name)
	if err != nil {
		return nil, errors.New("asset not found")
	}
	return asset, nil
}

func (s *service) Get(ctx context.Context, id string) (*Asset, error) {
	asset, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}
	return asset, nil
}

func (s *service) GetByMRN(ctx context.Context, qualifiedName string) (*Asset, error) {
	asset, err := s.repo.GetByMRN(ctx, qualifiedName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("failed to get asset by MRN: %w", err)
	}
	return asset, nil
}

func (s *service) Search(ctx context.Context, filter SearchFilter, calculateCounts bool) ([]*Asset, int, AvailableFilters, error) {
	if err := s.validator.Struct(filter); err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	assets, total, availableFilters, err := s.repo.Search(ctx, filter, calculateCounts)
	if err != nil {
		return nil, 0, AvailableFilters{}, fmt.Errorf("failed to search assets: %w", err)
	}

	return assets, total, availableFilters, nil
}

func (s *service) Summary(ctx context.Context) (*AssetSummary, error) {
	s.summaryCache.RLock()
	if s.summaryCache.data != nil && time.Now().Before(s.summaryCache.expiresAt) {
		cached := s.summaryCache.data
		s.summaryCache.RUnlock()
		return cached, nil
	}
	s.summaryCache.RUnlock()

	summary, err := s.repo.Summary(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	s.summaryCache.Lock()
	s.summaryCache.data = summary
	s.summaryCache.expiresAt = time.Now().Add(summaryCacheTTL)
	s.summaryCache.Unlock()

	return summary, nil
}

func (s *service) Update(ctx context.Context, id string, input UpdateInput) (*Asset, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	asset, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("getting asset: %w", err)
	}

	updated := false
	schemaUpdated := false

	if input.Name != nil {
		asset.Name = input.Name
		updated = true
	}
	if input.Description != nil {
		asset.Description = input.Description
		updated = true
	}
	if input.UserDescription != nil {
		if *input.UserDescription == "" {
			asset.UserDescription = nil
		} else {
			asset.UserDescription = input.UserDescription
		}
		updated = true
	}
	if input.Metadata != nil {
		asset.Metadata = input.Metadata
		updated = true
	}
	if input.Schema != nil {
		if input.Schema == nil {
			input.Schema = make(map[string]string)
		}
		asset.Schema = input.Schema
		updated = true
		schemaUpdated = true
	}
	if input.Tags != nil {
		asset.Tags = input.Tags
		updated = true
	}
	if input.Sources != nil {
		asset.Sources = UpdateSources(asset.Sources, input.Sources)
		updated = true
	}
	if input.Environments != nil {
		if asset.Environments == nil {
			asset.Environments = make(map[string]Environment)
		}
		for k, v := range input.Environments {
			asset.Environments[k] = v
		}
		updated = true
	}
	if input.ExternalLinks != nil {
		asset.ExternalLinks = input.ExternalLinks
		updated = true
	}
	if input.Query != nil {
		asset.Query = input.Query
		updated = true
	}
	if input.QueryLanguage != nil {
		asset.QueryLanguage = input.QueryLanguage
		updated = true
	}

	if !updated {
		return asset, nil
	}

	asset.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to update asset: %w", err)
	}

	if s.notificationObserver != nil && !input.SkipNotification {
		changeType := "asset_change"
		if schemaUpdated {
			changeType = "schema_change"
		}
		s.notificationObserver.OnAssetUpdated(ctx, asset, changeType)
	}

	return asset, nil
}

func UpdateSources(existing, new []AssetSource) []AssetSource {
	sourceMap := make(map[string]AssetSource)

	for _, src := range existing {
		if src.Name != "" {
			sourceMap[src.Name] = src
		}
	}

	for _, src := range new {
		if src.Name == "" {
			continue
		}

		existingSource := sourceMap[src.Name]

		if src.Properties != nil {
			existingSource.Properties = src.Properties
		}
		if !src.LastSyncAt.IsZero() {
			existingSource.LastSyncAt = src.LastSyncAt
		}
		if src.Priority != 0 {
			existingSource.Priority = src.Priority
		}

		existingSource.Name = src.Name
		sourceMap[src.Name] = existingSource
	}

	result := make([]AssetSource, 0, len(sourceMap))
	for _, src := range sourceMap {
		result = append(result, src)
	}

	return result
}

func (s *service) Delete(ctx context.Context, id string) error {
	asset, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("fetching asset before deletion: %w", err)
	}

	if s.notificationObserver != nil {
		s.notificationObserver.OnAssetDeleted(ctx, asset)
	}

	// Notify membership observers before deletion
	if s.membershipObserver != nil {
		if err := s.membershipObserver.OnAssetDeleted(ctx, id); err != nil {
			log.Warn().Err(err).Str("asset_id", id).Msg("Failed to notify membership observer of deletion")
		}
	}
	for _, observer := range s.membershipObservers {
		if err := observer.OnAssetDeleted(ctx, id); err != nil {
			log.Warn().Err(err).Str("asset_id", id).Msg("Failed to notify membership observer of deletion")
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("failed to delete asset: %w", err)
	}

	log.Info().
		Str("asset_id", id).
		Msg("Asset deleted")

	if s.metrics != nil {
		s.metrics.Count("asset.deleted", 1)
	}

	return nil
}

func (s *service) DeleteByMRN(ctx context.Context, mrn string) error {
	asset, err := s.repo.GetByMRN(ctx, mrn)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("fetching asset before deletion: %w", err)
	}

	if s.notificationObserver != nil {
		s.notificationObserver.OnAssetDeleted(ctx, asset)
	}

	if s.membershipObserver != nil {
		if err := s.membershipObserver.OnAssetDeleted(ctx, asset.ID); err != nil {
			log.Warn().Err(err).Str("asset_id", asset.ID).Msg("Failed to notify membership observer of deletion")
		}
	}
	for _, observer := range s.membershipObservers {
		if err := observer.OnAssetDeleted(ctx, asset.ID); err != nil {
			log.Warn().Err(err).Str("asset_id", asset.ID).Msg("Failed to notify membership observer of deletion")
		}
	}

	if err := s.repo.DeleteByMRN(ctx, mrn); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("failed to delete asset by MRN: %w", err)
	}

	log.Info().
		Str("asset_mrn", mrn).
		Msg("Asset deleted by MRN")

	if s.metrics != nil {
		s.metrics.Count("asset.deleted", 1)
	}

	return nil
}

func (s *service) AddTag(ctx context.Context, id string, tag string) (*Asset, error) {
	asset, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("getting asset: %w", err)
	}

	for _, existingTag := range asset.Tags {
		if existingTag == tag {
			return asset, nil
		}
	}

	asset.Tags = append(asset.Tags, tag)
	asset.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to add tag to asset: %w", err)
	}

	log.Debug().
		Str("asset_id", id).
		Str("tag", tag).
		Msg("Asset tag added")

	if s.metrics != nil {
		s.metrics.Count("asset.tag.added", 1)
	}

	return asset, nil
}

func (s *service) RemoveTag(ctx context.Context, assetId string, tag string) (*Asset, error) {
	asset, err := s.repo.Get(ctx, assetId)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("getting asset: %w", err)
	}

	found := false
	newTags := make([]string, 0, len(asset.Tags))
	for _, existingTag := range asset.Tags {
		if existingTag != tag {
			newTags = append(newTags, existingTag)
		} else {
			found = true
		}
	}

	if !found {
		return asset, nil
	}

	asset.Tags = newTags
	asset.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to remove tag from asset: %w", err)
	}

	log.Debug().
		Str("asset_id", assetId).
		Str("tag", tag).
		Msg("Asset tag removed")

	if s.metrics != nil {
		s.metrics.Count("asset.tag.removed", 1)
	}

	return asset, nil
}

func (s *service) AddTerms(ctx context.Context, assetID string, termIDs []string, source string, createdBy string) error {
	_, err := s.repo.Get(ctx, assetID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("verifying asset exists: %w", err)
	}

	if err := s.repo.AddTerms(ctx, assetID, termIDs, source, createdBy); err != nil {
		return fmt.Errorf("adding terms to asset: %w", err)
	}

	log.Debug().
		Str("asset_id", assetID).
		Int("term_count", len(termIDs)).
		Msg("Terms added to asset")

	if s.metrics != nil {
		s.metrics.Count("asset.terms.added", int64(len(termIDs)))
	}

	return nil
}

func (s *service) RemoveTerm(ctx context.Context, assetID string, termID string) error {
	if err := s.repo.RemoveTerm(ctx, assetID, termID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("removing term from asset: %w", err)
	}

	log.Debug().
		Str("asset_id", assetID).
		Str("term_id", termID).
		Msg("Term removed from asset")

	if s.metrics != nil {
		s.metrics.Count("asset.terms.removed", 1)
	}

	return nil
}

func (s *service) GetTerms(ctx context.Context, assetID string) ([]AssetTerm, error) {
	terms, err := s.repo.GetTerms(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("getting asset terms: %w", err)
	}

	return terms, nil
}

func (s *service) GetAssetsByTerm(ctx context.Context, termID string, limit, offset int) ([]*Asset, int, error) {
	assets, total, err := s.repo.GetAssetsByTerm(ctx, termID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("getting assets by term: %w", err)
	}

	return assets, total, nil
}

func (s *service) GetMyAssets(ctx context.Context, userID string, teamIDs []string, limit, offset int) ([]*Asset, int, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	assets, total, err := s.repo.GetMyAssets(ctx, userID, teamIDs, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("getting user assets: %w", err)
	}

	return assets, total, nil
}
