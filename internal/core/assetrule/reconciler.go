package assetrule

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/background"
	"github.com/rs/zerolog/log"
)

const DefaultReconcileInterval = 30 * time.Minute

// Reconciler periodically re-evaluates all asset rules using differential reconciliation.
type Reconciler struct {
	membershipSvc *MembershipService
	task          *background.SingletonTask
}

// ReconcilerConfig configures the reconciler.
type ReconcilerConfig struct {
	Interval time.Duration
	DB       *pgxpool.Pool
}

// NewReconciler creates a new reconciler.
func NewReconciler(membershipSvc *MembershipService, config *ReconcilerConfig) *Reconciler {
	if config == nil {
		config = &ReconcilerConfig{}
	}
	if config.Interval <= 0 {
		config.Interval = DefaultReconcileInterval
	}

	r := &Reconciler{
		membershipSvc: membershipSvc,
	}

	r.task = background.NewSingletonTask(background.SingletonConfig{
		Name:         "assetrule-reconcile",
		DB:           config.DB,
		Interval:     config.Interval,
		InitialDelay: 45 * time.Second,
		TaskFn: func(ctx context.Context) error {
			log.Info().Msg("Starting scheduled asset rule reconciliation")
			return membershipSvc.ReconcileAll(ctx)
		},
	})

	return r
}

// Start begins the periodic reconciliation loop.
func (r *Reconciler) Start(ctx context.Context) {
	r.task.Start(ctx)
}

// Stop gracefully shuts down the reconciler.
func (r *Reconciler) Stop() {
	r.task.Stop()
}
