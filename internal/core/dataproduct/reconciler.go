package dataproduct

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	DefaultReconcileInterval = 30 * time.Minute
)

// Reconciler periodically re-evaluates all rules to fix any membership drift.
type Reconciler struct {
	membershipSvc *MembershipService
	interval      time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ReconcilerConfig configures the reconciler.
type ReconcilerConfig struct {
	// Interval between full reconciliation runs. Default: 30 minutes.
	Interval time.Duration
}

// NewReconciler creates a new reconciler.
func NewReconciler(membershipSvc *MembershipService, config *ReconcilerConfig) *Reconciler {
	if config == nil {
		config = &ReconcilerConfig{}
	}
	if config.Interval <= 0 {
		config.Interval = DefaultReconcileInterval
	}

	return &Reconciler{
		membershipSvc: membershipSvc,
		interval:      config.Interval,
	}
}

// Start begins the periodic reconciliation loop.
func (r *Reconciler) Start(ctx context.Context) {
	r.ctx, r.cancel = context.WithCancel(ctx)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.reconcileLoop()
	}()

	log.Info().
		Dur("interval", r.interval).
		Msg("Data product membership reconciler started")
}

// Stop gracefully shuts down the reconciler.
func (r *Reconciler) Stop() {
	log.Info().Msg("Stopping membership reconciler...")

	if r.cancel != nil {
		r.cancel()
	}

	r.wg.Wait()
	log.Info().Msg("Membership reconciler stopped")
}

func (r *Reconciler) reconcileLoop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Do an initial reconciliation on startup after a short delay
	// This allows the system to fully initialize
	select {
	case <-r.ctx.Done():
		return
	case <-time.After(30 * time.Second):
		r.runReconciliation()
	}

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.runReconciliation()
		}
	}
}

func (r *Reconciler) runReconciliation() {
	log.Info().Msg("Starting scheduled membership reconciliation")

	if err := r.membershipSvc.ReconcileAll(r.ctx); err != nil {
		log.Error().Err(err).Msg("Membership reconciliation failed")
	}
}

// RunNow triggers an immediate reconciliation (for manual/API use).
func (r *Reconciler) RunNow(ctx context.Context) error {
	log.Info().Msg("Running manual membership reconciliation")
	return r.membershipSvc.ReconcileAll(ctx)
}
