package assetrule

import (
	"context"
	"fmt"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/rs/zerolog/log"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

// CreateInput is the input for creating an asset rule.
type CreateInput struct {
	Name            string              `json:"name" validate:"required,min=1,max=255"`
	Description     *string             `json:"description,omitempty"`
	Links           []ExternalLink      `json:"links,omitempty"`
	TermIDs         []string            `json:"term_ids,omitempty"`
	RuleType        enrichment.RuleType `json:"rule_type" validate:"required,oneof=query metadata_match"`
	QueryExpression *string             `json:"query_expression,omitempty"`
	MetadataField   *string             `json:"metadata_field,omitempty"`
	PatternType     *string             `json:"pattern_type,omitempty" validate:"omitempty,oneof=exact wildcard regex prefix"`
	PatternValue    *string             `json:"pattern_value,omitempty"`
	Priority        int                 `json:"priority"`
	IsEnabled       bool                `json:"is_enabled"`
}

// UpdateInput is the input for updating an asset rule.
type UpdateInput struct {
	Name            *string              `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description     *string              `json:"description,omitempty"`
	Links           []ExternalLink       `json:"links,omitempty"`
	TermIDs         []string             `json:"term_ids,omitempty"`
	RuleType        *enrichment.RuleType `json:"rule_type,omitempty" validate:"omitempty,oneof=query metadata_match"`
	QueryExpression *string              `json:"query_expression,omitempty"`
	MetadataField   *string              `json:"metadata_field,omitempty"`
	PatternType     *string              `json:"pattern_type,omitempty" validate:"omitempty,oneof=exact wildcard regex prefix"`
	PatternValue    *string              `json:"pattern_value,omitempty"`
	Priority        *int                 `json:"priority,omitempty"`
	IsEnabled       *bool                `json:"is_enabled,omitempty"`
}

// RulePreviewInput is the input for previewing a rule.
type RulePreviewInput struct {
	RuleType        enrichment.RuleType `json:"rule_type" validate:"required,oneof=query metadata_match"`
	QueryExpression *string             `json:"query_expression,omitempty"`
	MetadataField   *string             `json:"metadata_field,omitempty"`
	PatternType     *string             `json:"pattern_type,omitempty"`
	PatternValue    *string             `json:"pattern_value,omitempty"`
}

// Implement enrichment.EnrichmentRule for RulePreviewInput.
func (r *RulePreviewInput) GetID() string                    { return "" }
func (r *RulePreviewInput) GetRuleType() enrichment.RuleType { return r.RuleType }
func (r *RulePreviewInput) GetQueryExpression() *string       { return r.QueryExpression }
func (r *RulePreviewInput) GetMetadataField() *string         { return r.MetadataField }
func (r *RulePreviewInput) GetPatternType() *string           { return r.PatternType }
func (r *RulePreviewInput) GetPatternValue() *string          { return r.PatternValue }
func (r *RulePreviewInput) GetIsEnabled() bool                { return true }

// Service provides business logic for asset rules.
type Service interface {
	Create(ctx context.Context, input CreateInput, createdBy *string) (*AssetRule, error)
	Get(ctx context.Context, id string) (*AssetRule, error)
	Update(ctx context.Context, id string, input UpdateInput) (*AssetRule, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)
	PreviewRule(ctx context.Context, input RulePreviewInput, limit int) (*RulePreview, error)
	GetRuleAssets(ctx context.Context, ruleID string, limit, offset int) ([]string, int, error)
	GetEnrichedLinks(ctx context.Context, assetID string) ([]EnrichedExternalLink, error)
}

type service struct {
	repo       Repository
	memberRepo MembershipRepository
	evaluator  *enrichment.Evaluator
	memberSvc  *MembershipService
	validator  *validator.Validate
}

// NewService creates a new asset rule service.
func NewService(
	repo Repository,
	memberRepo MembershipRepository,
	evaluator *enrichment.Evaluator,
	memberSvc *MembershipService,
) Service {
	return &service{
		repo:       repo,
		memberRepo: memberRepo,
		evaluator:  evaluator,
		memberSvc:  memberSvc,
		validator:  validator.New(),
	}
}

func (s *service) Create(ctx context.Context, input CreateInput, createdBy *string) (*AssetRule, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if len(input.Links) == 0 && len(input.TermIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one link or term is required", ErrInvalidInput)
	}

	// Build a temporary rule for validation
	tempRule := &AssetRule{
		RuleType:        input.RuleType,
		QueryExpression: input.QueryExpression,
		MetadataField:   input.MetadataField,
		PatternType:     input.PatternType,
		PatternValue:    input.PatternValue,
		IsEnabled:       input.IsEnabled,
	}
	if err := enrichment.ValidateRule(tempRule); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	now := time.Now().UTC()
	rule := &AssetRule{
		Name:            input.Name,
		Description:     input.Description,
		Links:           input.Links,
		TermIDs:         input.TermIDs,
		RuleType:        input.RuleType,
		QueryExpression: input.QueryExpression,
		MetadataField:   input.MetadataField,
		PatternType:     input.PatternType,
		PatternValue:    input.PatternValue,
		Priority:        input.Priority,
		IsEnabled:       input.IsEnabled,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if rule.Links == nil {
		rule.Links = []ExternalLink{}
	}
	if rule.TermIDs == nil {
		rule.TermIDs = []string{}
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}

	if s.memberSvc != nil {
		if err := s.memberSvc.OnRuleCreated(ctx, rule); err != nil {
			log.Warn().Err(err).Str("rule_id", rule.ID).Msg("Failed to evaluate asset rule on create, will reconcile later")
		}
	}

	return s.repo.Get(ctx, rule.ID)
}

func (s *service) Get(ctx context.Context, id string) (*AssetRule, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, id string, input UpdateInput) (*AssetRule, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Description != nil {
		existing.Description = input.Description
	}
	if input.Links != nil {
		existing.Links = input.Links
	}
	if input.TermIDs != nil {
		existing.TermIDs = input.TermIDs
	}
	if input.RuleType != nil {
		existing.RuleType = *input.RuleType
	}
	if input.QueryExpression != nil {
		existing.QueryExpression = input.QueryExpression
	}
	if input.MetadataField != nil {
		existing.MetadataField = input.MetadataField
	}
	if input.PatternType != nil {
		existing.PatternType = input.PatternType
	}
	if input.PatternValue != nil {
		existing.PatternValue = input.PatternValue
	}
	if input.Priority != nil {
		existing.Priority = *input.Priority
	}
	if input.IsEnabled != nil {
		existing.IsEnabled = *input.IsEnabled
	}

	if err := enrichment.ValidateRule(existing); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	if s.memberSvc != nil {
		if err := s.memberSvc.OnRuleUpdated(ctx, existing); err != nil {
			log.Warn().Err(err).Str("rule_id", existing.ID).Msg("Failed to evaluate asset rule on update, will reconcile later")
		}
	}

	return s.repo.Get(ctx, id)
}

func (s *service) Delete(ctx context.Context, id string) error {
	if s.memberSvc != nil {
		if err := s.memberSvc.OnRuleDeleted(ctx, id); err != nil {
			log.Warn().Err(err).Str("rule_id", id).Msg("Failed to clean up asset rule memberships on delete")
		}
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, offset, limit)
}

func (s *service) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = DefaultLimit
	} else if filter.Limit > MaxLimit {
		filter.Limit = MaxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.Search(ctx, filter)
}

func (s *service) PreviewRule(ctx context.Context, input RulePreviewInput, limit int) (*RulePreview, error) {
	if err := enrichment.ValidateRule(&input); err != nil {
		return &RulePreview{
			AssetIDs:   []string{},
			AssetCount: 0,
			Errors:     []string{err.Error()},
		}, nil
	}

	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}

	assetIDs, err := s.evaluator.ExecuteRule(ctx, &input)
	if err != nil {
		return &RulePreview{
			AssetIDs:   []string{},
			AssetCount: 0,
			Errors:     []string{err.Error()},
		}, nil
	}

	total := len(assetIDs)
	if limit > 0 && limit < len(assetIDs) {
		assetIDs = assetIDs[:limit]
	}

	return &RulePreview{
		AssetIDs:   assetIDs,
		AssetCount: total,
	}, nil
}

func (s *service) GetRuleAssets(ctx context.Context, ruleID string, limit, offset int) ([]string, int, error) {
	if _, err := s.repo.Get(ctx, ruleID); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}

	return s.memberRepo.GetMembershipAssetIDs(ctx, ruleID, limit, offset)
}

func (s *service) GetEnrichedLinks(ctx context.Context, assetID string) ([]EnrichedExternalLink, error) {
	return s.repo.GetRuleManagedLinks(ctx, assetID)
}
