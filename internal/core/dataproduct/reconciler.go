package dataproduct

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/background"
	"github.com/rs/zerolog/log"
)

const (
	DefaultReconcileInterval = 30 * time.Minute
)

// Reconciler periodically re-evaluates all rules to fix any membership drift.
type Reconciler struct {
	membershipSvc *MembershipService
	task          *background.SingletonTask
}

// ReconcilerConfig configures the reconciler.
type ReconcilerConfig struct {
	// Interval between full reconciliation runs. Default: 30 minutes.
	Interval time.Duration
	// DB is the PostgreSQL connection pool for singleton coordination.
	DB *pgxpool.Pool
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
		Name:         "dataproduct-reconcile",
		DB:           config.DB,
		Interval:     config.Interval,
		InitialDelay: 30 * time.Second,
		TaskFn: func(ctx context.Context) error {
			log.Info().Msg("Starting scheduled membership reconciliation")
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

// RunNow triggers an immediate reconciliation (for manual/API use).
func (r *Reconciler) RunNow(ctx context.Context) error {
	log.Info().Msg("Running manual membership reconciliation")
	return r.membershipSvc.ReconcileAll(ctx)
}
