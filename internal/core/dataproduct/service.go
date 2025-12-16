package dataproduct

import (
	"context"
	"fmt"
	"regexp"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/marmotdata/marmot/internal/query"
	"github.com/rs/zerolog/log"
)

const (
	DefaultLimit        = 50
	MaxLimit            = 100
	MaxRules            = 10
	MaxReconcileProducts = 10000
)

type Service interface {
	Create(ctx context.Context, input CreateInput) (*DataProduct, error)
	Get(ctx context.Context, id string) (*DataProduct, error)
	Update(ctx context.Context, id string, input UpdateInput) (*DataProduct, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, offset, limit int) (*ListResult, error)
	Search(ctx context.Context, filter SearchFilter) (*ListResult, error)

	AddAssets(ctx context.Context, dataProductID string, assetIDs []string, createdBy string) error
	RemoveAsset(ctx context.Context, dataProductID string, assetID string) error
	GetManualAssets(ctx context.Context, dataProductID string, limit, offset int) (*AssetsResult, error)

	CreateRule(ctx context.Context, dataProductID string, input RuleInput) (*Rule, error)
	UpdateRule(ctx context.Context, ruleID string, input RuleInput) (*Rule, error)
	DeleteRule(ctx context.Context, ruleID string) error
	GetRules(ctx context.Context, dataProductID string) ([]Rule, error)
	PreviewRule(ctx context.Context, input RuleInput, limit int) (*RulePreview, error)

	GetResolvedAssets(ctx context.Context, dataProductID string, limit, offset int) (*ResolvedAssets, error)
	GetDataProductsForAsset(ctx context.Context, assetID string) ([]*DataProduct, error)

	SetRuleObserver(observer RuleObserver)
}

// RuleObserver is notified when rules are created, updated, or deleted.
type RuleObserver interface {
	OnRuleCreated(ctx context.Context, rule *Rule) error
	OnRuleUpdated(ctx context.Context, rule *Rule) error
	OnRuleDeleted(ctx context.Context, ruleID string) error
}

type service struct {
	repo         Repository
	validator    *validator.Validate
	ruleObserver RuleObserver
}

func NewService(repo Repository) Service {
	return &service{
		repo:      repo,
		validator: validator.New(),
	}
}

func (s *service) SetRuleObserver(observer RuleObserver) {
	s.ruleObserver = observer
}

func (s *service) Create(ctx context.Context, input CreateInput) (*DataProduct, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	for _, rule := range input.Rules {
		if err := s.validateRule(rule); err != nil {
			return nil, err
		}
	}

	if len(input.Rules) > MaxRules {
		return nil, fmt.Errorf("%w: maximum %d rules allowed per data product", ErrInvalidInput, MaxRules)
	}

	now := time.Now().UTC()
	dp := &DataProduct{
		Name:          input.Name,
		Description:   input.Description,
		Documentation: input.Documentation,
		Metadata:      input.Metadata,
		Tags:          input.Tags,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, dp, input.Owners); err != nil {
		return nil, err
	}

	for _, ruleInput := range input.Rules {
		rule, err := s.repo.CreateRule(ctx, dp.ID, &ruleInput)
		if err != nil {
			return nil, fmt.Errorf("creating rule: %w", err)
		}
		if s.ruleObserver != nil {
			if err := s.ruleObserver.OnRuleCreated(ctx, rule); err != nil {
				log.Warn().Err(err).Str("rule_id", rule.ID).Msg("Failed to evaluate rule on create, will reconcile later")
			}
		}
	}

	return s.Get(ctx, dp.ID)
}

func (s *service) Get(ctx context.Context, id string) (*DataProduct, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, id string, input UpdateInput) (*DataProduct, error) {
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
	if input.Documentation != nil {
		existing.Documentation = input.Documentation
	}
	if input.Metadata != nil {
		existing.Metadata = input.Metadata
	}
	if input.Tags != nil {
		existing.Tags = input.Tags
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, existing, input.Owners); err != nil {
		return nil, err
	}

	return s.Get(ctx, id)
}

func (s *service) Delete(ctx context.Context, id string) error {
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
	if err := s.validator.Struct(filter); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

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

func (s *service) AddAssets(ctx context.Context, dataProductID string, assetIDs []string, createdBy string) error {
	if _, err := s.repo.Get(ctx, dataProductID); err != nil {
		return err
	}

	if len(assetIDs) == 0 {
		return fmt.Errorf("%w: at least one asset ID required", ErrInvalidInput)
	}

	return s.repo.AddAssets(ctx, dataProductID, assetIDs, createdBy)
}

func (s *service) RemoveAsset(ctx context.Context, dataProductID string, assetID string) error {
	if _, err := s.repo.Get(ctx, dataProductID); err != nil {
		return err
	}

	return s.repo.RemoveAsset(ctx, dataProductID, assetID)
}

func (s *service) GetManualAssets(ctx context.Context, dataProductID string, limit, offset int) (*AssetsResult, error) {
	if _, err := s.repo.Get(ctx, dataProductID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.GetManualAssets(ctx, dataProductID, limit, offset)
}

func (s *service) CreateRule(ctx context.Context, dataProductID string, input RuleInput) (*Rule, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if err := s.validateRule(input); err != nil {
		return nil, err
	}

	dp, err := s.repo.Get(ctx, dataProductID)
	if err != nil {
		return nil, err
	}

	if len(dp.Rules) >= MaxRules {
		return nil, fmt.Errorf("%w: maximum %d rules allowed per data product", ErrInvalidInput, MaxRules)
	}

	rule, err := s.repo.CreateRule(ctx, dataProductID, &input)
	if err != nil {
		return nil, err
	}

	if s.ruleObserver != nil {
		if err := s.ruleObserver.OnRuleCreated(ctx, rule); err != nil {
			log.Warn().Err(err).Str("rule_id", rule.ID).Msg("Failed to evaluate rule on create, will reconcile later")
		}
	}

	return rule, nil
}

func (s *service) UpdateRule(ctx context.Context, ruleID string, input RuleInput) (*Rule, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if err := s.validateRule(input); err != nil {
		return nil, err
	}

	rule, err := s.repo.UpdateRule(ctx, ruleID, &input)
	if err != nil {
		return nil, err
	}

	if s.ruleObserver != nil {
		if err := s.ruleObserver.OnRuleUpdated(ctx, rule); err != nil {
			log.Warn().Err(err).Str("rule_id", rule.ID).Msg("Failed to evaluate rule on update, will reconcile later")
		}
	}

	return rule, nil
}

func (s *service) DeleteRule(ctx context.Context, ruleID string) error {
	if s.ruleObserver != nil {
		if err := s.ruleObserver.OnRuleDeleted(ctx, ruleID); err != nil {
			log.Warn().Err(err).Str("rule_id", ruleID).Msg("Failed to clean up rule memberships on delete")
		}
	}

	return s.repo.DeleteRule(ctx, ruleID)
}

func (s *service) GetRules(ctx context.Context, dataProductID string) ([]Rule, error) {
	if _, err := s.repo.Get(ctx, dataProductID); err != nil {
		return nil, err
	}

	return s.repo.GetRules(ctx, dataProductID)
}

func (s *service) PreviewRule(ctx context.Context, input RuleInput, limit int) (*RulePreview, error) {
	if err := s.validateRule(input); err != nil {
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

	return s.repo.PreviewRule(ctx, &input, limit)
}

func (s *service) GetResolvedAssets(ctx context.Context, dataProductID string, limit, offset int) (*ResolvedAssets, error) {
	if _, err := s.repo.Get(ctx, dataProductID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.ResolveAssets(ctx, dataProductID, limit, offset)
}

func (s *service) GetDataProductsForAsset(ctx context.Context, assetID string) ([]*DataProduct, error) {
	return s.repo.GetDataProductsForAsset(ctx, assetID)
}

func (s *service) validateRule(input RuleInput) error {
	if input.RuleType == RuleTypeQuery {
		if input.QueryExpression == nil || *input.QueryExpression == "" {
			return fmt.Errorf("%w: query_expression required for query rule type", ErrInvalidInput)
		}

		parser := query.NewParser()
		if _, err := parser.Parse(*input.QueryExpression); err != nil {
			return fmt.Errorf("%w: invalid query syntax: %v", ErrInvalidInput, err)
		}
	} else if input.RuleType == RuleTypeMetadataMatch {
		if input.MetadataField == nil || *input.MetadataField == "" {
			return fmt.Errorf("%w: metadata_field required for metadata_match rule type", ErrInvalidInput)
		}
		if input.PatternType == nil || *input.PatternType == "" {
			return fmt.Errorf("%w: pattern_type required for metadata_match rule type", ErrInvalidInput)
		}
		if input.PatternValue == nil || *input.PatternValue == "" {
			return fmt.Errorf("%w: pattern_value required for metadata_match rule type", ErrInvalidInput)
		}

		if *input.PatternType == PatternTypeRegex {
			if _, err := regexp.Compile(*input.PatternValue); err != nil {
				return fmt.Errorf("%w: invalid regex pattern: %v", ErrInvalidInput, err)
			}
		}
	} else {
		return fmt.Errorf("%w: invalid rule_type", ErrInvalidInput)
	}

	return nil
}
