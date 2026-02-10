package assetrule

import (
	"context"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/marmotdata/marmot/internal/worker"
	"github.com/rs/zerolog/log"
)

// MembershipService evaluates asset rules and maintains rule-to-asset memberships.
type MembershipService struct {
	repo       Repository
	memberRepo MembershipRepository
	evaluator  *enrichment.Evaluator

	workerPool *worker.Pool
	batcher    *worker.BatchProcessor[*asset.Asset]

	ctx    context.Context
	cancel context.CancelFunc
}

type MembershipConfig struct {
	MaxWorkers    int
	BatchSize     int
	FlushInterval time.Duration
}

func NewMembershipService(
	repo Repository,
	memberRepo MembershipRepository,
	evaluator *enrichment.Evaluator,
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
		repo:       repo,
		memberRepo: memberRepo,
		evaluator:  evaluator,
	}

	svc.workerPool = worker.NewPool(worker.PoolConfig{
		Name:       "assetrule-membership-evaluator",
		MaxWorkers: config.MaxWorkers,
		QueueSize:  200,
		OnJobComplete: func(job worker.Job, err error, duration time.Duration) {
			if err != nil {
				log.Error().
					Str("job_id", job.ID()).
					Err(err).
					Dur("duration", duration).
					Msg("Asset rule evaluation job failed")
			}
		},
	})

	svc.batcher = worker.NewBatchProcessor(worker.BatchConfig[*asset.Asset]{
		Name:          "assetrule-membership-batcher",
		BatchSize:     config.BatchSize,
		FlushInterval: config.FlushInterval,
		ProcessFn:     svc.processBatch,
	})

	return svc
}

func (s *MembershipService) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.workerPool.Start(ctx)
	s.batcher.Start(ctx)
	log.Info().Msg("Asset rule membership service started")
}

func (s *MembershipService) Stop() {
	log.Info().Msg("Stopping asset rule membership service...")
	if s.cancel != nil {
		s.cancel()
	}
	s.batcher.Stop()
	s.workerPool.Stop()
	log.Info().Msg("Asset rule membership service stopped")
}

func (s *MembershipService) OnAssetCreated(ctx context.Context, ast *asset.Asset) {
	if ast.IsStub {
		return
	}
	s.batcher.Add(ast)
}

func (s *MembershipService) OnAssetDeleted(ctx context.Context, assetID string) error {
	if err := s.memberRepo.DeleteMembershipsByAsset(ctx, assetID); err != nil {
		return err
	}
	return s.memberRepo.DeleteTermMembershipsByAsset(ctx, assetID)
}

func (s *MembershipService) OnRuleCreated(ctx context.Context, rule *AssetRule) error {
	targets := enrichment.ExtractRuleTargets(rule)
	if err := s.memberRepo.SaveRuleTargets(ctx, rule.ID, targets); err != nil {
		return err
	}

	if rule.IsEnabled {
		s.workerPool.Submit(&ruleEvaluationJob{
			svc:    s,
			ruleID: rule.ID,
		})
	}

	return nil
}

func (s *MembershipService) OnRuleUpdated(ctx context.Context, rule *AssetRule) error {
	if err := s.memberRepo.DeleteMembershipsByRule(ctx, rule.ID); err != nil {
		return err
	}
	if err := s.memberRepo.DeleteTermMembershipsByRule(ctx, rule.ID); err != nil {
		return err
	}

	targets := enrichment.ExtractRuleTargets(rule)
	if err := s.memberRepo.SaveRuleTargets(ctx, rule.ID, targets); err != nil {
		return err
	}

	if rule.IsEnabled {
		s.workerPool.Submit(&ruleEvaluationJob{
			svc:    s,
			ruleID: rule.ID,
		})
	}

	return nil
}

func (s *MembershipService) OnRuleDeleted(ctx context.Context, ruleID string) error {
	if err := s.memberRepo.DeleteMembershipsByRule(ctx, ruleID); err != nil {
		return err
	}
	return s.memberRepo.DeleteTermMembershipsByRule(ctx, ruleID)
}

func (s *MembershipService) EvaluateRule(ctx context.Context, ruleID string) error {
	rule, err := s.repo.Get(ctx, ruleID)
	if err != nil {
		return err
	}

	if !rule.IsEnabled {
		return nil
	}

	assetIDs, err := s.evaluator.ExecuteRule(ctx, rule)
	if err != nil {
		return err
	}

	if len(assetIDs) == 0 {
		return nil
	}

	if err := s.memberRepo.CreateMemberships(ctx, ruleID, assetIDs); err != nil {
		return err
	}
	if len(rule.TermIDs) > 0 {
		if err := s.memberRepo.CreateTermMemberships(ctx, ruleID, rule.TermIDs, assetIDs); err != nil {
			return err
		}
	}

	return nil
}

// ReconcileAll re-evaluates all enabled rules using differential reconciliation,
// skipping rules whose config hash hasn't changed since the last run.
func (s *MembershipService) ReconcileAll(ctx context.Context) error {
	log.Info().Msg("Starting asset rule membership reconciliation")
	start := time.Now()

	rules, err := s.repo.GetAllEnabled(ctx)
	if err != nil {
		return err
	}

	var evaluated, skipped int
	for _, rule := range rules {
		hash := rule.ComputeHash()

		if rule.ReconciliationHash != nil && *rule.ReconciliationHash == hash {
			skipped++
			continue
		}

		evaluated++

		newAssetIDs, err := s.evaluator.ExecuteRule(ctx, rule)
		if err != nil {
			log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to execute asset rule")
			continue
		}

		existing, err := s.memberRepo.GetExistingMembershipAssetIDs(ctx, rule.ID)
		if err != nil {
			log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to get existing memberships")
			continue
		}

		newSet := make(map[string]struct{}, len(newAssetIDs))
		var toInsert []string
		for _, id := range newAssetIDs {
			newSet[id] = struct{}{}
			if _, ok := existing[id]; !ok {
				toInsert = append(toInsert, id)
			}
		}

		var toDelete []string
		for id := range existing {
			if _, ok := newSet[id]; !ok {
				toDelete = append(toDelete, id)
			}
		}

		if len(toDelete) > 0 {
			if err := s.memberRepo.DeleteMembershipsBatch(ctx, rule.ID, toDelete); err != nil {
				log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to delete stale memberships")
			}
			if len(rule.TermIDs) > 0 {
				if err := s.memberRepo.DeleteTermMembershipsBatch(ctx, rule.ID, toDelete); err != nil {
					log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to delete stale term memberships")
				}
			}
		}

		if len(toInsert) > 0 {
			if err := s.memberRepo.CreateMemberships(ctx, rule.ID, toInsert); err != nil {
				log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to insert new memberships")
			}
			if len(rule.TermIDs) > 0 {
				if err := s.memberRepo.CreateTermMemberships(ctx, rule.ID, rule.TermIDs, toInsert); err != nil {
					log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to insert new term memberships")
				}
			}
		}

		if err := s.repo.UpdateReconciliationState(ctx, rule.ID, hash); err != nil {
			log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to update reconciliation state")
		}

		log.Debug().
			Str("rule_id", rule.ID).
			Int("inserted", len(toInsert)).
			Int("deleted", len(toDelete)).
			Msg("Asset rule reconciled")
	}

	log.Info().
		Int("total_rules", len(rules)).
		Int("evaluated", evaluated).
		Int("skipped", skipped).
		Dur("duration", time.Since(start)).
		Msg("Asset rule membership reconciliation completed")

	return nil
}

func (s *MembershipService) processBatch(ctx context.Context, assets []*asset.Asset) error {
	for _, ast := range assets {
		if err := s.evaluateAsset(ctx, ast); err != nil {
			log.Error().
				Err(err).
				Str("asset_id", ast.ID).
				Msg("Failed to evaluate asset for asset rule memberships")
		}
	}
	return nil
}

func (s *MembershipService) evaluateAsset(ctx context.Context, ast *asset.Asset) error {
	sig := enrichment.AssetSignature{
		ID:           ast.ID,
		Type:         ast.Type,
		Providers:    ast.Providers,
		Tags:         ast.Tags,
		MetadataKeys: enrichment.ExtractMetadataKeys(ast.Metadata),
	}

	candidates, err := s.memberRepo.FindCandidateRules(ctx, sig)
	if err != nil {
		return err
	}

	for _, candidate := range candidates {
		rule, err := s.repo.Get(ctx, candidate.RuleID)
		if err != nil {
			log.Debug().Err(err).Str("rule_id", candidate.RuleID).Msg("Failed to get rule")
			continue
		}

		if !rule.IsEnabled {
			continue
		}

		var matches bool
		if rule.RuleType == enrichment.RuleTypeMetadataMatch {
			matches = enrichment.EvaluateMetadataRuleInMemory(rule, ast.Metadata)
		} else {
			matches, err = s.evaluator.EvaluateRuleForAsset(ctx, rule, ast.ID)
			if err != nil {
				log.Debug().Err(err).Str("rule_id", rule.ID).Str("asset_id", ast.ID).Msg("Rule evaluation failed")
				continue
			}
		}

		if matches {
			if err := s.memberRepo.CreateMemberships(ctx, rule.ID, []string{ast.ID}); err != nil {
				log.Error().Err(err).Str("rule_id", rule.ID).Str("asset_id", ast.ID).Msg("Failed to create membership")
			}
			if len(rule.TermIDs) > 0 {
				if err := s.memberRepo.CreateTermMemberships(ctx, rule.ID, rule.TermIDs, []string{ast.ID}); err != nil {
					log.Error().Err(err).Str("rule_id", rule.ID).Str("asset_id", ast.ID).Msg("Failed to create term membership")
				}
			}
		}
	}

	return nil
}

type ruleEvaluationJob struct {
	svc    *MembershipService
	ruleID string
}

func (j *ruleEvaluationJob) ID() string {
	return "assetrule-eval:" + j.ruleID
}

func (j *ruleEvaluationJob) Execute(ctx context.Context) error {
	return j.svc.EvaluateRule(ctx, j.ruleID)
}
