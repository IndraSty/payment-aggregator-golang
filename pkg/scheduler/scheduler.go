package scheduler

import (
	"context"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler wraps robfig/cron with graceful shutdown support.
type Scheduler struct {
	cron        *cron.Cron
	reconcileUC domain.ReconcileUsecase
}

// New creates a new Scheduler instance.
func New(reconcileUC domain.ReconcileUsecase) *Scheduler {
	c := cron.New(
		cron.WithLogger(cronLogger{}),
	)

	return &Scheduler{
		cron:        c,
		reconcileUC: reconcileUC,
	}
}

// Register adds all cron jobs to the scheduler.
func (s *Scheduler) Register(schedule string) error {
	// Reconciliation job — runs daily at 2AM by default (configurable via .env)
	_, err := s.cron.AddFunc(schedule, func() {
		log.Info().Msg("Cron: starting scheduled reconciliation")

		ctx := context.Background()
		reports, err := s.reconcileUC.RunReconciliation(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Cron: reconciliation failed")
			return
		}

		totalDiscrepancies := 0
		for _, r := range reports {
			totalDiscrepancies += r.Discrepancies
		}

		log.Info().
			Int("providers", len(reports)).
			Int("total_discrepancies", totalDiscrepancies).
			Msg("Cron: reconciliation completed")
	})

	if err != nil {
		return err
	}

	log.Info().Str("schedule", schedule).Msg("Reconciliation cron job registered")
	return nil
}

// Start begins the scheduler in the background.
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Info().Msg("Scheduler started")
}

// Stop gracefully stops the scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() {
	log.Info().Msg("Stopping scheduler...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Info().Msg("Scheduler stopped")
}

// cronLogger adapts zerolog to robfig/cron's Logger interface.
type cronLogger struct{}

func (l cronLogger) Info(msg string, keysAndValues ...any) {
	log.Info().Fields(keysAndValues).Msg("Cron: " + msg)
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...any) {
	log.Error().Err(err).Fields(keysAndValues).Msg("Cron: " + msg)
}
