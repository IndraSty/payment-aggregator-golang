package domain

import (
	"context"
	"time"
)

// ChargeRequest is the unified input for creating a charge across all providers.
// Usecase layer builds this from the HTTP request, then passes to the provider.
type ChargeRequest struct {
	ExternalID    string // unique ID we generate, sent to provider
	Amount        int64  // in smallest currency unit
	Currency      string // ISO 4217
	PaymentMethod string // provider-specific method name
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Description   string
	Metadata      map[string]any
	CallbackURL   string // webhook URL for this provider
	ReturnURL     string // redirect after payment (for hosted pages)
}

// ChargeResponse is the unified output after creating a charge.
// Each provider's mapper converts provider-specific response to this struct.
type ChargeResponse struct {
	ExternalID  string // provider's transaction ID
	Status      TransactionStatus
	PaymentURL  string // redirect URL for hosted payment pages (optional)
	ExpiredAt   *time.Time
	RawResponse map[string]any // original provider response, stored in metadata
}

// WebhookPayload is the parsed and normalized webhook from any provider.
type WebhookPayload struct {
	ExternalID string
	EventType  string
	Status     TransactionStatus
	Amount     int64
	Currency   string
	RawData    map[string]any
}

// PaymentProviderClient is the interface every provider must implement.
// Following the Open/Closed Principle: add new providers without changing existing code.
type PaymentProviderClient interface {
	// Name returns the provider identifier.
	Name() PaymentProvider

	// Charge creates a new payment charge at the provider.
	Charge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error)

	// GetStatus fetches the current status of a charge from the provider.
	// Used during reconciliation to sync local state.
	GetStatus(ctx context.Context, externalID string) (*ChargeResponse, error)

	// ParseWebhook validates and parses an incoming webhook from the provider.
	// Performs signature verification internally.
	ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*WebhookPayload, error)

	// SupportedMethods returns the payment methods this provider supports.
	SupportedMethods() []string
}

// ProviderRouter selects the appropriate provider for a given currency.
// Returns ErrNoProviderAvailable if no provider handles the currency.
type ProviderRouter interface {
	// Route returns the best available provider for the given currency.
	// Considers circuit breaker state — skips providers that are open (down).
	Route(ctx context.Context, currency string) (PaymentProviderClient, error)

	// GetProvider returns a specific provider by name.
	GetProvider(name PaymentProvider) (PaymentProviderClient, error)
}

// --- User domain ---

// User represents an API client registered in the system.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialized
	APIKey       string    `json:"-"` // stored as SHA-256 hash, never returned as-is
	CreatedAt    time.Time `json:"created_at"`
}

// UserRepository defines database operations for users.
type UserRepository interface {
	// Create inserts a new user. Returns ErrUserAlreadyExists if email taken.
	Create(ctx context.Context, user *User) error

	// GetByEmail retrieves a user by email for login.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// GetByAPIKey retrieves a user by their hashed API key.
	GetByAPIKey(ctx context.Context, hashedKey string) (*User, error)
}

// --- Usecase interfaces (implemented in usecase layer) ---

// ChargeUsecase defines the business logic for creating and managing charges.
type ChargeUsecase interface {
	// CreateCharge creates a new payment transaction.
	// Handles idempotency check, provider routing, and audit logging.
	CreateCharge(ctx context.Context, req *CreateChargeInput) (*Transaction, error)

	// GetCharge retrieves a transaction by ID (scoped to user).
	GetCharge(ctx context.Context, userID, txID string) (*Transaction, error)

	// ListCharges returns paginated transactions for a user.
	ListCharges(ctx context.Context, userID string, filter TransactionFilter) ([]*Transaction, int64, error)

	// ExpireCharge manually expires a pending transaction.
	ExpireCharge(ctx context.Context, userID, txID string) (*Transaction, error)
}

// WebhookUsecase defines the business logic for processing incoming webhooks.
type WebhookUsecase interface {
	// ProcessMidtrans handles an incoming Midtrans webhook.
	ProcessMidtrans(ctx context.Context, payload []byte, headers map[string]string) error

	// ProcessXendit handles an incoming Xendit webhook.
	ProcessXendit(ctx context.Context, payload []byte, headers map[string]string) error

	// ProcessStripe handles an incoming Stripe webhook.
	ProcessStripe(ctx context.Context, payload []byte, headers map[string]string) error
}

// ReconcileUsecase defines the business logic for reconciliation.
type ReconcileUsecase interface {
	// RunReconciliation triggers reconciliation for all providers.
	RunReconciliation(ctx context.Context) ([]*ReconciliationReport, error)

	// GetReport retrieves a reconciliation report by ID.
	GetReport(ctx context.Context, id string) (*ReconciliationReport, error)

	// ListReports returns paginated reconciliation reports.
	ListReports(ctx context.Context, filter ReconciliationFilter) ([]*ReconciliationReport, int64, error)
}

// AuthUsecase defines authentication business logic.
type AuthUsecase interface {
	// Register creates a new user and returns their API key (plaintext, one time only).
	Register(ctx context.Context, input *RegisterInput) (*RegisterOutput, error)

	// Login validates credentials and returns JWT tokens.
	Login(ctx context.Context, input *LoginInput) (*TokenPair, error)

	// RefreshToken validates refresh token and returns new token pair.
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
}

// --- Input/Output types for usecases ---

type CreateChargeInput struct {
	UserID         string
	IdempotencyKey string
	Amount         int64
	Currency       string
	PaymentMethod  string
	CustomerName   string
	CustomerEmail  string
	CustomerPhone  string
	Description    string
	Metadata       map[string]any
}

type RegisterInput struct {
	Email    string
	Password string
}

type RegisterOutput struct {
	User            *User
	PlaintextAPIKey string // only returned once at registration
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}
