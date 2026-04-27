package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/metrics"
	"github.com/IndraSty/payment-aggregator-golang/pkg/validator"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type chargeUsecase struct {
	txRepo          domain.TransactionRepository
	auditRepo       domain.AuditRepository
	idempotencyRepo domain.IdempotencyRepository
	providerRouter  domain.ProviderRouter
}

// NewChargeUsecase creates a new charge usecase.
func NewChargeUsecase(
	txRepo domain.TransactionRepository,
	auditRepo domain.AuditRepository,
	idempotencyRepo domain.IdempotencyRepository,
	providerRouter domain.ProviderRouter,
) domain.ChargeUsecase {
	return &chargeUsecase{
		txRepo:          txRepo,
		auditRepo:       auditRepo,
		idempotencyRepo: idempotencyRepo,
		providerRouter:  providerRouter,
	}
}

func (u *chargeUsecase) CreateCharge(ctx context.Context, input *domain.CreateChargeInput) (*domain.Transaction, error) {
	// Step 1: Validate input
	if err := validator.ValidateCreateCharge(input); err != nil {
		return nil, domain.NewAppError(400, err.Error(), nil)
	}

	input.Currency = strings.ToUpper(input.Currency)

	// Step 2: Check idempotency — if key already used, return cached response
	existing, err := u.idempotencyRepo.Get(ctx, input.IdempotencyKey)
	if err != nil {
		log.Warn().Err(err).Str("idempotency_key", input.IdempotencyKey).Msg("Idempotency cache read failed, proceeding")
	}
	if existing != nil {
		// Key already used — fetch and return the existing transaction
		txID, err := uuid.Parse(existing.TransactionID)
		if err == nil {
			tx, err := u.txRepo.GetByID(ctx, txID)
			if err == nil {
				log.Info().
					Str("idempotency_key", input.IdempotencyKey).
					Str("transaction_id", tx.ID.String()).
					Msg("Returning cached idempotent response")
				return tx, nil
			}
		}
	}

	// Step 3: Route to provider based on currency
	providerClient, err := u.providerRouter.Route(ctx, input.Currency)
	if err != nil {
		return nil, domain.ErrServiceUnavailable(input.Currency)
	}

	// Step 4: Build charge request for the provider
	externalID := fmt.Sprintf("pa-%s", uuid.New().String())
	chargeReq := &domain.ChargeRequest{
		ExternalID:    externalID,
		Amount:        input.Amount,
		Currency:      input.Currency,
		PaymentMethod: input.PaymentMethod,
		CustomerName:  input.CustomerName,
		CustomerEmail: input.CustomerEmail,
		CustomerPhone: input.CustomerPhone,
		Description:   input.Description,
		Metadata:      input.Metadata,
	}

	// Step 5: Call provider
	start := time.Now()
	chargeResp, err := providerClient.Charge(ctx, chargeReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.RecordCharge(string(providerClient.Name()), input.Currency, "failed", duration)
		log.Error().
			Err(err).
			Str("provider", string(providerClient.Name())).
			Str("external_id", externalID).
			Msg("Provider charge failed")
		return nil, domain.ErrServiceUnavailable(string(providerClient.Name()))
	}

	metrics.RecordCharge(string(providerClient.Name()), input.Currency, "success", duration)

	// Step 6: Persist the transaction
	userID, err := uuid.Parse(input.UserID)
	if err != nil {
		return nil, domain.ErrInternal(fmt.Errorf("invalid user id: %w", err))
	}

	now := time.Now()
	tx := &domain.Transaction{
		ID:             uuid.New(),
		UserID:         userID,
		ExternalID:     chargeResp.ExternalID,
		IdempotencyKey: input.IdempotencyKey,
		Provider:       providerClient.Name(),
		Amount:         input.Amount,
		Currency:       input.Currency,
		Status:         chargeResp.Status,
		PaymentMethod:  input.PaymentMethod,
		Metadata:       chargeResp.RawResponse,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiredAt:      chargeResp.ExpiredAt,
	}

	if err := u.txRepo.Create(ctx, tx); err != nil {
		return nil, domain.ErrInternal(fmt.Errorf("persist transaction: %w", err))
	}

	// Step 7: Write audit log for initial creation
	auditEntry := &domain.TransactionAuditLog{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		FromStatus:    nil, // nil = initial creation
		ToStatus:      tx.Status,
		Source:        "api",
		RawPayload:    chargeResp.RawResponse,
		CreatedAt:     now,
	}
	if err := u.auditRepo.Create(ctx, auditEntry); err != nil {
		// Non-fatal — log but don't fail the request
		log.Error().Err(err).Str("transaction_id", tx.ID.String()).Msg("Failed to write initial audit log")
	}

	// Step 8: Cache idempotency key in Redis
	idempotencyRecord := &domain.IdempotencyRecord{
		TransactionID: tx.ID.String(),
		StatusCode:    201,
	}
	if err := u.idempotencyRepo.Set(ctx, input.IdempotencyKey, idempotencyRecord); err != nil {
		// Non-fatal — idempotency cache miss is recoverable
		log.Warn().Err(err).Str("idempotency_key", input.IdempotencyKey).Msg("Failed to cache idempotency key")
	}

	log.Info().
		Str("transaction_id", tx.ID.String()).
		Str("provider", string(tx.Provider)).
		Str("currency", tx.Currency).
		Int64("amount", tx.Amount).
		Msg("Charge created successfully")

	return tx, nil
}

func (u *chargeUsecase) GetCharge(ctx context.Context, userID, txID string) (*domain.Transaction, error) {
	id, err := uuid.Parse(txID)
	if err != nil {
		return nil, domain.ErrNotFound("transaction not found")
	}

	tx, err := u.txRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrTransactionNotFound {
			return nil, domain.ErrNotFound("transaction not found")
		}
		return nil, domain.ErrInternal(err)
	}

	// Ensure transaction belongs to the requesting user
	if tx.UserID.String() != userID {
		return nil, domain.ErrNotFound("transaction not found")
	}

	return tx, nil
}

func (u *chargeUsecase) ListCharges(ctx context.Context, userID string, filter domain.TransactionFilter) ([]*domain.Transaction, int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, 0, domain.ErrInternal(fmt.Errorf("invalid user id: %w", err))
	}

	filter.Page, filter.Limit = validator.ValidatePagination(filter.Page, filter.Limit)

	txs, total, err := u.txRepo.List(ctx, uid, filter)
	if err != nil {
		return nil, 0, domain.ErrInternal(err)
	}

	return txs, total, nil
}

func (u *chargeUsecase) ExpireCharge(ctx context.Context, userID, txID string) (*domain.Transaction, error) {
	tx, err := u.GetCharge(ctx, userID, txID)
	if err != nil {
		return nil, err
	}

	// Validate state transition
	if !tx.CanTransitionTo(domain.StatusExpired) {
		return nil, domain.NewAppError(422,
			fmt.Sprintf("cannot expire transaction in status: %s", tx.Status),
			domain.ErrTransactionInvalidState,
		)
	}

	prevStatus := tx.Status

	if err := u.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusExpired); err != nil {
		return nil, domain.ErrInternal(err)
	}

	tx.Status = domain.StatusExpired
	tx.UpdatedAt = time.Now()

	// Audit log the status change
	auditEntry := &domain.TransactionAuditLog{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		FromStatus:    &prevStatus,
		ToStatus:      domain.StatusExpired,
		Source:        "api",
		CreatedAt:     time.Now(),
	}
	if err := u.auditRepo.Create(ctx, auditEntry); err != nil {
		log.Error().Err(err).Str("transaction_id", tx.ID.String()).Msg("Failed to write expire audit log")
	}

	log.Info().
		Str("transaction_id", tx.ID.String()).
		Str("from_status", string(prevStatus)).
		Msg("Transaction expired manually")

	return tx, nil
}
