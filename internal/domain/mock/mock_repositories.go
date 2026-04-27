package mock

import (
	"context"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockTransactionRepository mocks domain.TransactionRepository
type MockTransactionRepository struct {
	mock.Mock
}

func (m *MockTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) GetByExternalID(ctx context.Context, externalID string, provider domain.PaymentProvider) (*domain.Transaction, error) {
	args := m.Called(ctx, externalID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTransactionRepository) List(ctx context.Context, userID uuid.UUID, filter domain.TransactionFilter) ([]*domain.Transaction, int64, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockTransactionRepository) GetPendingByProvider(ctx context.Context, provider domain.PaymentProvider, since time.Time) ([]*domain.Transaction, error) {
	args := m.Called(ctx, provider, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

// MockAuditRepository mocks domain.AuditRepository
type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) Create(ctx context.Context, log *domain.TransactionAuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockAuditRepository) GetByTransactionID(ctx context.Context, txID uuid.UUID) ([]*domain.TransactionAuditLog, error) {
	args := m.Called(ctx, txID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TransactionAuditLog), args.Error(1)
}

// MockIdempotencyRepository mocks domain.IdempotencyRepository
type MockIdempotencyRepository struct {
	mock.Mock
}

func (m *MockIdempotencyRepository) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IdempotencyRecord), args.Error(1)
}

func (m *MockIdempotencyRepository) Set(ctx context.Context, key string, record *domain.IdempotencyRecord) error {
	args := m.Called(ctx, key, record)
	return args.Error(0)
}

// MockProviderRouter mocks domain.ProviderRouter
type MockProviderRouter struct {
	mock.Mock
}

func (m *MockProviderRouter) Route(ctx context.Context, currency string) (domain.PaymentProviderClient, error) {
	args := m.Called(ctx, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.PaymentProviderClient), args.Error(1)
}

func (m *MockProviderRouter) GetProvider(name domain.PaymentProvider) (domain.PaymentProviderClient, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.PaymentProviderClient), args.Error(1)
}

// MockPaymentProviderClient mocks domain.PaymentProviderClient
type MockPaymentProviderClient struct {
	mock.Mock
}

func (m *MockPaymentProviderClient) Name() domain.PaymentProvider {
	args := m.Called()
	return args.Get(0).(domain.PaymentProvider)
}

func (m *MockPaymentProviderClient) Charge(ctx context.Context, req *domain.ChargeRequest) (*domain.ChargeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChargeResponse), args.Error(1)
}

func (m *MockPaymentProviderClient) GetStatus(ctx context.Context, externalID string) (*domain.ChargeResponse, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChargeResponse), args.Error(1)
}

func (m *MockPaymentProviderClient) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*domain.WebhookPayload, error) {
	args := m.Called(ctx, payload, headers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WebhookPayload), args.Error(1)
}

func (m *MockPaymentProviderClient) SupportedMethods() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockUserRepository mocks domain.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByAPIKey(ctx context.Context, hashedKey string) (*domain.User, error) {
	args := m.Called(ctx, hashedKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
