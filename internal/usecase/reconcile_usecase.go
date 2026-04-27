package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/metrics"
	"github.com/IndraSty/payment-aggregator-golang/pkg/validator"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type reconcileUsecase struct {
	txRepo        domain.TransactionRepository
	auditRepo     domain.AuditRepository
	reconcileRepo domain.ReconciliationRepository
	router        domain.ProviderRouter
	lookbackHours int
}

// NewReconcileUsecase creates a new reconciliation usecase.
func NewReconcileUsecase(
	txRepo domain.TransactionRepository,
	auditRepo domain.AuditRepository,
	reconcileRepo domain.ReconciliationRepository,
	router domain.ProviderRouter,
	lookbackHours int,
) domain.ReconcileUsecase {
	return &reconcileUsecase{
		txRepo:        txRepo,
		auditRepo:     auditRepo,
		reconcileRepo: reconcileRepo,
		router:        router,
		lookbackHours: lookbackHours,
	}
}

// RunReconciliation runs reconciliation for all providers concurrently.
func (u *reconcileUsecase) RunReconciliation(ctx context.Context) ([]*domain.ReconciliationReport, error) {
	providers := []domain.PaymentProvider{
		domain.ProviderMidtrans,
		domain.ProviderXendit,
		domain.ProviderStripe,
	}

	var reports []*domain.ReconciliationReport
	for _, p := range providers {
		report, err := u.reconcileProvider(ctx, p)
		if err != nil {
			// Log but continue with other providers
			log.Error().Err(err).Str("provider", string(p)).Msg("Reconciliation failed for provider")
			continue
		}
		reports = append(reports, report)
	}

	log.Info().Int("providers_reconciled", len(reports)).Msg("Reconciliation run completed")
	return reports, nil
}

// reconcileProvider reconciles pending transactions for a single provider.
func (u *reconcileUsecase) reconcileProvider(ctx context.Context, providerName domain.PaymentProvider) (*domain.ReconciliationReport, error) {
	logger := log.With().Str("provider", string(providerName)).Logger()

	// Step 1: Create report with status "running"
	report := &domain.ReconciliationReport{
		ID:        uuid.New(),
		Provider:  providerName,
		Date:      time.Now().UTC().Truncate(24 * time.Hour),
		Status:    domain.ReconcileRunning,
		CreatedAt: time.Now(),
	}
	if err := u.reconcileRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("create reconciliation report: %w", err)
	}

	// Step 2: Fetch all pending transactions within lookback window
	since := time.Now().Add(-time.Duration(u.lookbackHours) * time.Hour)
	pendingTxs, err := u.txRepo.GetPendingByProvider(ctx, providerName, since)
	if err != nil {
		report.Status = domain.ReconcileFailed
		_ = u.reconcileRepo.Update(ctx, report)
		return report, fmt.Errorf("fetch pending transactions: %w", err)
	}

	// Step 3: Get provider client
	providerClient, err := u.router.GetProvider(providerName)
	if err != nil {
		report.Status = domain.ReconcileFailed
		_ = u.reconcileRepo.Update(ctx, report)
		return report, fmt.Errorf("get provider client: %w", err)
	}

	// Step 4: Check each pending transaction against the provider
	discrepancies := 0
	for _, tx := range pendingTxs {
		providerStatus, err := providerClient.GetStatus(ctx, tx.ExternalID)
		if err != nil {
			logger.Warn().
				Err(err).
				Str("transaction_id", tx.ID.String()).
				Str("external_id", tx.ExternalID).
				Msg("Failed to fetch status from provider during reconciliation")
			discrepancies++
			continue
		}

		// Step 5: If provider status differs from local, update local
		if providerStatus.Status != tx.Status && tx.CanTransitionTo(providerStatus.Status) {
			prevStatus := tx.Status

			if err := u.txRepo.UpdateStatus(ctx, tx.ID, providerStatus.Status); err != nil {
				logger.Error().
					Err(err).
					Str("transaction_id", tx.ID.String()).
					Msg("Failed to update status during reconciliation")
				discrepancies++
				continue
			}

			// Audit log the reconciliation-driven status change
			auditEntry := &domain.TransactionAuditLog{
				ID:            uuid.New(),
				TransactionID: tx.ID,
				FromStatus:    &prevStatus,
				ToStatus:      providerStatus.Status,
				Source:        "reconciliation",
				RawPayload:    providerStatus.RawResponse,
				CreatedAt:     time.Now(),
			}
			if err := u.auditRepo.Create(ctx, auditEntry); err != nil {
				logger.Error().Err(err).Str("transaction_id", tx.ID.String()).Msg("Failed to write reconciliation audit log")
			}

			metrics.RecordReconciliationDiscrepancy(string(providerName))
			discrepancies++
			logger.Info().
				Str("transaction_id", tx.ID.String()).
				Str("from_status", string(prevStatus)).
				Str("to_status", string(providerStatus.Status)).
				Msg("Reconciliation corrected transaction status")
		}
	}

	// Step 6: Update report with results
	report.TotalChecked = len(pendingTxs)
	report.Discrepancies = discrepancies
	report.Status = domain.ReconcileCompleted

	if err := u.reconcileRepo.Update(ctx, report); err != nil {
		logger.Error().Err(err).Msg("Failed to update reconciliation report")
	}

	logger.Info().
		Int("total_checked", report.TotalChecked).
		Int("discrepancies", report.Discrepancies).
		Msg("Provider reconciliation completed")

	return report, nil
}

func (u *reconcileUsecase) GetReport(ctx context.Context, id string) (*domain.ReconciliationReport, error) {
	reportID, err := uuid.Parse(id)
	if err != nil {
		return nil, domain.ErrNotFound("reconciliation report not found")
	}

	report, err := u.reconcileRepo.GetByID(ctx, reportID)
	if err != nil {
		return nil, domain.ErrNotFound("reconciliation report not found")
	}

	return report, nil
}

func (u *reconcileUsecase) ListReports(ctx context.Context, filter domain.ReconciliationFilter) ([]*domain.ReconciliationReport, int64, error) {
	filter.Page, filter.Limit = validator.ValidatePagination(filter.Page, filter.Limit)

	reports, total, err := u.reconcileRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, domain.ErrInternal(err)
	}

	return reports, total, nil
}
