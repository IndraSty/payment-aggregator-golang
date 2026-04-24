package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError represents a structured application error with HTTP status code.
type AppError struct {
	Code    int    // HTTP status code
	Message string // human-readable message
	Err     error  // underlying error (not exposed to client)
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError.
func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// --- Sentinel errors used across all layers ---

var (
	// Transaction errors
	ErrTransactionNotFound     = errors.New("transaction not found")
	ErrTransactionDuplicate    = errors.New("transaction already exists with this idempotency key")
	ErrTransactionInvalidState = errors.New("transaction status transition is invalid")
	ErrTransactionExpired      = errors.New("transaction has expired")

	// Provider errors
	ErrProviderUnavailable = errors.New("payment provider is currently unavailable")
	ErrProviderTimeout     = errors.New("payment provider request timed out")
	ErrProviderRejected    = errors.New("payment provider rejected the request")
	ErrNoProviderAvailable = errors.New("no payment provider available for this currency")

	// Auth errors
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInvalidAPIKey = errors.New("invalid or missing API key")
	ErrInvalidToken  = errors.New("invalid or expired token")
	ErrForbidden     = errors.New("access forbidden")

	// Validation errors
	ErrInvalidCurrency      = errors.New("currency not supported")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrInvalidPaymentMethod = errors.New("payment method not supported for this provider")
	ErrInvalidWebhookSig    = errors.New("webhook signature verification failed")

	// Webhook errors
	ErrWebhookReplay  = errors.New("webhook event already processed (replay attack prevention)")
	ErrWebhookExpired = errors.New("webhook timestamp is too old")

	// Idempotency errors
	ErrIdempotencyConflict = errors.New("idempotency key used with different request parameters")

	// User errors
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// --- Helper constructors for common HTTP errors ---

func ErrBadRequest(msg string) *AppError {
	return NewAppError(http.StatusBadRequest, msg, nil)
}

func ErrNotFound(msg string) *AppError {
	return NewAppError(http.StatusNotFound, msg, nil)
}

func ErrConflict(msg string) *AppError {
	return NewAppError(http.StatusConflict, msg, nil)
}

func ErrInternal(err error) *AppError {
	return NewAppError(http.StatusInternalServerError, "internal server error", err)
}

func ErrUnauth(msg string) *AppError {
	return NewAppError(http.StatusUnauthorized, msg, nil)
}

func ErrTooManyRequests() *AppError {
	return NewAppError(http.StatusTooManyRequests, "rate limit exceeded", nil)
}

func ErrServiceUnavailable(provider string) *AppError {
	return NewAppError(
		http.StatusServiceUnavailable,
		fmt.Sprintf("payment provider %s is currently unavailable", provider),
		nil,
	)
}
