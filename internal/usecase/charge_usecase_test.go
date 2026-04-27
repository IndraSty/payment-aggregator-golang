package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain/mock"
	"github.com/IndraSty/payment-aggregator-golang/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
)

func setupChargeUsecase(
	txRepo *mock.MockTransactionRepository,
	auditRepo *mock.MockAuditRepository,
	idempotencyRepo *mock.MockIdempotencyRepository,
	router *mock.MockProviderRouter,
) domain.ChargeUsecase {
	return usecase.NewChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)
}

// TestCreateCharge_Success tests successful charge creation
func TestCreateCharge_Success(t *testing.T) {
	txRepo := new(mock.MockTransactionRepository)
	auditRepo := new(mock.MockAuditRepository)
	idempotencyRepo := new(mock.MockIdempotencyRepository)
	router := new(mock.MockProviderRouter)
	providerClient := new(mock.MockPaymentProviderClient)

	uc := setupChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)

	ctx := context.Background()
	userID := uuid.New()
	expiredAt := time.Now().Add(24 * time.Hour)

	input := &domain.CreateChargeInput{
		UserID:         userID.String(),
		IdempotencyKey: "test-key-123",
		Amount:         150000,
		Currency:       "IDR",
		PaymentMethod:  "bank_transfer",
		CustomerName:   "John Doe",
		CustomerEmail:  "john@example.com",
		Description:    "Test payment",
	}

	chargeResp := &domain.ChargeResponse{
		ExternalID:  "pa-external-123",
		Status:      domain.StatusPending,
		PaymentURL:  "https://payment.url",
		ExpiredAt:   &expiredAt,
		RawResponse: map[string]any{"token": "abc123"},
	}

	// Setup mock expectations
	idempotencyRepo.On("Get", ctx, input.IdempotencyKey).Return(nil, nil)
	router.On("Route", ctx, "IDR").Return(providerClient, nil)
	providerClient.On("Name").Return(domain.ProviderMidtrans)
	providerClient.On("Charge", ctx, testifymock.AnythingOfType("*domain.ChargeRequest")).Return(chargeResp, nil)
	txRepo.On("Create", ctx, testifymock.AnythingOfType("*domain.Transaction")).Return(nil)
	auditRepo.On("Create", ctx, testifymock.AnythingOfType("*domain.TransactionAuditLog")).Return(nil)
	idempotencyRepo.On("Set", ctx, input.IdempotencyKey, testifymock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil)

	// Execute
	tx, err := uc.CreateCharge(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.Equal(t, domain.StatusPending, tx.Status)
	assert.Equal(t, domain.ProviderMidtrans, tx.Provider)
	assert.Equal(t, int64(150000), tx.Amount)
	assert.Equal(t, "IDR", tx.Currency)
	assert.Equal(t, input.IdempotencyKey, tx.IdempotencyKey)

	txRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
	idempotencyRepo.AssertExpectations(t)
	router.AssertExpectations(t)
}

// TestCreateCharge_IdempotencyHit tests that duplicate key returns cached transaction
func TestCreateCharge_IdempotencyHit(t *testing.T) {
	txRepo := new(mock.MockTransactionRepository)
	auditRepo := new(mock.MockAuditRepository)
	idempotencyRepo := new(mock.MockIdempotencyRepository)
	router := new(mock.MockProviderRouter)

	uc := setupChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)

	ctx := context.Background()
	userID := uuid.New()
	existingTxID := uuid.New()

	input := &domain.CreateChargeInput{
		UserID:         userID.String(),
		IdempotencyKey: "duplicate-key",
		Amount:         150000,
		Currency:       "IDR",
		PaymentMethod:  "bank_transfer",
	}

	cachedRecord := &domain.IdempotencyRecord{
		TransactionID: existingTxID.String(),
		StatusCode:    201,
	}

	existingTx := &domain.Transaction{
		ID:             existingTxID,
		UserID:         userID,
		IdempotencyKey: "duplicate-key",
		Amount:         150000,
		Currency:       "IDR",
		Status:         domain.StatusPending,
		Provider:       domain.ProviderMidtrans,
	}

	// Cache hit — returns existing record
	idempotencyRepo.On("Get", ctx, input.IdempotencyKey).Return(cachedRecord, nil)
	txRepo.On("GetByID", ctx, existingTxID).Return(existingTx, nil)

	tx, err := uc.CreateCharge(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, tx)
	// Must return the SAME transaction — not a new one
	assert.Equal(t, existingTxID, tx.ID)

	// Provider must NOT be called — no double charge
	router.AssertNotCalled(t, "Route")
	txRepo.AssertNotCalled(t, "Create")
}

// TestCreateCharge_InvalidAmount tests that negative/zero amount is rejected
func TestCreateCharge_InvalidAmount(t *testing.T) {
	txRepo := new(mock.MockTransactionRepository)
	auditRepo := new(mock.MockAuditRepository)
	idempotencyRepo := new(mock.MockIdempotencyRepository)
	router := new(mock.MockProviderRouter)

	uc := setupChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)

	ctx := context.Background()

	input := &domain.CreateChargeInput{
		UserID:         uuid.New().String(),
		IdempotencyKey: "test-key",
		Amount:         -1000, // invalid
		Currency:       "IDR",
		PaymentMethod:  "bank_transfer",
	}

	tx, err := uc.CreateCharge(ctx, input)

	assert.Nil(t, tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount")

	// Nothing should be called
	router.AssertNotCalled(t, "Route")
	txRepo.AssertNotCalled(t, "Create")
}

// TestCreateCharge_InvalidCurrency tests unsupported currency is rejected
func TestCreateCharge_InvalidCurrency(t *testing.T) {
	txRepo := new(mock.MockTransactionRepository)
	auditRepo := new(mock.MockAuditRepository)
	idempotencyRepo := new(mock.MockIdempotencyRepository)
	router := new(mock.MockProviderRouter)

	uc := setupChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)

	ctx := context.Background()

	input := &domain.CreateChargeInput{
		UserID:         uuid.New().String(),
		IdempotencyKey: "test-key",
		Amount:         150000,
		Currency:       "JPY", // not supported
		PaymentMethod:  "bank_transfer",
	}

	tx, err := uc.CreateCharge(ctx, input)

	assert.Nil(t, tx)
	assert.Error(t, err)

	router.AssertNotCalled(t, "Route")
}

// TestCreateCharge_ProviderUnavailable tests circuit breaker fallback behavior
func TestCreateCharge_ProviderUnavailable(t *testing.T) {
	txRepo := new(mock.MockTransactionRepository)
	auditRepo := new(mock.MockAuditRepository)
	idempotencyRepo := new(mock.MockIdempotencyRepository)
	router := new(mock.MockProviderRouter)

	uc := setupChargeUsecase(txRepo, auditRepo, idempotencyRepo, router)

	ctx := context.Background()

	input := &domain.CreateChargeInput{
		UserID:         uuid.New().String(),
		IdempotencyKey: "test-key",
		Amount:         150000,
		Currency:       "IDR",
		PaymentMethod:  "bank_transfer",
	}

	idempotencyRepo.On("Get", ctx, input.IdempotencyKey).Return(nil, nil)
	// Router returns error — all providers down
	router.On("Route", ctx, "IDR").Return(nil, domain.ErrNoProviderAvailable)

	tx, err := uc.CreateCharge(ctx, input)

	assert.Nil(t, tx)
	assert.Error(t, err)
	txRepo.AssertNotCalled(t, "Create")
}
