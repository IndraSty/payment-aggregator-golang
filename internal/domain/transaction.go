package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TransactionStatus represents the lifecycle state of a transaction.
type TransactionStatus string

const (
	StatusPending TransactionStatus = "pending"
	StatusSuccess TransactionStatus = "success"
	StatusFailed  TransactionStatus = "failed"
	StatusExpired TransactionStatus = "expired"
)

// PaymentProvider represents a supported payment gateway.
type PaymentProvider string

const (
	ProviderMidtrans PaymentProvider = "midtrans"
	ProviderXendit   PaymentProvider = "xendit"
	ProviderStripe   PaymentProvider = "stripe"
)

// SupportedCurrencies maps currency codes to their allowed providers.
// IDR routes to Midtrans or Xendit; USD/EUR routes to Stripe.
var SupportedCurrencies = map[string][]PaymentProvider{
	"IDR": {ProviderMidtrans, ProviderXendit},
	"USD": {ProviderStripe},
	"EUR": {ProviderStripe},
}

// ValidTransitions defines the allowed state machine transitions.
// pending → success, failed, expired
// no transitions allowed from terminal states (success, failed, expired)
var ValidTransitions = map[TransactionStatus][]TransactionStatus{
	StatusPending: {StatusSuccess, StatusFailed, StatusExpired},
	StatusSuccess: {},
	StatusFailed:  {},
	StatusExpired: {},
}

// Transaction is the core domain entity.
type Transaction struct {
	ID             uuid.UUID         `json:"id"`
	UserID         uuid.UUID         `json:"user_id"`
	ExternalID     string            `json:"external_id"` // ID from payment provider
	IdempotencyKey string            `json:"idempotency_key"`
	Provider       PaymentProvider   `json:"provider"`
	Amount         int64             `json:"amount"`   // in smallest unit (cents/rupiah)
	Currency       string            `json:"currency"` // ISO 4217
	Status         TransactionStatus `json:"status"`
	PaymentMethod  string            `json:"payment_method"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ExpiredAt      *time.Time        `json:"expired_at,omitempty"`
}

// CanTransitionTo checks if the status transition is valid per the state machine.
func (t *Transaction) CanTransitionTo(next TransactionStatus) bool {
	allowed, ok := ValidTransitions[t.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the transaction is in a final state.
func (t *Transaction) IsTerminal() bool {
	return t.Status == StatusSuccess ||
		t.Status == StatusFailed ||
		t.Status == StatusExpired
}

// TransactionAuditLog records every status change permanently.
// This table is append-only — no UPDATE or DELETE ever.
type TransactionAuditLog struct {
	ID            uuid.UUID          `json:"id"`
	TransactionID uuid.UUID          `json:"transaction_id"`
	FromStatus    *TransactionStatus `json:"from_status,omitempty"` // nil = initial creation
	ToStatus      TransactionStatus  `json:"to_status"`
	Source        string             `json:"source"` // "api", "webhook", "reconciliation", "system"
	RawPayload    map[string]any     `json:"raw_payload,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// WebhookEvent records every incoming webhook for idempotency and auditing.
type WebhookEvent struct {
	ID         uuid.UUID       `json:"id"`
	Provider   PaymentProvider `json:"provider"`
	EventType  string          `json:"event_type"`
	RawPayload map[string]any  `json:"raw_payload"`
	Processed  bool            `json:"processed"`
	ReceivedAt time.Time       `json:"received_at"`
}

// ReconciliationReport stores the result of a reconciliation run.
type ReconciliationReport struct {
	ID            uuid.UUID            `json:"id"`
	Provider      PaymentProvider      `json:"provider"`
	Date          time.Time            `json:"date"`
	TotalChecked  int                  `json:"total_checked"`
	Discrepancies int                  `json:"discrepancies"`
	Status        ReconciliationStatus `json:"status"`
	CreatedAt     time.Time            `json:"created_at"`
}

// ReconciliationStatus represents the state of a reconciliation run.
type ReconciliationStatus string

const (
	ReconcileRunning   ReconciliationStatus = "running"
	ReconcileCompleted ReconciliationStatus = "completed"
	ReconcileFailed    ReconciliationStatus = "failed"
)

// --- Repository interfaces (implemented in repository layer) ---

// TransactionRepository defines all database operations for transactions.
type TransactionRepository interface {
	// Create inserts a new transaction. Returns ErrTransactionDuplicate if
	// idempotency_key already exists.
	Create(ctx context.Context, tx *Transaction) error

	// GetByID retrieves a transaction by its UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)

	// GetByIdempotencyKey retrieves a transaction by idempotency key.
	GetByIdempotencyKey(ctx context.Context, key string) (*Transaction, error)

	// GetByExternalID retrieves a transaction by provider's external ID.
	GetByExternalID(ctx context.Context, externalID string, provider PaymentProvider) (*Transaction, error)

	// UpdateStatus transitions a transaction to a new status.
	// Returns ErrTransactionInvalidState if transition is not allowed.
	UpdateStatus(ctx context.Context, id uuid.UUID, status TransactionStatus) error

	// List returns paginated transactions for a user with optional filters.
	List(ctx context.Context, userID uuid.UUID, filter TransactionFilter) ([]*Transaction, int64, error)

	// GetPendingByProvider returns pending transactions for reconciliation.
	GetPendingByProvider(ctx context.Context, provider PaymentProvider, since time.Time) ([]*Transaction, error)
}

// AuditRepository defines operations for the append-only audit log.
type AuditRepository interface {
	// Create inserts a new audit log entry. Never update or delete.
	Create(ctx context.Context, log *TransactionAuditLog) error

	// GetByTransactionID returns all audit entries for a transaction.
	GetByTransactionID(ctx context.Context, txID uuid.UUID) ([]*TransactionAuditLog, error)
}

// WebhookRepository defines operations for webhook event storage.
type WebhookRepository interface {
	// Create stores a webhook event.
	Create(ctx context.Context, event *WebhookEvent) error

	// ExistsByEventType checks if an event was already processed (replay prevention).
	ExistsByEventType(ctx context.Context, provider PaymentProvider, eventType string) (bool, error)

	// MarkProcessed marks a webhook event as processed.
	MarkProcessed(ctx context.Context, id uuid.UUID) error
}

// ReconciliationRepository defines operations for reconciliation reports.
type ReconciliationRepository interface {
	// Create starts a new reconciliation report with status "running".
	Create(ctx context.Context, report *ReconciliationReport) error

	// Update updates the report with results.
	Update(ctx context.Context, report *ReconciliationReport) error

	// GetByID retrieves a reconciliation report.
	GetByID(ctx context.Context, id uuid.UUID) (*ReconciliationReport, error)

	// List returns paginated reconciliation reports.
	List(ctx context.Context, filter ReconciliationFilter) ([]*ReconciliationReport, int64, error)
}

// IdempotencyRepository defines Redis-based idempotency key operations.
type IdempotencyRepository interface {
	// Get retrieves cached response for an idempotency key.
	// Returns nil if key does not exist.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)

	// Set stores a response with TTL (24 hours by default).
	Set(ctx context.Context, key string, record *IdempotencyRecord) error
}

// --- Filter and pagination types ---

// TransactionFilter holds query parameters for listing transactions.
type TransactionFilter struct {
	Status   *TransactionStatus
	Provider *PaymentProvider
	Currency *string
	Page     int
	Limit    int
}

// ReconciliationFilter holds query parameters for listing reconciliation reports.
type ReconciliationFilter struct {
	Provider *PaymentProvider
	Status   *ReconciliationStatus
	Page     int
	Limit    int
}

// IdempotencyRecord is what gets cached in Redis for an idempotency key.
type IdempotencyRecord struct {
	TransactionID string `json:"transaction_id"`
	StatusCode    int    `json:"status_code"`
	Response      []byte `json:"response"` // raw JSON response body
}
