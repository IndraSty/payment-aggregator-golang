package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/metrics"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type webhookUsecase struct {
	txRepo      domain.TransactionRepository
	auditRepo   domain.AuditRepository
	webhookRepo domain.WebhookRepository
	router      domain.ProviderRouter
}

// NewWebhookUsecase creates a new webhook usecase.
func NewWebhookUsecase(
	txRepo domain.TransactionRepository,
	auditRepo domain.AuditRepository,
	webhookRepo domain.WebhookRepository,
	router domain.ProviderRouter,
) domain.WebhookUsecase {
	return &webhookUsecase{
		txRepo:      txRepo,
		auditRepo:   auditRepo,
		webhookRepo: webhookRepo,
		router:      router,
	}
}

func (u *webhookUsecase) ProcessMidtrans(ctx context.Context, payload []byte, headers map[string]string) error {
	return u.process(ctx, domain.ProviderMidtrans, payload, headers)
}

func (u *webhookUsecase) ProcessXendit(ctx context.Context, payload []byte, headers map[string]string) error {
	return u.process(ctx, domain.ProviderXendit, payload, headers)
}

func (u *webhookUsecase) ProcessStripe(ctx context.Context, payload []byte, headers map[string]string) error {
	return u.process(ctx, domain.ProviderStripe, payload, headers)
}

// process is the unified webhook processing pipeline for all providers.
func (u *webhookUsecase) process(ctx context.Context, providerName domain.PaymentProvider, payload []byte, headers map[string]string) error {
	logger := log.With().Str("provider", string(providerName)).Logger()

	// Step 1: Get the provider client to parse and verify the webhook
	providerClient, err := u.router.GetProvider(providerName)
	if err != nil {
		return domain.ErrInternal(fmt.Errorf("get provider: %w", err))
	}

	// Step 2: Parse + verify signature (provider-specific)
	webhookPayload, err := providerClient.ParseWebhook(ctx, payload, headers)
	if err != nil {
		logger.Warn().Err(err).Msg("Webhook signature verification failed")
		return domain.NewAppError(401, "webhook signature verification failed", err)
	}

	metrics.RecordWebhook(string(providerName), webhookPayload.EventType)

	// Step 3: Replay attack prevention — check if already processed
	alreadyProcessed, err := u.webhookRepo.ExistsByEventType(ctx, providerName, webhookPayload.EventType+":"+webhookPayload.ExternalID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check webhook replay")
		// Non-fatal — continue processing
	}
	if alreadyProcessed {
		logger.Info().
			Str("event_type", webhookPayload.EventType).
			Str("external_id", webhookPayload.ExternalID).
			Msg("Duplicate webhook received, skipping")
		return nil // Return nil — not an error, just idempotent
	}

	// Step 4: Persist the raw webhook event
	eventID := uuid.New()
	webhookEvent := &domain.WebhookEvent{
		ID:         eventID,
		Provider:   providerName,
		EventType:  webhookPayload.EventType + ":" + webhookPayload.ExternalID,
		RawPayload: webhookPayload.RawData,
		Processed:  false,
		ReceivedAt: time.Now(),
	}
	if err := u.webhookRepo.Create(ctx, webhookEvent); err != nil {
		logger.Error().Err(err).Msg("Failed to persist webhook event")
		// Non-fatal — still process the status update
	}

	// Step 5: Find the transaction by provider's external ID
	tx, err := u.txRepo.GetByExternalID(ctx, webhookPayload.ExternalID, providerName)
	if err != nil {
		if err == domain.ErrTransactionNotFound {
			logger.Warn().
				Str("external_id", webhookPayload.ExternalID).
				Msg("Webhook received for unknown transaction, ignoring")
			return nil // Not our transaction — ignore gracefully
		}
		return domain.ErrInternal(fmt.Errorf("get transaction by external id: %w", err))
	}

	// Step 6: Validate and apply state transition
	newStatus := webhookPayload.Status
	if !tx.CanTransitionTo(newStatus) {
		logger.Info().
			Str("transaction_id", tx.ID.String()).
			Str("current_status", string(tx.Status)).
			Str("new_status", string(newStatus)).
			Msg("Ignoring webhook — invalid or redundant state transition")
		return nil // Not an error — terminal status already reached
	}

	prevStatus := tx.Status

	if err := u.txRepo.UpdateStatus(ctx, tx.ID, newStatus); err != nil {
		return domain.ErrInternal(fmt.Errorf("update transaction status: %w", err))
	}

	// Step 7: Write audit log
	auditEntry := &domain.TransactionAuditLog{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		FromStatus:    &prevStatus,
		ToStatus:      newStatus,
		Source:        "webhook",
		RawPayload:    webhookPayload.RawData,
		CreatedAt:     time.Now(),
	}
	if err := u.auditRepo.Create(ctx, auditEntry); err != nil {
		logger.Error().Err(err).Str("transaction_id", tx.ID.String()).Msg("Failed to write webhook audit log")
	}

	// Step 8: Mark webhook event as processed
	if err := u.webhookRepo.MarkProcessed(ctx, eventID); err != nil {
		logger.Error().Err(err).Msg("Failed to mark webhook as processed")
	}

	logger.Info().
		Str("transaction_id", tx.ID.String()).
		Str("external_id", webhookPayload.ExternalID).
		Str("from_status", string(prevStatus)).
		Str("to_status", string(newStatus)).
		Msg("Webhook processed successfully")

	return nil
}
