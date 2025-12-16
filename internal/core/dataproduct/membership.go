package dataproduct

import (
	"context"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/query"
	"github.com/marmotdata/marmot/internal/worker"
	"github.com/rs/zerolog/log"
)

// MembershipService handles the evaluation of data product rules and
// maintains the precomputed membership table.
type MembershipService struct {
	repo        Repository
	memberRepo  MembershipRepository
	assetGetter AssetGetter

	// Background processing
	workerPool *worker.Pool
	batcher    *worker.BatchProcessor[*asset.Asset]

	ctx    context.Context
	cancel context.CancelFunc
}

// AssetGetter provides read access to assets.
type AssetGetter interface {
	Get(ctx context.Context, id string) (*asset.Asset, error)
}

// MembershipConfig configures the membership service.
type MembershipConfig struct {
	// MaxWorkers for rule evaluation. Default: 5.
	MaxWorkers int
	// BatchSize for membership updates. Default: 50.
	BatchSize int
	// FlushInterval for batched updates. Default: 500ms.
	FlushInterval time.Duration
}

// NewMembershipService creates a new membership service.
func NewMembershipService(
	repo Repository,
	memberRepo MembershipRepository,
	assetGetter AssetGetter,
	config *MembershipConfig,
) *MembershipService {
	if config == nil {
		config = &MembershipConfig{}
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 5
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 500 * time.Millisecond
	}

	svc := &MembershipService{
		repo:        repo,
		memberRepo:  memberRepo,
		assetGetter: assetGetter,
	}

	// Create worker pool for rule evaluation
	svc.workerPool = worker.NewPool(worker.PoolConfig{
		Name:       "membership-evaluator",
		MaxWorkers: config.MaxWorkers,
		QueueSize:  200,
		OnJobComplete: func(job worker.Job, err error, duration time.Duration) {
			if err != nil {
				log.Error().
					Str("job_id", job.ID()).
					Err(err).
					Dur("duration", duration).
					Msg("Membership evaluation job failed")
			}
		},
	})

	// Create batch processor for membership updates
	svc.batcher = worker.NewBatchProcessor(worker.BatchConfig[*asset.Asset]{
		Name:          "membership-batcher",
		BatchSize:     config.BatchSize,
		FlushInterval: config.FlushInterval,
		ProcessFn:     svc.processBatch,
	})

	return svc
}

// Start begins background processing.
func (s *MembershipService) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.workerPool.Start(ctx)
	s.batcher.Start(ctx)

	log.Info().Msg("Membership service started")
}

// Stop gracefully shuts down the service.
func (s *MembershipService) Stop() {
	log.Info().Msg("Stopping membership service...")

	if s.cancel != nil {
		s.cancel()
	}

	s.batcher.Stop()
	s.workerPool.Stop()

	log.Info().Msg("Membership service stopped")
}

// OnAssetCreated is called when a new asset is created.
// It queues the asset for membership evaluation.
func (s *MembershipService) OnAssetCreated(ctx context.Context, ast *asset.Asset) {
	if ast.IsStub {
		return
	}
	s.batcher.Add(ast)
}

// OnAssetDeleted is called when an asset is deleted.
// It removes all memberships for this asset.
func (s *MembershipService) OnAssetDeleted(ctx context.Context, assetID string) error {
	return s.memberRepo.DeleteMembershipsByAsset(ctx, assetID)
}

// OnRuleCreated is called when a new rule is created.
// It extracts targets and evaluates the rule against all assets.
func (s *MembershipService) OnRuleCreated(ctx context.Context, rule *Rule) error {
	// Extract and save targets
	targets := ExtractRuleTargets(rule)
	if err := s.memberRepo.SaveRuleTargets(ctx, rule.ID, rule.DataProductID, targets); err != nil {
		return err
	}

	// Queue full evaluation of this rule
	if rule.IsEnabled {
		s.workerPool.Submit(&ruleEvaluationJob{
			svc:    s,
			ruleID: rule.ID,
		})
	}

	return nil
}

// OnRuleUpdated is called when a rule is updated.
// It re-extracts targets and re-evaluates the rule.
func (s *MembershipService) OnRuleUpdated(ctx context.Context, rule *Rule) error {
	// Delete old memberships for this rule
	if err := s.memberRepo.DeleteMembershipsByRule(ctx, rule.ID); err != nil {
		return err
	}

	// Re-extract targets
	targets := ExtractRuleTargets(rule)
	if err := s.memberRepo.SaveRuleTargets(ctx, rule.ID, rule.DataProductID, targets); err != nil {
		return err
	}

	// Queue full evaluation if enabled
	if rule.IsEnabled {
		s.workerPool.Submit(&ruleEvaluationJob{
			svc:    s,
			ruleID: rule.ID,
		})
	}

	return nil
}

// OnRuleDeleted is called when a rule is deleted.
// Memberships and targets are cascade-deleted by the database.
func (s *MembershipService) OnRuleDeleted(ctx context.Context, ruleID string) error {
	// Database handles cascade delete, but we can explicitly clean up
	return s.memberRepo.DeleteMembershipsByRule(ctx, ruleID)
}

// processBatch handles a batch of assets for membership evaluation.
func (s *MembershipService) processBatch(ctx context.Context, assets []*asset.Asset) error {
	for _, ast := range assets {
		if err := s.evaluateAsset(ctx, ast); err != nil {
			log.Error().
				Err(err).
				Str("asset_id", ast.ID).
				Msg("Failed to evaluate asset for memberships")
			// Continue with other assets
		}
	}
	return nil
}

// evaluateAsset finds candidate rules and checks if the asset matches.
func (s *MembershipService) evaluateAsset(ctx context.Context, ast *asset.Asset) error {
	// Extract asset signature for candidate lookup
	sig := AssetSignature{
		ID:           ast.ID,
		Type:         ast.Type,
		Providers:    ast.Providers,
		Tags:         ast.Tags,
		MetadataKeys: extractMetadataKeys(ast.Metadata),
	}

	// Find candidate rules
	candidates, err := s.memberRepo.FindCandidateRules(ctx, sig)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		return nil
	}

	// Evaluate each candidate rule against this single asset
	var memberships []Membership
	for _, candidate := range candidates {
		matches, err := s.evaluateRuleForAsset(ctx, candidate, ast)
		if err != nil {
			log.Debug().
				Err(err).
				Str("rule_id", candidate.RuleID).
				Str("asset_id", ast.ID).
				Msg("Rule evaluation failed")
			continue
		}

		if matches {
			memberships = append(memberships, Membership{
				DataProductID: candidate.DataProductID,
				AssetID:       ast.ID,
				Source:        SourceRule,
				RuleID:        &candidate.RuleID,
			})
		}
	}

	// Batch insert memberships
	if len(memberships) > 0 {
		return s.memberRepo.CreateMemberships(ctx, memberships)
	}

	return nil
}

// evaluateRuleForAsset checks if a single asset matches a rule.
func (s *MembershipService) evaluateRuleForAsset(ctx context.Context, candidate CandidateRule, ast *asset.Asset) (bool, error) {
	// Get the full rule
	rule, err := s.repo.GetRule(ctx, candidate.RuleID)
	if err != nil {
		return false, err
	}

	if !rule.IsEnabled {
		return false, nil
	}

	// Evaluate based on rule type
	if rule.RuleType == RuleTypeMetadataMatch {
		return evaluateMetadataRuleInMemory(rule, ast), nil
	}

	// For query rules, check against the database with asset ID filter
	return s.memberRepo.EvaluateRuleForAsset(ctx, rule, ast.ID)
}

// EvaluateRule fully evaluates a rule against all assets.
// Used for initial rule evaluation and reconciliation.
func (s *MembershipService) EvaluateRule(ctx context.Context, ruleID string) error {
	rule, err := s.repo.GetRule(ctx, ruleID)
	if err != nil {
		return err
	}

	if !rule.IsEnabled {
		return nil
	}

	// Execute the rule to get matching asset IDs
	assetIDs, err := s.repo.ExecuteRule(ctx, rule)
	if err != nil {
		return err
	}

	memberships := make([]Membership, len(assetIDs))
	for i, assetID := range assetIDs {
		memberships[i] = Membership{
			DataProductID: rule.DataProductID,
			AssetID:       assetID,
			Source:        SourceRule,
			RuleID:        &rule.ID,
		}
	}

	if len(memberships) > 0 {
		return s.memberRepo.CreateMemberships(ctx, memberships)
	}

	return nil
}

// ReconcileAll re-evaluates all rules and updates memberships.
// This is used for periodic reconciliation to fix any drift.
func (s *MembershipService) ReconcileAll(ctx context.Context) error {
	log.Info().Msg("Starting full membership reconciliation")
	start := time.Now()

	products, err := s.repo.List(ctx, 0, MaxReconcileProducts)
	if err != nil {
		return err
	}

	var totalRules int
	for _, product := range products.DataProducts {
		rules, err := s.repo.GetRules(ctx, product.ID)
		if err != nil {
			log.Error().Err(err).Str("product_id", product.ID).Msg("Failed to get rules for product")
			continue
		}

		for _, rule := range rules {
			if !rule.IsEnabled {
				continue
			}

			totalRules++

			// Delete existing rule memberships
			if err := s.memberRepo.DeleteMembershipsByRule(ctx, rule.ID); err != nil {
				log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to delete rule memberships")
				continue
			}

			// Re-evaluate
			if err := s.EvaluateRule(ctx, rule.ID); err != nil {
				log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to evaluate rule")
			}
		}
	}

	log.Info().
		Int("products", len(products.DataProducts)).
		Int("rules_evaluated", totalRules).
		Dur("duration", time.Since(start)).
		Msg("Full membership reconciliation completed")

	return nil
}

// ruleEvaluationJob implements worker.Job for evaluating a single rule.
type ruleEvaluationJob struct {
	svc    *MembershipService
	ruleID string
}

func (j *ruleEvaluationJob) ID() string {
	return "rule-eval:" + j.ruleID
}

func (j *ruleEvaluationJob) Execute(ctx context.Context) error {
	return j.svc.EvaluateRule(ctx, j.ruleID)
}

// evaluateMetadataRuleInMemory checks if an asset matches a metadata rule without DB access.
func evaluateMetadataRuleInMemory(rule *Rule, ast *asset.Asset) bool {
	if rule.MetadataField == nil || rule.PatternType == nil || rule.PatternValue == nil {
		return false
	}

	// Get the value from the asset's metadata
	value := getNestedMetadataValue(ast.Metadata, *rule.MetadataField)
	if value == nil {
		return false
	}

	strValue, ok := value.(string)
	if !ok {
		return false
	}

	switch *rule.PatternType {
	case PatternTypeExact:
		return strValue == *rule.PatternValue
	case PatternTypePrefix:
		return strings.HasPrefix(strValue, *rule.PatternValue)
	case PatternTypeWildcard:
		return matchWildcard(*rule.PatternValue, strValue)
	case PatternTypeRegex:
		// For regex, we'd need to compile - skip for in-memory evaluation
		// and let the DB handle it
		return false
	}

	return false
}

// getNestedMetadataValue extracts a value from nested metadata using dot notation.
func getNestedMetadataValue(metadata map[string]interface{}, field string) interface{} {
	parts := strings.Split(field, ".")
	var current interface{} = metadata

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}

// matchWildcard performs simple wildcard matching (* = any characters).
func matchWildcard(pattern, value string) bool {
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)

	// Simple implementation - convert * to regex-like matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}

	// Check prefix
	if parts[0] != "" && !strings.HasPrefix(value, parts[0]) {
		return false
	}

	// Check suffix
	lastPart := parts[len(parts)-1]
	if lastPart != "" && !strings.HasSuffix(value, lastPart) {
		return false
	}

	// Check middle parts exist in order
	pos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(value[pos:], parts[i])
		if idx == -1 {
			return false
		}
		pos += idx + len(parts[i])
	}

	return true
}

// extractMetadataKeys extracts the top-level keys from metadata.
func extractMetadataKeys(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	return keys
}

// ExtractRuleTargets analyzes a rule and extracts what it's targeting.
func ExtractRuleTargets(rule *Rule) []RuleTarget {
	var targets []RuleTarget

	if rule.RuleType == RuleTypeMetadataMatch {
		if rule.MetadataField != nil {
			parts := strings.Split(*rule.MetadataField, ".")
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeMetadataKey,
				TargetValue: parts[0],
			})
		}
		return targets
	}

	if rule.QueryExpression == nil {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
		return targets
	}

	parser := query.NewParser()
	parsed, err := parser.Parse(*rule.QueryExpression)
	if err != nil {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
		return targets
	}

	targets = extractTargetsFromQuery(parsed)

	if len(targets) == 0 {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
	}

	return targets
}

// extractTargetsFromQuery walks a parsed query and extracts targetable fields.
func extractTargetsFromQuery(q *query.Query) []RuleTarget {
	var targets []RuleTarget

	if q.Bool == nil {
		// Free text search - complex query
		return targets
	}

	// Process Must filters
	for _, filter := range q.Bool.Must {
		targets = append(targets, extractTargetsFromFilter(filter)...)
	}

	// Process Should filters (OR conditions)
	for _, filter := range q.Bool.Should {
		targets = append(targets, extractTargetsFromFilter(filter)...)
	}

	return targets
}

func extractTargetsFromFilter(filter query.Filter) []RuleTarget {
	var targets []RuleTarget

	switch filter.FieldType {
	case query.FieldAssetType:
		if v, ok := filter.Value.(string); ok {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeAssetType,
				TargetValue: v,
			})
		}
	case query.FieldProvider:
		if v, ok := filter.Value.(string); ok {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeProvider,
				TargetValue: v,
			})
		}
	case query.FieldMetadata:
		if len(filter.Field) > 0 {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeMetadataKey,
				TargetValue: filter.Field[0],
			})
		}
	case query.FieldName:
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
	}

	if nested, ok := filter.Value.(*query.BooleanQuery); ok {
		for _, f := range nested.Must {
			targets = append(targets, extractTargetsFromFilter(f)...)
		}
		for _, f := range nested.Should {
			targets = append(targets, extractTargetsFromFilter(f)...)
		}
	}

	return targets
}
